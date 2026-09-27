//go:build cgo && (windows || darwin || linux)

package main

import (
	"errors"
	"sync"
	"time"

	hook "github.com/robotn/gohook"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

type nativeInputs struct {
	events    chan<- inputEvent
	stopOnce  sync.Once
	midiStops []func()
	midiLatch midiLatch
}

func startPlatformInputs(events chan<- inputEvent) (func(), int, error) {
	inputs := &nativeInputs{events: events}
	keyboard := hook.Start()
	select {
	case event := <-keyboard:
		if event.Kind != hook.HookEnabled {
			return nil, 0, errors.New("キーボード入力の監視を開始できません")
		}
	case <-time.After(2 * time.Second):
		return nil, 0, errors.New("キーボード入力の監視を開始できません。macOSではアクセシビリティ、LinuxではX11の設定を確認してください")
	}
	go inputs.readKeyboard(keyboard)

	ports, _ := drivers.Ins()
	for source, port := range ports {
		stop, err := midi.ListenTo(port, func(message midi.Message, _ int32) {
			inputs.readMIDI(source, message)
		})
		if err == nil {
			inputs.midiStops = append(inputs.midiStops, stop)
		}
	}
	return inputs.stop, len(inputs.midiStops), nil
}

func (i *nativeInputs) readKeyboard(events <-chan hook.Event) {
	down := make(map[uint16]bool)
	for event := range events {
		switch event.Kind {
		case hook.KeyDown:
			if down[event.Keycode] {
				continue
			}
			down[event.Keycode] = true
			input := keyboardEvent(event.Keycode, event.Mask)
			if input.Code != "" {
				i.emit(input)
			}
		case hook.KeyUp:
			delete(down, event.Keycode)
		}
	}
}

func (i *nativeInputs) readMIDI(source int, message midi.Message) {
	var channel, data1, value uint8
	var status uint8
	switch {
	case message.GetNoteOn(&channel, &data1, &value):
		status = 0x90 | channel
	case message.GetNoteOff(&channel, &data1, &value):
		status = 0x80 | channel
	case message.GetControlChange(&channel, &data1, &value):
		status = 0xb0 | channel
	default:
		return
	}
	if binding, ok := i.midiLatch.press(source, status, data1, value); ok {
		i.emit(inputEvent{MIDI: &binding})
	}
}

func (i *nativeInputs) emit(event inputEvent) {
	select {
	case i.events <- event:
	default:
	}
}

func (i *nativeInputs) stop() {
	i.stopOnce.Do(func() {
		for _, stop := range i.midiStops {
			stop()
		}
		midi.CloseDriver()
		hook.End()
	})
}
