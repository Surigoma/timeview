package httpserver

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"timeview/internal/timer"
)

func TestBackupAtomicImportAndRuntimeExclusion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := NewWithConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	request(s, "PUT", "/timer/blackout", `{"enabled":false}`, "", "")
	request(s, "PUT", "/timer/message", `{"text":"live"}`, "", "")
	exported := request(s, "GET", "/timer/config", "", "", "")
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(exported.Body.Bytes(), &fields)
	for _, key := range []string{"remainingMs", "blackout", "message", "state", "instanceId"} {
		if _, ok := fields[key]; ok {
			t.Fatal("runtime leaked", key)
		}
	}
	var preferences map[string]json.RawMessage
	_ = json.Unmarshal(fields["timer"], &preferences)
	for _, key := range []string{"remainingMs", "blackout", "message", "state", "instanceId"} {
		if _, ok := preferences[key]; ok {
			t.Fatal("runtime leaked", key)
		}
	}

	source := timer.New(time.Now())
	duration := int64(900)
	_ = source.Configure(timer.Settings{Duration: &duration})
	source.SetPresets([]string{"next"})
	bindings := source.Bindings()
	bindings[0].MIDI = &timer.MIDIBinding{Status: 0x90, Data1: 36}
	source.SetBindings(bindings)
	data, _ := json.Marshal(timer.ExportConfig(source))
	body := string(data)
	before := s.model
	validated := request(s, "POST", "/timer/config/validate", body, "", exported.Header().Get("ETag"))
	if validated.Code != 200 || s.model.Version != before.Version || s.model.Duration != before.Duration {
		t.Fatalf("preview mutated: %s", validated.Body)
	}
	if r := request(s, "PATCH", "/timer/config", body, "", ""); r.Code != 428 {
		t.Fatal(r.Code)
	}
	imported := request(s, "PATCH", "/timer/config", body, "import", exported.Header().Get("ETag"))
	state := stateOf(t, imported)
	if state.Duration != 900 || state.Blackout || state.Message.Text != "live" || state.InstanceID != before.InstanceID {
		t.Fatal(state)
	}
	replay := request(s, "PATCH", "/timer/config", body, "import", exported.Header().Get("ETag"))
	if replay.Body.String() != imported.Body.String() {
		t.Fatal("idempotency lost")
	}
	if r := request(s, "PATCH", "/timer/config", body, "", exported.Header().Get("ETag")); r.Code != 412 {
		t.Fatal(r.Code)
	}
	loaded, err := timer.Load(path, time.Now())
	if err != nil || loaded.Duration != 900 || loaded.Presets()[0] != "next" || loaded.Bindings()[0].MIDI.Data1 != 36 || !loaded.Blackout || loaded.Message.Text != "" {
		t.Fatalf("restore: %+v %v", loaded, err)
	}
}

func TestBackupRejectsInvalidAndActiveImports(t *testing.T) {
	for _, bad := range []string{`{}`, `{"version":2}`, `{"state":"running"}`, `{"timer":null}`, `{"timer":{"durationSeconds":600.5}}`, `{`, strings.Repeat(" ", 65537)} {
		s := New()
		original := request(s, "GET", "/timer/config", "", "", "")
		r := request(s, "PATCH", "/timer/config", bad, "", s.model.State.ETag())
		if r.Code < 400 || request(s, "GET", "/timer/config", "", "", "").Body.String() != original.Body.String() {
			t.Fatalf("accepted invalid backup: %d", r.Code)
		}
	}
	for _, status := range []string{"running", "paused"} {
		s := New()
		backup := request(s, "GET", "/timer/config", "", "", "").Body.String()
		s.model.Status = status
		if r := request(s, "PATCH", "/timer/config", backup, "", s.model.State.ETag()); r.Code != http.StatusConflict {
			t.Fatal(r.Code)
		}
	}
	s := New()
	s.model.BrowserOnly = true
	backup := request(s, "GET", "/timer/config", "", "", "").Body.String()
	if r := request(s, "PATCH", "/timer/config", backup, "", s.model.State.ETag()); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if r := requestAs(s, "PATCH", "/timer/config", backup, "", s.model.State.ETag(), true); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
}

func TestBackupSaveFailureDoesNotCommit(t *testing.T) {
	s := New()
	s.configPath = filepath.Join(t.TempDir(), "missing", "config.json")
	source := timer.New(time.Now())
	source.SetPresets([]string{"change"})
	data, _ := json.Marshal(timer.ExportConfig(source))
	r := request(s, "PATCH", "/timer/config", string(data), "", s.model.State.ETag())
	if r.Code != 500 || len(s.model.Presets()) != 0 || s.model.Version != 0 {
		t.Fatal(r.Code, s.model.Presets())
	}
}
