package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRemoveDisconnectedDisplay(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0)
	s.now = func() time.Time { return now }
	id := strings.Repeat("d", 32)
	displayRequest(s, id, displayBody(s))
	original := s.model
	remove := func(key string) *httptest.ResponseRecorder {
		return requestAs(s, "DELETE", "/timer/displays/"+id, "", key, "", true)
	}
	if r := remove("online"); r.Code != 409 {
		t.Fatal(r.Code, r.Body)
	}
	now = now.Add(displayTimeout)
	client := make(chan struct{}, 1)
	s.clients[client] = struct{}{}
	if r := remove("remove"); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	if len(displaysOf(t, s)) != 0 || !reflect.DeepEqual(original, s.model) {
		t.Fatal("removal changed timer or retained display")
	}
	select {
	case <-client:
	default:
		t.Fatal("subscribers were not notified")
	}
	if r := remove("absent"); r.Code != 200 {
		t.Fatal(r.Code)
	}
	displayRequest(s, id, displayBody(s))
	if r := remove("remove"); r.Code != 200 {
		t.Fatal("retry did not replay", r.Code)
	}
	if len(displaysOf(t, s)) != 1 {
		t.Fatal("retry removed a reconnected display")
	}
	if r := remove("reconnected"); r.Code != 409 {
		t.Fatal("reconnected display removed", r.Code)
	}
}

func TestRemoveDisplayGuards(t *testing.T) {
	s := New()
	id := strings.Repeat("e", 32)
	for _, tc := range []struct {
		id, body, etag string
		browser        bool
		status         int
	}{
		{id, "", "", false, 403},
		{"bad", "", "", true, 422},
		{id, `{}`, "", true, 422},
		{id, strings.Repeat("x", bodyLimit+1), "", true, 413},
		{id, "", `"wrong"`, true, 412},
	} {
		if r := requestAs(s, "DELETE", "/timer/displays/"+tc.id, tc.body, "", tc.etag, tc.browser); r.Code != tc.status {
			t.Fatal(r.Code, r.Body)
		}
	}
}

func TestDisplayNames(t *testing.T) {
	s := New()
	id := strings.Repeat("f", 32)
	displayRequest(s, id, displayBody(s))
	original := s.model
	rename := func(body string, browser bool) *httptest.ResponseRecorder {
		return requestAs(s, "PUT", "/timer/displays/"+id+"/name", body, "", "", browser)
	}
	if r := rename(`{"name":" ステージ左 "}`, true); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	displayRequest(s, id, displayBody(s))
	if got := displaysOf(t, s)[0].Name; got != "ステージ左" {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(original, s.model) {
		t.Fatal("name changed timer model")
	}
	for _, body := range []string{`{}`, `{"name":null}`, `{"name":1}`, `{"name":"a\nb"}`, `{"name":"ok","extra":1}`, fmt.Sprintf(`{"name":%q}`, strings.Repeat("あ", 41))} {
		if r := rename(body, true); r.Code < 400 {
			t.Fatal(body, r.Code)
		}
		if displaysOf(t, s)[0].Name != "ステージ左" {
			t.Fatal("invalid name committed")
		}
	}
	if r := rename(`{"name":"blocked"}`, false); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if r := rename(fmt.Sprintf(`{"name":%q}`, strings.Repeat("あ", 40)), true); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := rename(`{"name":""}`, true); r.Code != 200 || displaysOf(t, s)[0].Name != "" {
		t.Fatal(r.Code)
	}
	delete(s.displays, id)
	if r := rename(`{"name":"missing"}`, true); r.Code != 404 {
		t.Fatal(r.Code)
	}
	if r := requestAs(s, "PUT", "/timer/displays/bad/name", `{"name":"x"}`, "", "", true); r.Code != 422 {
		t.Fatal(r.Code)
	}
}

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
