package main

import "sync"

const (
	maskShift = 1<<0 | 1<<4
	maskCtrl  = 1<<1 | 1<<5
	maskMeta  = 1<<2 | 1<<6
	maskAlt   = 1<<3 | 1<<7
)

var inputCodes = map[uint16]string{
	0x0001: "Escape", 0x0002: "Digit1", 0x0003: "Digit2", 0x0004: "Digit3", 0x0005: "Digit4", 0x0006: "Digit5", 0x0007: "Digit6", 0x0008: "Digit7", 0x0009: "Digit8", 0x000a: "Digit9", 0x000b: "Digit0",
	0x000c: "Minus", 0x000d: "Equal", 0x000e: "Backspace", 0x000f: "Tab", 0x0010: "KeyQ", 0x0011: "KeyW", 0x0012: "KeyE", 0x0013: "KeyR", 0x0014: "KeyT", 0x0015: "KeyY", 0x0016: "KeyU", 0x0017: "KeyI", 0x0018: "KeyO", 0x0019: "KeyP", 0x001a: "BracketLeft", 0x001b: "BracketRight", 0x001c: "Enter", 0x001d: "ControlLeft",
	0x001e: "KeyA", 0x001f: "KeyS", 0x0020: "KeyD", 0x0021: "KeyF", 0x0022: "KeyG", 0x0023: "KeyH", 0x0024: "KeyJ", 0x0025: "KeyK", 0x0026: "KeyL", 0x0027: "Semicolon", 0x0028: "Quote", 0x0029: "Backquote", 0x002a: "ShiftLeft", 0x002b: "Backslash", 0x002c: "KeyZ", 0x002d: "KeyX", 0x002e: "KeyC", 0x002f: "KeyV", 0x0030: "KeyB", 0x0031: "KeyN", 0x0032: "KeyM", 0x0033: "Comma", 0x0034: "Period", 0x0035: "Slash", 0x0036: "ShiftRight", 0x0037: "NumpadMultiply", 0x0038: "AltLeft", 0x0039: "Space", 0x003a: "CapsLock",
	0x003b: "F1", 0x003c: "F2", 0x003d: "F3", 0x003e: "F4", 0x003f: "F5", 0x0040: "F6", 0x0041: "F7", 0x0042: "F8", 0x0043: "F9", 0x0044: "F10", 0x0045: "NumLock", 0x0046: "ScrollLock", 0x0047: "Numpad7", 0x0048: "Numpad8", 0x0049: "Numpad9", 0x004a: "NumpadSubtract", 0x004b: "Numpad4", 0x004c: "Numpad5", 0x004d: "Numpad6", 0x004e: "NumpadAdd", 0x004f: "Numpad1", 0x0050: "Numpad2", 0x0051: "Numpad3", 0x0052: "Numpad0", 0x0053: "NumpadDecimal", 0x0056: "IntlBackslash", 0x0057: "F11", 0x0058: "F12",
	0x0070: "KanaMode", 0x0073: "IntlRo", 0x0079: "Convert", 0x007b: "NonConvert", 0x007d: "IntlYen",
	0x0e1c: "NumpadEnter", 0x0e1d: "ControlRight", 0x0e35: "NumpadDivide", 0x0e37: "PrintScreen", 0x0e38: "AltRight", 0x0e47: "Home", 0x0e48: "ArrowUp", 0x0e49: "PageUp", 0x0e4b: "ArrowLeft", 0x0e4d: "ArrowRight", 0x0e4f: "End", 0x0e50: "ArrowDown", 0x0e51: "PageDown", 0x0e52: "Insert", 0x0e53: "Delete", 0x0e5b: "MetaLeft", 0x0e5c: "MetaRight", 0x0e5d: "ContextMenu",
}

func keyboardEvent(keycode, mask uint16) inputEvent {
	return inputEvent{
		Code:  inputCodes[keycode],
		Ctrl:  mask&maskCtrl != 0,
		Shift: mask&maskShift != 0,
		Alt:   mask&maskAlt != 0,
		Meta:  mask&maskMeta != 0,
	}
}

type midiLatch struct {
	mu     sync.Mutex
	active map[midiKey]bool
}

type midiKey struct {
	source int
	midiBinding
}

func (l *midiLatch) press(source int, status, data1, value uint8) (midiBinding, bool) {
	kind := status & 0xf0
	if kind == 0x80 {
		status = status&0x0f | 0x90
		kind = 0x90
		value = 0
	}
	if kind != 0x90 && kind != 0xb0 {
		return midiBinding{}, false
	}
	binding := midiBinding{Status: status, Data1: data1}
	key := midiKey{source, binding}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active == nil {
		l.active = make(map[midiKey]bool)
	}
	if value == 0 {
		delete(l.active, key)
		return midiBinding{}, false
	}
	if l.active[key] {
		return midiBinding{}, false
	}
	l.active[key] = true
	return binding, true
}
