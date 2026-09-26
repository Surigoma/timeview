package timer

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func updateSettings(next *model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload settings
	if err := decode(data, &payload); err != nil {
		return err
	}
	return next.configure(payload)
}

func runCommand(next *model, c *gin.Context, data []byte, now time.Time) error {
	var payload command
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Command == "adjust" && c.GetHeader("Idempotency-Key") == "" {
		return &apiError{428, "PRECONDITION_REQUIRED", "加減算にはIdempotency-Keyが必要です"}
	}
	return next.command(payload, now)
}

func updateBlackout(next *model, _ *gin.Context, data []byte, _ time.Time) error {
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

func updateMessage(next *model, c *gin.Context, data []byte, _ time.Time) error {
	if c.Request.Method == http.MethodDelete {
		if len(bytes.TrimSpace(data)) > 0 {
			return invalid("消去では本文を指定しないでください")
		}
		next.Message = Message{}
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
		next.Message = Message{*payload.Text, *payload.Text != ""}
	}
	if payload.Visible != nil {
		next.Message.Visible = *payload.Visible
	}
	if payload.Text != nil && *payload.Text == "" {
		next.Message.Visible = false
	}
	if err := validateText(next.Message.Text); err != nil {
		return err
	}
	if next.Message.Visible && next.Message.Text == "" {
		return invalid("空のカンペは再表示できません")
	}
	return nil
}

func updatePresets(next *model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload struct {
		Presets *[]string `json:"presets"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Presets == nil {
		return invalid("presetsを指定してください")
	}
	if err := validatePresets(*payload.Presets); err != nil {
		return err
	}
	next.presets = append([]string{}, (*payload.Presets)...)
	return nil
}

func updateBindings(next *model, _ *gin.Context, data []byte, _ time.Time) error {
	var payload struct {
		Bindings *[]KeyBinding `json:"bindings"`
	}
	if err := decode(data, &payload); err != nil {
		return err
	}
	if payload.Bindings == nil {
		return invalid("bindingsを指定してください")
	}
	if err := validateBindings(*payload.Bindings); err != nil {
		return err
	}
	next.bindings = append([]KeyBinding{}, (*payload.Bindings)...)
	return nil
}
