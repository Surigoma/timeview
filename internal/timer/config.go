package timer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	kjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const configVersion = 1

var configValidator = validator.New(validator.WithRequiredStructEnabled())

type KeyBinding struct {
	Action string       `json:"action" validate:"required,oneof=start pause add subtract reset hide show clear blackout reveal preset0 preset1 preset2 preset3 preset4 preset5 preset6 preset7 preset8"`
	Code   string       `json:"code" validate:"required,max=64"`
	Ctrl   bool         `json:"ctrl"`
	Shift  bool         `json:"shift"`
	Alt    bool         `json:"alt"`
	Meta   bool         `json:"meta"`
	MIDI   *MIDIBinding `json:"midi,omitempty"`
}

type MIDIBinding struct {
	Status uint8 `json:"status"`
	Data1  uint8 `json:"data1"`
}

type timerConfig struct {
	Duration    int64  `json:"durationSeconds" validate:"gte=1,lte=86400"`
	Warning1    int64  `json:"warning1Seconds" validate:"gte=0,lte=86400"`
	Warning2    int64  `json:"warning2Seconds" validate:"gte=0,lte=86400"`
	DisplayMode string `json:"displayMode" validate:"oneof=timer timer_and_message message"`
	Language    string `json:"language,omitempty" validate:"omitempty,oneof=ja en"`
	LogLevel    string `json:"logLevel,omitempty" validate:"omitempty,oneof=debug info warn error"`
	Flash       bool   `json:"flash"`
	BrowserOnly bool   `json:"browserOnly"`
	Colors      Colors `json:"colors" validate:"required"`
}

type fileConfig struct {
	Version        int          `json:"version" validate:"eq=1"`
	Timer          timerConfig  `json:"timer" validate:"required"`
	Presets        []string     `json:"presets" validate:"max=9,dive,max=500"`
	KeypadBindings []KeyBinding `json:"keypadBindings" validate:"len=19,dive"`
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

func ValidatePresets(presets []string) error {
	if err := configValidator.Var(presets, "max=9,dive,max=500"); err != nil {
		return Invalid("定型文は最大9件です")
	}
	for _, text := range presets {
		if err := ValidateText(text); err != nil {
			return err
		}
	}
	return nil
}

func ValidateBindings(bindings []KeyBinding) error {
	if err := configValidator.Var(bindings, "len=19,dive"); err != nil {
		return Invalid("キー割り当ての内容が不正です")
	}
	seenActions := make(map[string]struct{}, len(bindings))
	type keyboardBinding struct {
		Code                   string
		Ctrl, Shift, Alt, Meta bool
	}
	seenKeys := make(map[keyboardBinding]struct{}, len(bindings))
	seenMIDI := make(map[MIDIBinding]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, ok := seenActions[binding.Action]; ok {
			return Invalid("キー操作が重複しています")
		}
		seenActions[binding.Action] = struct{}{}
		if !utf8.ValidString(binding.Code) || len(binding.Code) == 0 || utf8.RuneCountInString(binding.Code) > 64 {
			return Invalid("キーコードは1～64文字にしてください")
		}
		key := keyboardBinding{binding.Code, binding.Ctrl, binding.Shift, binding.Alt, binding.Meta}
		if _, ok := seenKeys[key]; ok {
			return Invalid("同じキーが複数の操作に割り当てられています")
		}
		seenKeys[key] = struct{}{}
		if binding.MIDI != nil {
			kind := binding.MIDI.Status & 0xf0
			if (kind != 0x90 && kind != 0xb0) || binding.MIDI.Data1 > 127 {
				return Invalid("MIDI割り当てはNote OnまたはControl Changeを指定してください")
			}
			if _, ok := seenMIDI[*binding.MIDI]; ok {
				return Invalid("同じMIDI入力が複数の操作に割り当てられています")
			}
			seenMIDI[*binding.MIDI] = struct{}{}
		}
		if binding.Action == "reset" && !binding.Ctrl && !binding.Shift && !binding.Alt && !binding.Meta {
			return Invalid("リセットには修飾キーが必要です")
		}
	}
	return nil
}

func configFromModel(m Model) fileConfig {
	return fileConfig{
		Version: configVersion,
		Timer: timerConfig{
			Duration: m.Duration, Warning1: m.Warning1, Warning2: m.Warning2,
			DisplayMode: m.DisplayMode, Language: m.Language, LogLevel: m.LogLevel, Flash: m.Flash, BrowserOnly: m.BrowserOnly, Colors: m.Colors,
		},
		Presets:        m.Presets(),
		KeypadBindings: m.Bindings(),
	}
}

func Load(path string, now time.Time) (Model, error) {
	m := New(now)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, Save(path, m)
	}
	if err != nil {
		return Model{}, fmt.Errorf("設定ファイルを読み込めません: %w", err)
	}
	if !utf8.Valid(data) {
		return Model{}, fmt.Errorf("設定ファイルはUTF-8で保存してください")
	}
	configStore := koanf.New(".")
	if err := configStore.Load(rawbytes.Provider(data), kjson.Parser()); err != nil {
		return Model{}, fmt.Errorf("設定ファイルのJSONが不正です: %w", err)
	}
	var config fileConfig
	if err := configStore.UnmarshalWithConf("", &config, koanf.UnmarshalConf{
		Tag: "json",
		DecoderConfig: &mapstructure.DecoderConfig{
			ErrorUnused:      true,
			WeaklyTypedInput: false,
			ZeroFields:       true,
		},
	}); err != nil {
		message := "設定ファイルのJSONが不正です"
		if strings.Contains(err.Error(), "invalid keys") {
			message = "設定ファイルにunknown fieldがあります"
		}
		return Model{}, fmt.Errorf("%s: %w", message, err)
	}
	if config.Version != configVersion {
		return Model{}, fmt.Errorf("未対応の設定バージョンです: %d", config.Version)
	}
	if err := configValidator.Struct(config); err != nil {
		return Model{}, fmt.Errorf("設定ファイルの値が不正です: %w", err)
	}
	settings := Settings{
		Duration: &config.Timer.Duration, Warning1: &config.Timer.Warning1,
		Warning2: &config.Timer.Warning2, DisplayMode: &config.Timer.DisplayMode,
		Flash: &config.Timer.Flash, BrowserOnly: &config.Timer.BrowserOnly, Colors: &config.Timer.Colors,
	}
	if config.Timer.Language != "" {
		settings.Language = &config.Timer.Language
	}
	if config.Timer.LogLevel != "" {
		settings.LogLevel = &config.Timer.LogLevel
	}
	if err := m.Configure(settings); err != nil {
		return Model{}, fmt.Errorf("設定ファイルのタイマー設定が不正です: %w", err)
	}
	if err := ValidatePresets(config.Presets); err != nil {
		return Model{}, fmt.Errorf("設定ファイルの定型文が不正です: %w", err)
	}
	if err := ValidateBindings(config.KeypadBindings); err != nil {
		return Model{}, fmt.Errorf("設定ファイルのキー割り当てが不正です: %w", err)
	}
	m.SetPresets(config.Presets)
	m.SetBindings(config.KeypadBindings)
	return m, nil
}

func Save(path string, m Model) error {
	configStore := koanf.New(".")
	if err := configStore.Load(structs.Provider(configFromModel(m), "json"), nil); err != nil {
		return err
	}
	data, err := configStore.Marshal(kjson.Parser())
	if err != nil {
		return err
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, data, "", "  "); err != nil {
		return err
	}
	formatted.WriteByte('\n')
	data = formatted.Bytes()
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
