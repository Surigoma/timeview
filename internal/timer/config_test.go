package timer

import (
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
	displayMode, flash, browserOnly := "message", true, true
	colors := Colors{"#010203", "#112233", "#223344", "#334455"}
	if err := model.Configure(Settings{
		Duration: &duration, Warning1: &warning1, Warning2: &warning2,
		DisplayMode: &displayMode, Flash: &flash, BrowserOnly: &browserOnly, Colors: &colors,
	}); err != nil {
		t.Fatal(err)
	}
	model.SetPresets([]string{"five minutes", "finish"})
	bindings := model.Bindings()
	bindings[0].Code = "KeyS"
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
	if loaded.DisplayMode != displayMode || !loaded.Flash || !loaded.BrowserOnly || loaded.Colors != colors {
		t.Fatalf("display settings were not restored: %+v", loaded.State)
	}
	if presets := loaded.Presets(); len(presets) != 2 || presets[0] != "five minutes" {
		t.Fatalf("presets were not restored: %#v", presets)
	}
	if got := loaded.Bindings(); got[0].Code != "KeyS" {
		t.Fatalf("bindings were not restored: %#v", got[0])
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

	if model.State.ETag() == "" {
		t.Fatal("ETag must not be empty")
	}
	if got := Invalid("message").Error(); got != "message" {
		t.Fatalf("unexpected API error: %q", got)
	}
}
