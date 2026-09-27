package systemlog

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSetLevelAndFiltering(t *testing.T) {
	var output bytes.Buffer
	level := new(slog.LevelVar)
	if err := SetLevel(level, "warn"); err != nil {
		t.Fatal(err)
	}
	logger := New(&output, level, true)
	logger.Info("hidden")
	logger.Warn("shown", "key", "value")
	if got := output.String(); strings.Contains(got, "hidden") || !strings.Contains(got, "WRN shown key=value") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestSetLevelRejectsUnknownValue(t *testing.T) {
	if err := SetLevel(new(slog.LevelVar), "trace"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSetLevelAcceptsSupportedValues(t *testing.T) {
	for name, want := range map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	} {
		t.Run(name, func(t *testing.T) {
			level := new(slog.LevelVar)
			if err := SetLevel(level, name); err != nil {
				t.Fatal(err)
			}
			if level.Level() != want {
				t.Fatalf("level = %s, want %s", level.Level(), want)
			}
		})
	}
}
