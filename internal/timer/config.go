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
	Action string `json:"action" validate:"required,oneof=start pause add subtract reset hide show clear blackout reveal preset0 preset1 preset2 preset3 preset4 preset5 preset6 preset7 preset8"`
	Code   string `json:"code" validate:"required,max=64"`
	Ctrl   bool   `json:"ctrl"`
	Shift  bool   `json:"shift"`
	Alt    bool   `json:"alt"`
	Meta   bool   `json:"meta"`
}

type timerConfig struct {
	Duration    int64  `json:"durationSeconds" validate:"gte=1,lte=86400"`
	Warning1    int64  `json:"warning1Seconds" validate:"gte=0,lte=86400"`
	Warning2    int64  `json:"warning2Seconds" validate:"gte=0,lte=86400"`
	DisplayMode string `json:"displayMode" validate:"oneof=timer timer_and_message message"`
	Flash       bool   `json:"flash"`
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

func validatePresets(presets []string) error {
	if err := configValidator.Var(presets, "max=9,dive,max=500"); err != nil {
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
	if err := configValidator.Var(bindings, "len=19,dive"); err != nil {
		return invalid("キー割り当ての内容が不正です")
	}
	seenActions := make(map[string]struct{}, len(bindings))
	seenKeys := make(map[KeyBinding]struct{}, len(bindings))
	for _, binding := range bindings {
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
	if errors.Is(err, os.ErrNotExist) {
		return m, saveConfig(path, m)
	}
	if err != nil {
		return model{}, fmt.Errorf("設定ファイルを読み込めません: %w", err)
	}
	if !utf8.Valid(data) {
		return model{}, fmt.Errorf("設定ファイルはUTF-8で保存してください")
	}
	configStore := koanf.New(".")
	if err := configStore.Load(rawbytes.Provider(data), kjson.Parser()); err != nil {
		return model{}, fmt.Errorf("設定ファイルのJSONが不正です: %w", err)
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
		return model{}, fmt.Errorf("%s: %w", message, err)
	}
	if config.Version != configVersion {
		return model{}, fmt.Errorf("未対応の設定バージョンです: %d", config.Version)
	}
	if err := configValidator.Struct(config); err != nil {
		return model{}, fmt.Errorf("設定ファイルの値が不正です: %w", err)
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
