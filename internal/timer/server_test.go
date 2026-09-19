package timer

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func request(s *Server, method, path, body, key, etag string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("If-Match", etag)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func stateOf(t *testing.T, w *httptest.ResponseRecorder) State {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var state State
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestTimerBlackoutAndPause(t *testing.T) {
	now := time.Now()
	s := New()
	s.now = func() time.Time { return now }
	s.model.anchor = now
	start := stateOf(t, request(s, "POST", "/timer/commands", `{"command":"start"}`, "start", ""))
	now = now.Add(30 * time.Second)
	paused := stateOf(t, request(s, "POST", "/timer/commands", `{"command":"pause"}`, "pause", ""))
	if paused.Remaining != 570000 || !paused.Blackout {
		t.Fatalf("bad pause: %+v", paused)
	}
	now = now.Add(time.Minute)
	resumed := stateOf(t, request(s, "POST", "/timer/commands", `{"command":"start"}`, "resume", ""))
	if resumed.Remaining != 570000 || resumed.Version <= start.Version {
		t.Fatal(resumed)
	}
	now = now.Add(600 * time.Second)
	revealed := stateOf(t, request(s, "PUT", "/timer/blackout", `{"enabled":false}`, "reveal", ""))
	if revealed.Remaining != -30000 || revealed.Blackout {
		t.Fatal(revealed)
	}
	again := stateOf(t, request(s, "PUT", "/timer/blackout", `{"enabled":false}`, "again", ""))
	if again.Version != revealed.Version {
		t.Fatal("same blackout changed version")
	}
	stateOf(t, request(s, "PUT", "/timer/blackout", `{"enabled":true}`, "black", ""))
	reset := stateOf(t, request(s, "POST", "/timer/commands", `{"command":"reset"}`, "reset", ""))
	if !reset.Blackout || reset.Status != "idle" || reset.Remaining != 600000 {
		t.Fatal(reset)
	}
}

func TestReplayAtomicityAndValidation(t *testing.T) {
	s := New()
	body := `{"command":"adjust","deltaSeconds":60}`
	first := stateOf(t, request(s, "POST", "/timer/commands", body, "add", ""))
	replay := stateOf(t, request(s, "POST", "/timer/commands", body, "add", ""))
	if first != replay || replay.Remaining != 660000 {
		t.Fatal("replay changed result")
	}
	if w := request(s, "POST", "/timer/commands", `{"command":"reset"}`, "add", ""); w.Code != 409 {
		t.Fatal(w.Code)
	}
	before := s.model.State
	if w := request(s, "PATCH", "/timer", `{"durationSeconds":10,"warning1Seconds":100}`, "settings", before.etag()); w.Code != 422 {
		t.Fatal(w.Code)
	}
	if s.model.State != before {
		t.Fatal("failed validation partially changed state")
	}
	if w := request(s, "PATCH", "/timer", `{"durationSeconds":900}`, "", `"stale"`); w.Code != 412 {
		t.Fatal(w.Code)
	}
	for _, body := range []string{`{"enabled":null}`, `{"enabled":"false"}`, `{}`, `{"enabled":true,"unknown":1}`} {
		if w := request(s, "PUT", "/timer/blackout", body, "", ""); w.Code != 422 {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	if w := request(s, "PUT", "/timer/message", `{"text":"`+strings.Repeat("あ", 501)+`"}`, "", ""); w.Code != 422 {
		t.Fatal(w.Code)
	}
	if w := request(s, "POST", "/timer/commands", body, "", ""); w.Code != 428 {
		t.Fatal(w.Code)
	}
}

func TestConcurrentAdjustAndRestart(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for _, key := range []string{"one", "two"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			w := request(s, "POST", "/timer/commands", `{"command":"adjust","deltaSeconds":60}`, key, "")
			if w.Code != 200 {
				t.Errorf("%d", w.Code)
			}
		}(key)
	}
	wg.Wait()
	if s.model.Remaining != 720000 {
		t.Fatal(s.model.Remaining)
	}
	next := New()
	if next.model.InstanceID == s.model.InstanceID || !next.model.Blackout || next.model.Remaining != 600000 {
		t.Fatal("restart did not reset")
	}
	r := httptest.NewRequest("POST", "/api/v1/timer/commands", strings.NewReader(`{"command":"start"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Timeview-Instance", s.model.InstanceID)
	w := httptest.NewRecorder()
	next.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("stale boot accepted")
	}
}

func TestReplayCapacityAndExpiry(t *testing.T) {
	now := time.Now()
	s := New()
	s.now = func() time.Time { return now }
	for i := 0; i < 10000; i++ {
		s.receipts[string(rune(i))] = receipt{expires: now.Add(time.Minute)}
	}
	w := request(s, "POST", "/timer/commands", `{"command":"start"}`, "new-key", "")
	if w.Code != 429 || s.model.Status != "idle" {
		t.Fatal("capacity must not execute request")
	}
	now = now.Add(2 * time.Minute)
	stateOf(t, request(s, "POST", "/timer/commands", `{"command":"start"}`, "new-key", ""))
	if len(s.receipts) != 1 {
		t.Fatal("expired receipts not removed")
	}
}
