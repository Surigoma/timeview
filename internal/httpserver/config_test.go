package httpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"timeview/internal/timer"
)

func TestConfigPersistsPreferencesButNotLiveState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timeview-config.json")
	server, err := NewWithConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := server.model.State.ETag()
	settings := `{"durationSeconds":900,"warning1Seconds":300,"warning2Seconds":90,"displayMode":"timer","language":"en","flash":true}`
	stateOf(t, request(server, "PATCH", "/timer", settings, "settings", initial))
	stateOf(t, request(server, "PUT", "/timer/presets", `{"presets":["残り5分","終了してください"]}`, "presets", ""))

	bindings := server.model.Bindings()
	bindings[0].Code = "KeyS"
	body, err := json.Marshal(map[string]any{"bindings": bindings})
	if err != nil {
		t.Fatal(err)
	}
	stateOf(t, request(server, "PUT", "/timer/bindings", string(body), "bindings", ""))
	stateOf(t, request(server, "PUT", "/timer/blackout", `{"enabled":false}`, "reveal", ""))
	stateOf(t, request(server, "PUT", "/timer/message", `{"text":"一時メッセージ"}`, "message", ""))
	running := stateOf(t, request(server, "POST", "/timer/commands", `{"command":"start"}`, "start", ""))
	stateOf(t, browserRequest(server, "PATCH", "/timer", `{"browserOnly":true}`, "browser-only", running.ETag()))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("saved config is not valid JSON")
	}
	for _, runtimeField := range []string{"remainingMs", "blackout", "message", "state"} {
		if strings.Contains(string(data), `"`+runtimeField+`":`) {
			t.Fatalf("runtime field %q was persisted", runtimeField)
		}
	}

	restarted, err := NewWithConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	got := restarted.model
	if got.Duration != 900 || got.Remaining != 900000 || got.Warning1 != 300 || got.Warning2 != 90 || got.DisplayMode != "timer" || got.Language != "en" || !got.Flash || !got.BrowserOnly {
		t.Fatalf("timer config was not restored: %+v", got.State)
	}
	if got.Status != "idle" || !got.Blackout || got.Message != (timer.Message{}) {
		t.Fatalf("live state must start safely: %+v", got.State)
	}
	presets := got.Presets()
	if len(presets) != 2 || presets[0] != "残り5分" {
		t.Fatalf("presets were not restored: %#v", presets)
	}
	bindings = got.Bindings()
	if bindings[0].Code != "KeyS" {
		t.Fatalf("bindings were not restored: %#v", bindings[0])
	}
}

func TestConfigRejectsUnknownAndInvalidFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timeview-config.json")
	server, err := NewWithConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	config["unknown"] = true
	data, _ = json.Marshal(config)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWithConfig(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown config field must fail startup: %v", err)
	}

	if err := timer.Save(path, server.model); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config = nil
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	config["version"] = float64(999)
	data, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWithConfig(path); err == nil || !strings.Contains(err.Error(), "未対応の設定バージョン") {
		t.Fatalf("unsupported config version must fail startup: %v", err)
	}
}

func TestConfigSaveFailureDoesNotChangeState(t *testing.T) {
	server := New()
	server.configPath = filepath.Join(t.TempDir(), "missing", "timeview-config.json")
	etag := server.model.State.ETag()
	response := request(server, "PATCH", "/timer", `{"durationSeconds":720,"warning1Seconds":180,"warning2Seconds":60}`, "settings", etag)
	if response.Code != 500 || !strings.Contains(response.Body.String(), "CONFIG_SAVE_FAILED") {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	if server.model.Duration != 600 || server.model.Version != 0 {
		t.Fatalf("state changed after failed save: %+v", server.model.State)
	}
}
