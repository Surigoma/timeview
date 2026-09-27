package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"instanceId":"boot"}`))
			return
		}
		if r.URL.Path != "/api/v1/timer/commands" || r.Method != http.MethodPost || r.Header.Get("X-Timeview-Instance") != "boot" || r.Header.Get("Idempotency-Key") == "" {
			t.Fatalf("unexpected request: %s %s %#v", r.Method, r.URL.Path, r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["command"] != "start" {
			t.Fatalf("unexpected body: %#v, %v", body, err)
		}
		_, _ = w.Write([]byte(`{"instanceId":"boot"}`))
	}))
	defer server.Close()
	var output strings.Builder
	if err := run([]string{"-server", server.URL, "start"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "OK start\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestPresetAndAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/timer":
			_, _ = w.Write([]byte(`{"instanceId":"boot"}`))
		case "/api/v1/timer/presets":
			_, _ = w.Write([]byte(`{"presets":["残り5分"]}`))
		case "/api/v1/timer/message":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"ブラウザから操作してください"}}`))
		}
	}))
	defer server.Close()
	err := run([]string{"-server", server.URL, "preset", "1"}, &strings.Builder{})
	if err == nil || err.Error() != "ブラウザから操作してください" {
		t.Fatalf("error = %v", err)
	}
}

func TestArgumentValidation(t *testing.T) {
	for _, args := range [][]string{{}, {"-server", "file:///tmp", "status"}, {"add", "0"}, {"preset", "10"}, {"message"}, {"unknown"}} {
		if err := run(args, &strings.Builder{}); err == nil {
			t.Fatalf("expected error for %#v", args)
		}
	}
}

func TestStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"instanceId":"boot","state":"idle"}`))
	}))
	defer server.Close()
	var output strings.Builder
	if err := run([]string{"-server", server.URL, "status"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"state":"idle"`) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRequestForCommands(t *testing.T) {
	for _, args := range [][]string{
		{"start"}, {"pause"}, {"reset"}, {"add"}, {"add", "30"}, {"subtract"},
		{"blackout"}, {"reveal"}, {"hide"}, {"show"}, {"clear"}, {"message", "hello", "world"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			path, method, _, err := requestFor(args, remote{})
			if err != nil || path == "" || method == "" {
				t.Fatalf("request = %q %q, error = %v", method, path, err)
			}
		})
	}
}

func TestResponseErrorFallback(t *testing.T) {
	if got := responseError(http.StatusBadGateway, []byte("invalid")).Error(); got != "TimeViewがHTTP 502を返しました" {
		t.Fatalf("error = %q", got)
	}
}
