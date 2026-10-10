package httpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"timeview/internal/auditlog"
)

type streamEvent struct {
	name string
	data json.RawMessage
}
type testStream struct{ scanner *bufio.Scanner }

func openTestStream(t *testing.T, host, topics string) *testStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", host+"/api/v1/timer/events"+topics, nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	return &testStream{scanner}
}
func (s *testStream) next(t *testing.T) streamEvent {
	t.Helper()
	event := streamEvent{}
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			event.name = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			event.data = json.RawMessage(strings.TrimPrefix(line, "data: "))
		}
		if line == "" && event.name != "" {
			return event
		}
	}
	t.Fatalf("stream ended: %v", s.scanner.Err())
	return event
}
func (s *testStream) named(t *testing.T, name string) json.RawMessage {
	t.Helper()
	for {
		event := s.next(t)
		if event.name == name {
			return event.data
		}
	}
}

func TestSSESubscribedLogsAndReconnect(t *testing.T) {
	log, err := auditlog.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	_ = log.Write(auditlog.Entry{Type: "process", Action: "start", Result: "success"})
	s := New()
	s.SetAuditLog(log)
	host := httptest.NewServer(s)
	t.Cleanup(host.Close)
	stream := openTestStream(t, host.URL, "?topics=displays,logs")
	if stream.next(t).name != "state" || stream.next(t).name != "displays" {
		t.Fatal("missing initial snapshots")
	}
	var snapshot struct {
		InstanceID string           `json:"instanceId"`
		Entries    []auditlog.Entry `json:"entries"`
	}
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if snapshot.InstanceID != s.model.InstanceID || len(snapshot.Entries) != 1 || snapshot.Entries[0].Action != "start" {
		t.Fatal(snapshot)
	}
	stateOf(t, request(s, "POST", "/timer/commands", `{"command":"start"}`, "start", ""))
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if len(snapshot.Entries) != 2 || snapshot.Entries[0].Action != "timer:start" {
		t.Fatal(snapshot)
	}
	if r := request(s, "POST", "/timer/commands", `{"command":"unknown"}`, "bad", ""); r.Code != 422 {
		t.Fatal(r.Code)
	}
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if len(snapshot.Entries) != 3 || snapshot.Entries[0].Result != "failure" {
		t.Fatal(snapshot)
	}
	reconnected := openTestStream(t, host.URL, "?topics=logs")
	_ = json.Unmarshal(reconnected.named(t, "logs"), &snapshot)
	if len(snapshot.Entries) != 3 {
		t.Fatal("reconnect lost history", snapshot)
	}
}

func TestSSEDisplayUpdatesTimeoutAndRetention(t *testing.T) {
	s := New()
	var clock atomic.Int64
	clock.Store(1700000000000)
	s.now = func() time.Time { return time.UnixMilli(clock.Load()) }
	host := httptest.NewServer(s)
	t.Cleanup(host.Close)
	stream := openTestStream(t, host.URL, "?topics=displays")
	var snapshot struct {
		Displays []displayReport `json:"displays"`
	}
	_ = json.Unmarshal(stream.named(t, "displays"), &snapshot)
	if len(snapshot.Displays) != 0 {
		t.Fatal(snapshot)
	}
	id := strings.Repeat("a", 32)
	if r := displayRequest(s, id, displayBody(s)); r.Code != 204 {
		t.Fatal(r.Code)
	}
	_ = json.Unmarshal(stream.named(t, "displays"), &snapshot)
	if len(snapshot.Displays) != 1 || !snapshot.Displays[0].Online {
		t.Fatal(snapshot)
	}
	clock.Add(displayTimeout.Milliseconds())
	_ = json.Unmarshal(stream.named(t, "displays"), &snapshot)
	if snapshot.Displays[0].Online {
		t.Fatal("timeout was not pushed")
	}
	clock.Add(displayRetention.Milliseconds())
	_ = json.Unmarshal(stream.named(t, "displays"), &snapshot)
	if len(snapshot.Displays) != 0 {
		t.Fatal("retention was not pushed")
	}
}

func TestSSEDefaultDoesNotSendOperatorData(t *testing.T) {
	s := New()
	host := httptest.NewServer(s)
	t.Cleanup(host.Close)
	stream := openTestStream(t, host.URL, "")
	if stream.next(t).name != "state" {
		t.Fatal("initial state missing")
	}
	displayRequest(s, strings.Repeat("b", 32), displayBody(s))
	for range 2 {
		if event := stream.next(t); event.name != "state" {
			t.Fatal("operator data leaked", event.name)
		}
	}
	if r := request(s, "GET", "/timer/events?topics=invalid", "", "", ""); r.Code != 422 {
		t.Fatal(r.Code)
	}
}

func TestSSELogReadFailureRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	log, err := auditlog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = log.Close()
	_ = os.Remove(path)
	s := New()
	s.SetAuditLog(log)
	host := httptest.NewServer(s)
	t.Cleanup(host.Close)
	stream := openTestStream(t, host.URL, "?topics=logs")
	var snapshot struct {
		Error   string           `json:"error"`
		Entries []auditlog.Entry `json:"entries"`
	}
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if snapshot.Error == "" {
		t.Fatal("read error hidden")
	}
	_ = os.WriteFile(path, []byte{}, 0600)
	snapshot.Error = ""
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if snapshot.Error != "" || snapshot.Entries == nil {
		t.Fatal("read did not recover")
	}
}

func TestSSELogsAreBoundedAndUnchangedTopicsAreNotRepeated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var seed bytes.Buffer
	for i := 0; i < 505; i++ {
		data, _ := json.Marshal(auditlog.Entry{Time: time.Unix(int64(i), 0), Type: "operation", Action: fmt.Sprintf("action:%d", i), Result: "success"})
		seed.Write(data)
		seed.WriteByte('\n')
	}
	if err := os.WriteFile(path, seed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := auditlog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	s := New()
	s.SetAuditLog(log)
	host := httptest.NewServer(s)
	t.Cleanup(host.Close)
	stream := openTestStream(t, host.URL, "?topics=displays,logs")
	var snapshot struct {
		Entries []auditlog.Entry `json:"entries"`
	}
	_ = json.Unmarshal(stream.named(t, "logs"), &snapshot)
	if len(snapshot.Entries) != 500 || snapshot.Entries[0].Action != "action:504" || snapshot.Entries[499].Action != "action:5" {
		t.Fatal("wrong history bound")
	}
	for range 2 {
		if event := stream.next(t); event.name != "state" {
			t.Fatal("unchanged topic repeated", event.name)
		}
	}
}
