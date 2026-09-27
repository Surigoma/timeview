package systemlog

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"
)

func New(w io.Writer, level slog.Leveler, noColor bool) *slog.Logger {
	return slog.New(tint.NewHandler(w, &tint.Options{
		Level:      level,
		TimeFormat: time.DateTime,
		NoColor:    noColor,
	}))
}

func SetLevel(level *slog.LevelVar, name string) error {
	var value slog.Level
	switch name {
	case "debug":
		value = slog.LevelDebug
	case "info":
		value = slog.LevelInfo
	case "warn":
		value = slog.LevelWarn
	case "error":
		value = slog.LevelError
	default:
		return fmt.Errorf("不明なログレベルです: %s", name)
	}
	level.Set(value)
	return nil
}
