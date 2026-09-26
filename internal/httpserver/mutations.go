package httpserver

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"timeview/internal/timer"
)

func updateSettings(next *timer.Model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload timer.Settings
	if err := decode(data, &payload); err != nil {
		return err
	}
	return next.Configure(payload)
}

func runCommand(next *timer.Model, c *gin.Context, data []byte, now time.Time) error {
	var payload timer.Command
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Command == "adjust" && c.GetHeader("Idempotency-Key") == "" {
		return &apiError{Status: 428, Code: "PRECONDITION_REQUIRED", Message: "加減算にはIdempotency-Keyが必要です"}
	}
	return next.Command(payload, now)
}

func updateBlackout(next *timer.Model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Enabled == nil {
		return invalid("enabledを指定してください")
	}
	next.Blackout = *payload.Enabled
	return nil
}

func updateMessage(next *timer.Model, c *gin.Context, data []byte, _ time.Time) error {
	if c.Request.Method == http.MethodDelete {
		if len(bytes.TrimSpace(data)) > 0 {
			return invalid("消去では本文を指定しないでください")
		}
		next.Message = timer.Message{}
		return nil
	}

	var payload struct {
		Text    *string `json:"text"`
		Visible *bool   `json:"visible"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Text == nil && payload.Visible == nil {
		return invalid("textまたはvisibleを指定してください")
	}
	if payload.Text != nil {
		next.Message = timer.Message{Text: *payload.Text, Visible: *payload.Text != ""}
	}
	if payload.Visible != nil {
		next.Message.Visible = *payload.Visible
	}
	if payload.Text != nil && *payload.Text == "" {
		next.Message.Visible = false
	}
	if err := timer.ValidateText(next.Message.Text); err != nil {
		return err
	}
	if next.Message.Visible && next.Message.Text == "" {
		return invalid("空のカンペは再表示できません")
	}
	return nil
}

func updatePresets(next *timer.Model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload struct {
		Presets *[]string `json:"presets"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Presets == nil {
		return invalid("presetsを指定してください")
	}
	if err := timer.ValidatePresets(*payload.Presets); err != nil {
		return err
	}
	next.SetPresets(*payload.Presets)
	return nil
}

func updateBindings(next *timer.Model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload struct {
		Bindings *[]timer.KeyBinding `json:"bindings"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Bindings == nil {
		return invalid("bindingsを指定してください")
	}
	if err := timer.ValidateBindings(*payload.Bindings); err != nil {
		return err
	}
	next.SetBindings(*payload.Bindings)
	return nil
}
