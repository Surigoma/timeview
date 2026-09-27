package timer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	path := filepath.Join(t.TempDir(), "timeview-config.json")

	model, err := Load(path, now)
	if err != nil {
		t.Fatal(err)
	}
	duration, warning1, warning2 := int64(900), int64(300), int64(90)
	displayMode, language, logLevel, flash, browserOnly := "message", "en", "debug", true, true
	colors := Colors{"#010203", "#112233", "#223344", "#334455"}
	if err := model.Configure(Settings{
		Duration: &duration, Warning1: &warning1, Warning2: &warning2,
		DisplayMode: &displayMode, Language: &language, LogLevel: &logLevel, Flash: &flash, BrowserOnly: &browserOnly, Colors: &colors,
	}); err != nil {
		t.Fatal(err)
	}
	model.SetPresets([]string{"five minutes", "finish"})
	bindings := model.Bindings()
	bindings[0].Code = "KeyS"
	bindings[0].MIDI = &MIDIBinding{Status: 0x90, Data1: 36}
	model.SetBindings(bindings)
	if err := Save(path, model); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Duration != duration || loaded.Remaining != duration*1000 || loaded.Warning1 != warning1 || loaded.Warning2 != warning2 {
		t.Fatalf("timer settings were not restored: %+v", loaded.State)
	}
	if loaded.DisplayMode != displayMode || loaded.Language != language || loaded.LogLevel != logLevel || !loaded.Flash || !loaded.BrowserOnly || loaded.Colors != colors {
		t.Fatalf("display settings were not restored: %+v", loaded.State)
	}
	if presets := loaded.Presets(); len(presets) != 2 || presets[0] != "five minutes" {
		t.Fatalf("presets were not restored: %#v", presets)
	}
	if got := loaded.Bindings(); got[0].Code != "KeyS" || got[0].MIDI == nil || got[0].MIDI.Data1 != 36 {
		t.Fatalf("bindings were not restored: %#v", got[0])
	}
}

func TestLegacyConfigUsesNewDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timeview-config.json")
	model, err := Load(path, time.Now())
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
	delete(config["timer"].(map[string]any), "language")
	delete(config["timer"].(map[string]any), "logLevel")
	data, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Language != "ja" || loaded.LogLevel != "info" || model.Language != "ja" || model.LogLevel != "info" {
		t.Fatalf("legacy config defaults = language %q, log level %q", loaded.Language, loaded.LogLevel)
	}
}

func TestModelSlicesAreCopied(t *testing.T) {
	model := New(time.Now())
	presets := []string{"original"}
	model.SetPresets(presets)
	presets[0] = "changed"
	gotPresets := model.Presets()
	gotPresets[0] = "also changed"
	if model.Presets()[0] != "original" {
		t.Fatal("presets share caller-owned storage")
	}

	bindings := model.Bindings()
	bindings[0].Code = "KeyS"
	if model.Bindings()[0].Code == "KeyS" {
		t.Fatal("bindings share caller-owned storage")
	}
	midi := MIDIBinding{Status: 0x90, Data1: 36}
	bindings[0].MIDI = &midi
	model.SetBindings(bindings)
	bindings[0].MIDI.Data1 = 37
	gotBindings := model.Bindings()
	gotBindings[0].MIDI.Data1 = 38
	if model.Bindings()[0].MIDI.Data1 != 36 {
		t.Fatal("MIDI bindings share caller-owned storage")
	}

	if model.State.ETag() == "" {
		t.Fatal("ETag must not be empty")
	}
	if got := Invalid("message").Error(); got != "message" {
		t.Fatalf("unexpected API error: %q", got)
	}
}
