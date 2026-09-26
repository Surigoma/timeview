package httpserver

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"timeview/internal/timer"
)

func rawRequest(s *Server, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestReadEndpointsAndHeaders(t *testing.T) {
	s := New()
	for _, test := range []struct {
		path, body string
	}{
		{"/health", `"status":"ok"`},
		{"/timer", `"state":"idle"`},
		{"/timer/presets", `"presets":[]`},
		{"/timer/bindings", `"bindings":`},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := request(s, http.MethodGet, test.path, "", "", "")
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("missing API headers: %v", response.Header())
			}
		})
	}
	if response := request(s, http.MethodGet, "/missing", "", "", ""); response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "NOT_FOUND") {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
}

func TestMutationTransportValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		status      int
		code        string
	}{
		{"content type", "text/plain", []byte(`{"enabled":true}`), 400, "INVALID_CONTENT_TYPE"},
		{"body limit", "application/json", bytes.Repeat([]byte(" "), bodyLimit+1), 413, "BODY_TOO_LARGE"},
		{"invalid utf8", "application/json", []byte{0xff}, 422, "INVALID_ARGUMENT"},
		{"invalid json", "application/json", []byte(`{`), 400, "INVALID_JSON"},
		{"multiple json values", "application/json", []byte(`{"enabled":true}{"enabled":false}`), 400, "INVALID_JSON"},
		{"json array", "application/json", []byte(`[]`), 400, "INVALID_JSON"},
		{"null field", "application/json", []byte(`{"enabled":null}`), 422, "INVALID_ARGUMENT"},
		{"wrong type", "application/json", []byte(`{"enabled":"yes"}`), 422, "INVALID_ARGUMENT"},
		{"unknown field", "application/json", []byte(`{"enabled":true,"extra":1}`), 422, "INVALID_ARGUMENT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := rawRequest(New(), http.MethodPut, "/timer/blackout", test.contentType, test.body)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestDecodeRejectsNestedNull(t *testing.T) {
	var payload struct {
		Items []struct {
			Enabled bool `json:"enabled"`
		} `json:"items"`
	}
	err := decode([]byte(`{"items":[{"enabled":null}]}`), &payload)
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("nested null was not rejected: %v", err)
	}
}

func TestMessageLifecycleAndValidation(t *testing.T) {
	s := New()
	shown := stateOf(t, request(s, http.MethodPut, "/timer/message", `{"text":"次の演目","visible":true}`, "message", ""))
	if shown.Message.Text != "次の演目" || !shown.Message.Visible {
		t.Fatal(shown.Message)
	}
	if response := request(s, http.MethodPut, "/timer/message", `{}`, "empty", ""); response.Code != 422 {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	if response := request(s, http.MethodDelete, "/timer/message", `{}`, "delete-with-body", ""); response.Code != 422 {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	cleared := stateOf(t, request(s, http.MethodDelete, "/timer/message", "", "delete", ""))
	if cleared.Message != (timer.Message{}) {
		t.Fatal(cleared.Message)
	}
	if response := request(s, http.MethodPut, "/timer/message", `{"visible":true}`, "show-empty", ""); response.Code != 422 {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	hidden := stateOf(t, request(s, http.MethodPut, "/timer/message", `{"text":"","visible":true}`, "empty-text", ""))
	if hidden.Message.Visible {
		t.Fatal("empty text must stay hidden")
	}
}

func TestRateLimitAndSSECapacity(t *testing.T) {
	now := time.Now()
	s := New()
	s.now = func() time.Time { return now }
	for i := 0; i < 30; i++ {
		if response := request(s, http.MethodGet, "/timer", "", "", ""); response.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, response.Code)
		}
	}
	response := request(s, http.MethodGet, "/timer", "", "", "")
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("status %d, headers %v", response.Code, response.Header())
	}

	full := New()
	full.now = func() time.Time { return now }
	for i := 0; i < 1024; i++ {
		full.limits[string(rune(i))] = &clientLimit{read: rate.NewLimiter(1, 1), write: rate.NewLimiter(1, 1), last: now}
	}
	if response := request(full, http.MethodGet, "/timer", "", "", ""); response.Code != http.StatusTooManyRequests {
		t.Fatalf("full limiter status %d", response.Code)
	}

	events := New()
	for range 50 {
		events.clients[make(chan struct{})] = struct{}{}
	}
	if response := request(events, http.MethodGet, "/timer/events", "", "", ""); response.Code != http.StatusTooManyRequests {
		t.Fatalf("SSE capacity status %d", response.Code)
	}
}

func TestExpiredRateLimitAndInternalError(t *testing.T) {
	now := time.Now()
	s := New()
	s.now = func() time.Time { return now }
	s.limits["expired"] = &clientLimit{read: rate.NewLimiter(1, 1), write: rate.NewLimiter(1, 1), last: now.Add(-2 * time.Minute)}
	if response := request(s, http.MethodGet, "/timer", "", "", ""); response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	if _, exists := s.limits["expired"]; exists {
		t.Fatal("expired limiter was not removed")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeError(c, errors.New("boom"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "INTERNAL_ERROR") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}
