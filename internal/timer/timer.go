package timer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
	"unicode/utf8"
)

type Message struct {
	Text    string `json:"text"`
	Visible bool   `json:"visible"`
}

type Colors struct {
	Normal   string `json:"normal" validate:"hexcolor"`
	Warning1 string `json:"warning1" validate:"hexcolor"`
	Warning2 string `json:"warning2" validate:"hexcolor"`
	Overtime string `json:"overtime" validate:"hexcolor"`
}

type State struct {
	InstanceID  string  `json:"instanceId"`
	Version     int64   `json:"version"`
	Status      string  `json:"state"`
	Duration    int64   `json:"durationSeconds"`
	Remaining   int64   `json:"remainingMs"`
	ServerTime  int64   `json:"serverTimeMs"`
	Warning1    int64   `json:"warning1Seconds"`
	Warning2    int64   `json:"warning2Seconds"`
	Message     Message `json:"message"`
	DisplayMode string  `json:"displayMode"`
	Language    string  `json:"language"`
	LogLevel    string  `json:"logLevel"`
	Blackout    bool    `json:"blackout"`
	Flash       bool    `json:"flash"`
	BrowserOnly bool    `json:"browserOnly"`
	Colors      Colors  `json:"colors"`
}

type Model struct {
	State
	anchor   time.Time
	presets  []string
	bindings []KeyBinding
}

func New(now time.Time) Model {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		panic(err)
	}
	return Model{State: State{
		InstanceID: hex.EncodeToString(id), Status: "idle", Duration: 600, Remaining: 600000,
		Warning1: 180, Warning2: 60, Blackout: true, DisplayMode: "timer_and_message", Language: "ja", LogLevel: "info",
		Colors: Colors{"#4b5263", "#f2cc60", "#ff9854", "#ff6069"},
	}, anchor: now, presets: []string{}, bindings: defaultBindings()}
}

func (m Model) Snapshot(now time.Time) State {
	s := m.State
	if s.Status == "running" {
		s.Remaining -= now.Sub(m.anchor).Milliseconds()
	}
	s.ServerTime = now.UnixMilli()
	return s
}

func (m Model) Presets() []string { return append([]string{}, m.presets...) }

func (m *Model) SetPresets(presets []string) { m.presets = append([]string{}, presets...) }

func (m Model) Bindings() []KeyBinding { return copyBindings(m.bindings) }

func (m *Model) SetBindings(bindings []KeyBinding) {
	m.bindings = copyBindings(bindings)
}

func copyBindings(bindings []KeyBinding) []KeyBinding {
	result := append([]KeyBinding{}, bindings...)
	for i := range result {
		if result[i].MIDI != nil {
			midi := *result[i].MIDI
			result[i].MIDI = &midi
		}
	}
	return result
}

func (s State) ETag() string { return fmt.Sprintf(`"%s:%d"`, s.InstanceID, s.Version) }

type APIError struct {
	Status        int
	Code, Message string
}

func (e *APIError) Error() string   { return e.Message }
func Invalid(message string) error  { return &APIError{422, "INVALID_ARGUMENT", message} }
func Conflict(message string) error { return &APIError{409, "CONFLICT", message} }

type Command struct {
	Command string `json:"command"`
	Delta   *int64 `json:"deltaSeconds,omitempty"`
}

func (m *Model) Command(c Command, now time.Time) error {
	if c.Command != "adjust" && c.Delta != nil {
		return Invalid("この操作ではdeltaSecondsを指定できません")
	}
	current := m.Snapshot(now).Remaining
	switch c.Command {
	case "start":
		if m.Status != "running" {
			m.Remaining, m.anchor, m.Status = current, now, "running"
		}
	case "pause":
		if m.Status == "running" {
			m.Remaining, m.Status = current, "paused"
		}
	case "reset":
		m.Remaining, m.Status = m.Duration*1000, "idle"
	case "adjust":
		if c.Delta == nil || *c.Delta == 0 || *c.Delta < -3600 || *c.Delta > 3600 {
			return Invalid("加減算は±3600秒以内の非ゼロ整数を指定してください")
		}
		current += *c.Delta * 1000
		if current < -86400000 || current > 86400000 {
			return Invalid("残り時間が±24時間を超えます")
		}
		m.Remaining, m.anchor = current, now
	default:
		return Invalid("不明なコマンドです")
	}
	return nil
}

type Settings struct {
	Duration    *int64  `json:"durationSeconds,omitempty"`
	Warning1    *int64  `json:"warning1Seconds,omitempty"`
	Warning2    *int64  `json:"warning2Seconds,omitempty"`
	DisplayMode *string `json:"displayMode,omitempty"`
	Language    *string `json:"language,omitempty"`
	LogLevel    *string `json:"logLevel,omitempty"`
	Flash       *bool   `json:"flash,omitempty"`
	BrowserOnly *bool   `json:"browserOnly,omitempty"`
	Colors      *Colors `json:"colors,omitempty"`
}

func (m *Model) Configure(p Settings) error {
	if p.Duration != nil {
		if m.Status != "idle" {
			return Conflict("持ち時間の変更前にリセットしてください")
		}
		if *p.Duration < 1 || *p.Duration > 86400 {
			return Invalid("持ち時間は1秒～24時間です")
		}
		m.Duration, m.Remaining = *p.Duration, *p.Duration*1000
	}
	if p.Warning1 != nil {
		m.Warning1 = *p.Warning1
	}
	if p.Warning2 != nil {
		m.Warning2 = *p.Warning2
	}
	if m.Warning2 < 0 || m.Warning1 < m.Warning2 || m.Warning1 > m.Duration {
		return Invalid("警告は 0 ≤ 第2警告 ≤ 第1警告 ≤ 持ち時間 にしてください")
	}
	if p.DisplayMode != nil {
		switch *p.DisplayMode {
		case "timer", "timer_and_message", "message":
			m.DisplayMode = *p.DisplayMode
		default:
			return Invalid("表示モードが不正です")
		}
	}
	if p.Language != nil {
		if *p.Language != "ja" && *p.Language != "en" {
			return Invalid("言語はjaまたはenを指定してください")
		}
		m.Language = *p.Language
	}
	if p.LogLevel != nil {
		switch *p.LogLevel {
		case "debug", "info", "warn", "error":
			m.LogLevel = *p.LogLevel
		default:
			return Invalid("ログレベルはdebug、info、warn、errorのいずれかを指定してください")
		}
	}
	if p.Flash != nil {
		m.Flash = *p.Flash
	}
	if p.BrowserOnly != nil {
		m.BrowserOnly = *p.BrowserOnly
	}
	if p.Colors != nil {
		for _, c := range []string{p.Colors.Normal, p.Colors.Warning1, p.Colors.Warning2, p.Colors.Overtime} {
			if len(c) != 7 || c[0] != '#' {
				return Invalid("色は#RRGGBBで指定してください")
			}
			if _, err := hex.DecodeString(c[1:]); err != nil {
				return Invalid("色は#RRGGBBで指定してください")
			}
		}
		m.Colors = *p.Colors
	}
	return nil
}

func ValidateText(s string) error {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 500 {
		return Invalid("カンペは500文字以内にしてください")
	}
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' {
			return Invalid("使用できない制御文字が含まれています")
		}
	}
	return nil
}
