package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func displayRequest(s *Server, id, body string) *httptest.ResponseRecorder {
	return requestAs(s, "PUT", "/timer/displays/"+id, body, "", "", true)
}
func displayBody(s *Server) string {
	return fmt.Sprintf(`{"instanceId":%q,"version":%d}`, s.model.InstanceID, s.model.Version)
}
func displaysOf(t *testing.T, s *Server) []displayReport {
	t.Helper()
	r := request(s, "GET", "/timer/displays", "", "", "")
	var result struct {
		Displays []displayReport `json:"displays"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil {
		t.Fatal(r.Code, r.Body)
	}
	return result.Displays
}

func TestDisplayResponseLifecycle(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0)
	s.now = func() time.Time { return now }
	id := strings.Repeat("a", 32)
	if len(displaysOf(t, s)) != 0 {
		t.Fatal("control counted as display")
	}
	original := s.model.State
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 {
		t.Fatal(r.Code, r.Body)
	}
	list := displaysOf(t, s)
	if len(list) != 1 || !list[0].Online || list[0].LastSeen != now.UnixMilli() {
		t.Fatal(list)
	}
	if s.model.State != original {
		t.Fatal("telemetry mutated timer")
	}
	now = now.Add(displayTimeout - time.Millisecond)
	if !displaysOf(t, s)[0].Online {
		t.Fatal("expired early")
	}
	now = now.Add(time.Millisecond)
	if displaysOf(t, s)[0].Online {
		t.Fatal("missing timeout")
	}
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 {
		t.Fatal(r.Code)
	}
	if !displaysOf(t, s)[0].Online {
		t.Fatal("did not reconnect")
	}
	now = now.Add(displayRetention)
	if len(displaysOf(t, s)) != 0 {
		t.Fatal("did not expire history")
	}
	// A restart starts with no reports and rejects old instance acknowledgements.
	newer := New()
	if r := displayRequest(newer, id, displayBody(s)); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if len(displaysOf(t, newer)) != 0 {
		t.Fatal("restart retained history")
	}
}

func TestDisplayValidationAndBrowserOnly(t *testing.T) {
	id := strings.Repeat("b", 32)
	cases := []struct {
		id, body string
		status   int
	}{
		{"invalid", `{}`, 422}, {id, `{`, 400}, {id, `{"instanceId":"wrong","version":0}`, 409}, {id, `{"instanceId":"INSTANCE"}`, 422},
		{id, `{"instanceId":"INSTANCE","version":-1}`, 422}, {id, `{"instanceId":"INSTANCE","version":1}`, 422},
		{id, `{"instanceId":"INSTANCE","version":0,"extra":1}`, 422}, {id, strings.Repeat(" ", bodyLimit+1), 413},
	}
	for _, tc := range cases {
		s := New()
		body := strings.ReplaceAll(tc.body, "INSTANCE", s.model.InstanceID)
		if r := displayRequest(s, tc.id, body); r.Code != tc.status {
			t.Fatalf("%s: %d %s", body, r.Code, r.Body)
		}
		if len(s.displays) != 0 {
			t.Fatal("invalid report recorded")
		}
	}
	s := New()
	s.model.BrowserOnly = true
	if r := request(s, "PUT", "/timer/displays/"+id, displayBody(s), "", ""); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 {
		t.Fatal(r.Code, r.Body)
	}
	r := httptest.NewRequest("PUT", "/api/v1/timer/displays/"+id, strings.NewReader(displayBody(s)))
	r.Header.Set("X-Timeview-Client", "browser")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestDisplayCapacityAndOutOfOrderReports(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0)
	s.now = func() time.Time { return now }
	s.displays = map[string]displayReport{}
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("%032x", i)
		s.displays[id] = displayReport{ID: id, LastSeen: now.UnixMilli()}
	}
	id := strings.Repeat("c", 32)
	if r := displayRequest(s, id, displayBody(s)); r.Code != 429 {
		t.Fatal(r.Code)
	}
	now = now.Add(displayTimeout)
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 || len(s.displays) != 50 {
		t.Fatal(r.Code, len(s.displays))
	}
	s.model.Version = 2
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 {
		t.Fatal(r.Code)
	}
	old := fmt.Sprintf(`{"instanceId":%q,"version":1}`, s.model.InstanceID)
	if r := displayRequest(s, id, old); r.Code != 204 || s.displays[id].Version != 2 {
		t.Fatal(r.Code, s.displays[id])
	}
}
