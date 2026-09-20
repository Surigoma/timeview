package timer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

const configVersion = 1

type KeyBinding struct {
	Action string `json:"action"`
	Code   string `json:"code"`
	Ctrl   bool   `json:"ctrl"`
	Shift  bool   `json:"shift"`
	Alt    bool   `json:"alt"`
	Meta   bool   `json:"meta"`
}

type timerConfig struct {
	Duration    int64  `json:"durationSeconds"`
	Warning1    int64  `json:"warning1Seconds"`
	Warning2    int64  `json:"warning2Seconds"`
	DisplayMode string `json:"displayMode"`
	Flash       bool   `json:"flash"`
	Colors      Colors `json:"colors"`
}

type fileConfig struct {
	Version        int          `json:"version"`
	Timer          timerConfig  `json:"timer"`
	Presets        []string     `json:"presets"`
	KeypadBindings []KeyBinding `json:"keypadBindings"`
}

func defaultBindings() []KeyBinding {
	bindings := []KeyBinding{
		{Action: "start", Code: "NumpadEnter"},
		{Action: "pause", Code: "NumpadDecimal"},
		{Action: "add", Code: "NumpadAdd"},
		{Action: "subtract", Code: "NumpadSubtract"},
		{Action: "reset", Code: "Numpad0", Ctrl: true},
		{Action: "hide", Code: "Numpad0"},
		{Action: "show", Code: "NumpadMultiply"},
		{Action: "clear", Code: "NumpadDivide"},
		{Action: "blackout", Code: "NumpadSubtract", Ctrl: true},
		{Action: "reveal", Code: "NumpadAdd", Ctrl: true},
	}
	for i := range 9 {
		bindings = append(bindings, KeyBinding{Action: fmt.Sprintf("preset%d", i), Code: fmt.Sprintf("Numpad%d", i+1)})
	}
	return bindings
}

func validatePresets(presets []string) error {
	if len(presets) > 9 {
		return invalid("定型文は最大9件です")
	}
	for _, text := range presets {
		if err := validateText(text); err != nil {
			return err
		}
	}
	return nil
}

func validateBindings(bindings []KeyBinding) error {
	defaults := defaultBindings()
	if len(bindings) != len(defaults) {
		return invalid("キー割り当ての件数が不正です")
	}
	actions := make(map[string]struct{}, len(defaults))
	for _, binding := range defaults {
		actions[binding.Action] = struct{}{}
	}
	seenActions := make(map[string]struct{}, len(bindings))
	seenKeys := make(map[KeyBinding]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, ok := actions[binding.Action]; !ok {
			return invalid("不明なキー操作が含まれています")
		}
		if _, ok := seenActions[binding.Action]; ok {
			return invalid("キー操作が重複しています")
		}
		seenActions[binding.Action] = struct{}{}
		if !utf8.ValidString(binding.Code) || len(binding.Code) == 0 || utf8.RuneCountInString(binding.Code) > 64 {
			return invalid("キーコードは1～64文字にしてください")
		}
		key := binding
		key.Action = ""
		if _, ok := seenKeys[key]; ok {
			return invalid("同じキーが複数の操作に割り当てられています")
		}
		seenKeys[key] = struct{}{}
		if binding.Action == "reset" && !binding.Ctrl && !binding.Shift && !binding.Alt && !binding.Meta {
			return invalid("リセットには修飾キーが必要です")
		}
	}
	return nil
}

func configFromModel(m model) fileConfig {
	return fileConfig{
		Version: configVersion,
		Timer: timerConfig{
			Duration: m.Duration, Warning1: m.Warning1, Warning2: m.Warning2,
			DisplayMode: m.DisplayMode, Flash: m.Flash, Colors: m.Colors,
		},
		Presets:        append([]string{}, m.presets...),
		KeypadBindings: append([]KeyBinding{}, m.bindings...),
	}
}

func loadModel(path string, now time.Time) (model, error) {
	m := newModel(now)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m, saveConfig(path, m)
	}
	if err != nil {
		return model{}, fmt.Errorf("設定ファイルを読み込めません: %w", err)
	}
	if !utf8.Valid(data) {
		return model{}, fmt.Errorf("設定ファイルはUTF-8で保存してください")
	}
	var config fileConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return model{}, fmt.Errorf("設定ファイルのJSONが不正です: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return model{}, fmt.Errorf("設定ファイルにはJSONオブジェクトを1つだけ指定してください")
	}
	if config.Version != configVersion {
		return model{}, fmt.Errorf("未対応の設定バージョンです: %d", config.Version)
	}
	settings := settings{
		Duration: &config.Timer.Duration, Warning1: &config.Timer.Warning1,
		Warning2: &config.Timer.Warning2, DisplayMode: &config.Timer.DisplayMode,
		Flash: &config.Timer.Flash, Colors: &config.Timer.Colors,
	}
	if err := m.configure(settings); err != nil {
		return model{}, fmt.Errorf("設定ファイルのタイマー設定が不正です: %w", err)
	}
	if err := validatePresets(config.Presets); err != nil {
		return model{}, fmt.Errorf("設定ファイルの定型文が不正です: %w", err)
	}
	if err := validateBindings(config.KeypadBindings); err != nil {
		return model{}, fmt.Errorf("設定ファイルのキー割り当てが不正です: %w", err)
	}
	m.presets = append([]string{}, config.Presets...)
	m.bindings = append([]KeyBinding{}, config.KeypadBindings...)
	return m, nil
}

func saveConfig(path string, m model) error {
	data, err := json.MarshalIndent(configFromModel(m), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".timeview-config-*")
	if err != nil {
		return fmt.Errorf("設定ファイルを作成できません: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	_, err = temporary.Write(data)
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("設定ファイルを書き込めません: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("設定ファイルを置き換えられません: %w", err)
	}
	return nil
}
