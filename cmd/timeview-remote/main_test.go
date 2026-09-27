package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	for _, args := range [][]string{{}, {"-server", "file:///tmp", "status"}, {"listen", "extra"}, {"add", "0"}, {"preset", "10"}, {"message"}, {"unknown"}} {
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

func TestInputBindings(t *testing.T) {
	midi := midiBinding{Status: 0x90, Data1: 36}
	bindings := []keyBinding{
		{Action: "start", Code: "KeyS", Ctrl: true},
		{Action: "preset2", Code: "Numpad3", MIDI: &midi},
	}
	if got := actionFor(bindings, inputEvent{Code: "KeyS", Ctrl: true}); got != "start" {
		t.Fatalf("keyboard action = %q", got)
	}
	if got := actionFor(bindings, inputEvent{Code: "KeyS"}); got != "" {
		t.Fatalf("keyboard action without modifier = %q", got)
	}
	if got := actionFor(bindings, inputEvent{MIDI: &midi}); got != "preset2" {
		t.Fatalf("MIDI action = %q", got)
	}
	if got := strings.Join(commandForAction("preset2"), " "); got != "preset 3" {
		t.Fatalf("preset command = %q", got)
	}
	if commandForAction("preset9") != nil || commandForAction("unknown") != nil {
		t.Fatal("invalid actions must not create commands")
	}
}

func TestNativeInputMappingAndMIDILatch(t *testing.T) {
	keyboard := keyboardEvent(0x0e1c, maskCtrl|maskShift)
	if keyboard.Code != "NumpadEnter" || !keyboard.Ctrl || !keyboard.Shift || keyboard.Alt || keyboard.Meta {
		t.Fatalf("keyboard event = %#v", keyboard)
	}

	var latch midiLatch
	var events []midiBinding
	for _, message := range [][3]uint8{{0x90, 36, 127}, {0x90, 36, 100}, {0x80, 36, 0}, {0x90, 36, 64}} {
		status, data1, value := message[0], message[1], message[2]
		if binding, ok := latch.press(1, status, data1, value); ok {
			events = append(events, binding)
		}
	}
	if len(events) != 2 || events[0].Data1 != 36 || events[1].Data1 != 36 {
		t.Fatalf("MIDI events = %#v", events)
	}
}

func TestListenUsesServerBindings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/timer/bindings":
			_, _ = w.Write([]byte(`{"bindings":[{"action":"start","code":"NumpadEnter","ctrl":false,"shift":false,"alt":false,"meta":false}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/timer":
			_, _ = w.Write([]byte(`{"instanceId":"boot"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/timer/commands":
			_, _ = w.Write([]byte(`{"instanceId":"boot"}`))
			cancel()
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	original := startInputs
	defer func() { startInputs = original }()
	stopped := make(chan struct{})
	startInputs = func(events chan<- inputEvent) (func(), int, error) {
		events <- inputEvent{Code: "NumpadEnter"}
		return func() { close(stopped) }, 1, nil
	}
	var output strings.Builder
	r := remote{server.URL + "/api/v1/timer", &http.Client{Timeout: time.Second}}
	if err := r.listen(ctx, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "MIDI入力: 1台") || !strings.Contains(output.String(), "OK start") {
		t.Fatalf("output = %q", output.String())
	}
	select {
	case <-stopped:
	default:
		t.Fatal("input listener was not stopped")
	}
}
