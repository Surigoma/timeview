package timer

import (
	"strings"
	"testing"
	"time"
)

func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }
func boolPointer(value bool) *bool       { return &value }

func TestCommandValidationAndNoOps(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		name    string
		prepare func(*Model)
		command Command
	}{
		{"delta with start", nil, Command{Command: "start", Delta: int64Pointer(1)}},
		{"missing delta", nil, Command{Command: "adjust"}},
		{"zero delta", nil, Command{Command: "adjust", Delta: int64Pointer(0)}},
		{"large delta", nil, Command{Command: "adjust", Delta: int64Pointer(3601)}},
		{"remaining overflow", func(m *Model) { m.Remaining = 86400000 }, Command{Command: "adjust", Delta: int64Pointer(1)}},
		{"unknown command", nil, Command{Command: "launch"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := New(now)
			if test.prepare != nil {
				test.prepare(&m)
			}
			if err := m.Command(test.command, now); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	m := New(now)
	m.Status = "running"
	m.anchor = now
	if err := m.Command(Command{Command: "start"}, now.Add(time.Second)); err != nil || m.anchor != now {
		t.Fatalf("running start changed state: %+v, %v", m.State, err)
	}
	m.Status = "idle"
	if err := m.Command(Command{Command: "pause"}, now); err != nil || m.Status != "idle" {
		t.Fatalf("idle pause changed state: %+v, %v", m.State, err)
	}
}

func TestConfigureValidation(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		name    string
		prepare func(*Model)
		input   Settings
	}{
		{"duration too small", nil, Settings{Duration: int64Pointer(0)}},
		{"duration while active", func(m *Model) { m.Status = "running" }, Settings{Duration: int64Pointer(30)}},
		{"warning order", nil, Settings{Warning1: int64Pointer(30)}},
		{"display mode", nil, Settings{DisplayMode: stringPointer("clock")}},
		{"language", nil, Settings{Language: stringPointer("fr")}},
		{"log level", nil, Settings{LogLevel: stringPointer("trace")}},
		{"short color", nil, Settings{Colors: &Colors{Normal: "#fff", Warning1: "#112233", Warning2: "#223344", Overtime: "#334455"}}},
		{"invalid color", nil, Settings{Colors: &Colors{Normal: "#xxxxxx", Warning1: "#112233", Warning2: "#223344", Overtime: "#334455"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := New(now)
			if test.prepare != nil {
				test.prepare(&m)
			}
			if err := m.Configure(test.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	m := New(now)
	colors := Colors{"#010203", "#112233", "#223344", "#334455"}
	input := Settings{DisplayMode: stringPointer("message"), Language: stringPointer("en"), LogLevel: stringPointer("debug"), Flash: boolPointer(true), BrowserOnly: boolPointer(true), Colors: &colors}
	if err := m.Configure(input); err != nil {
		t.Fatal(err)
	}
	if m.DisplayMode != "message" || m.Language != "en" || m.LogLevel != "debug" || !m.Flash || !m.BrowserOnly || m.Colors != colors {
		t.Fatalf("settings not applied: %+v", m.State)
	}
}

func TestTextPresetAndBindingValidation(t *testing.T) {
	if err := ValidateText(string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	}
	if err := ValidateText("bad\x00text"); err == nil {
		t.Fatal("control character was accepted")
	}
	if err := ValidateText("line one\nline two\tend"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePresets(make([]string, 10)); err == nil {
		t.Fatal("too many presets were accepted")
	}
	if err := ValidatePresets([]string{strings.Repeat("あ", 501)}); err == nil {
		t.Fatal("oversized preset was accepted")
	}

	bindings := defaultBindings()
	duplicateAction := append([]KeyBinding{}, bindings...)
	duplicateAction[1].Action = duplicateAction[0].Action
	duplicateKey := append([]KeyBinding{}, bindings...)
	duplicateKey[1].Code = duplicateKey[0].Code
	resetWithoutModifier := append([]KeyBinding{}, bindings...)
	resetWithoutModifier[4].Ctrl = false
	invalidUTF8 := append([]KeyBinding{}, bindings...)
	invalidUTF8[0].Code = string([]byte{0xff})
	for name, candidate := range map[string][]KeyBinding{
		"duplicate action": duplicateAction,
		"duplicate key":    duplicateKey,
		"unsafe reset":     resetWithoutModifier,
		"invalid UTF-8":    invalidUTF8,
		"wrong count":      bindings[:18],
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateBindings(candidate); err == nil {
				t.Fatal("invalid bindings were accepted")
			}
		})
	}
}
