//go:build windows

package main

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	whKeyboardLL  = 13
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmQuit        = 0x0012
	pmNoRemove    = 0
	llkhfExtended = 0x01

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkLWin    = 0x5b
	vkRWin    = 0x5c

	callbackFunction = 0x00030000
	mimData          = 0x03c3
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	setWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx      = user32.NewProc("CallNextHookEx")
	getMessage          = user32.NewProc("GetMessageW")
	peekMessage         = user32.NewProc("PeekMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postThreadMessage   = user32.NewProc("PostThreadMessageW")
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getCurrentThreadID  = kernel32.NewProc("GetCurrentThreadId")
	rtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")
	winmm               = syscall.NewLazyDLL("winmm.dll")
	midiInGetNumDevs    = winmm.NewProc("midiInGetNumDevs")
	midiInOpen          = winmm.NewProc("midiInOpen")
	midiInStart         = winmm.NewProc("midiInStart")
	midiInStop          = winmm.NewProc("midiInStop")
	midiInReset         = winmm.NewProc("midiInReset")
	midiInClose         = winmm.NewProc("midiInClose")
	windowsScanCodes    = map[uint32]string{
		0x01: "Escape", 0x02: "Digit1", 0x03: "Digit2", 0x04: "Digit3", 0x05: "Digit4", 0x06: "Digit5", 0x07: "Digit6", 0x08: "Digit7", 0x09: "Digit8", 0x0a: "Digit9", 0x0b: "Digit0",
		0x0c: "Minus", 0x0d: "Equal", 0x0e: "Backspace", 0x0f: "Tab", 0x10: "KeyQ", 0x11: "KeyW", 0x12: "KeyE", 0x13: "KeyR", 0x14: "KeyT", 0x15: "KeyY", 0x16: "KeyU", 0x17: "KeyI", 0x18: "KeyO", 0x19: "KeyP", 0x1a: "BracketLeft", 0x1b: "BracketRight", 0x1c: "Enter", 0x1d: "ControlLeft",
		0x1e: "KeyA", 0x1f: "KeyS", 0x20: "KeyD", 0x21: "KeyF", 0x22: "KeyG", 0x23: "KeyH", 0x24: "KeyJ", 0x25: "KeyK", 0x26: "KeyL", 0x27: "Semicolon", 0x28: "Quote", 0x29: "Backquote", 0x2a: "ShiftLeft", 0x2b: "Backslash", 0x2c: "KeyZ", 0x2d: "KeyX", 0x2e: "KeyC", 0x2f: "KeyV", 0x30: "KeyB", 0x31: "KeyN", 0x32: "KeyM", 0x33: "Comma", 0x34: "Period", 0x35: "Slash", 0x36: "ShiftRight", 0x37: "NumpadMultiply", 0x38: "AltLeft", 0x39: "Space", 0x3a: "CapsLock",
		0x3b: "F1", 0x3c: "F2", 0x3d: "F3", 0x3e: "F4", 0x3f: "F5", 0x40: "F6", 0x41: "F7", 0x42: "F8", 0x43: "F9", 0x44: "F10", 0x45: "NumLock", 0x46: "ScrollLock", 0x47: "Numpad7", 0x48: "Numpad8", 0x49: "Numpad9", 0x4a: "NumpadSubtract", 0x4b: "Numpad4", 0x4c: "Numpad5", 0x4d: "Numpad6", 0x4e: "NumpadAdd", 0x4f: "Numpad1", 0x50: "Numpad2", 0x51: "Numpad3", 0x52: "Numpad0", 0x53: "NumpadDecimal", 0x56: "IntlBackslash", 0x57: "F11", 0x58: "F12",
		0x70: "KanaMode", 0x73: "IntlRo", 0x79: "Convert", 0x7b: "NonConvert", 0x7d: "IntlYen",
	}
)

type keyboardData struct {
	VKCode, ScanCode, Flags, Time uint32
	ExtraInfo                     uintptr
}

type windowsMessage struct {
	Window, Message, WParam, LParam uintptr
	Time                            uint32
	PointX, PointY                  int32
	Private                         uint32
}

type windowsInputs struct {
	events       chan<- inputEvent
	threadID     uintptr
	stopOnce     sync.Once
	midiCallback uintptr
	midiHandles  []uintptr
	midiMu       sync.Mutex
	midiActive   map[struct {
		handle uintptr
		midiBinding
	}]bool
}

func startPlatformInputs(events chan<- inputEvent) (func(), int, error) {
	inputs := &windowsInputs{
		events: events,
		midiActive: make(map[struct {
			handle uintptr
			midiBinding
		}]bool),
	}
	ready := make(chan error, 1)
	go inputs.keyboardLoop(ready)
	if err := <-ready; err != nil {
		return nil, 0, err
	}
	inputs.openMIDI()
	return inputs.stop, len(inputs.midiHandles), nil
}

func (i *windowsInputs) keyboardLoop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	down := make(map[uint64]bool)
	callback := syscall.NewCallback(func(code, message, data uintptr) uintptr {
		if int32(code) >= 0 && data != 0 {
			var keyboard keyboardData
			rtlMoveMemory.Call(uintptr(unsafe.Pointer(&keyboard)), data, unsafe.Sizeof(keyboard))
			id := uint64(keyboard.ScanCode)<<1 | uint64(keyboard.Flags&llkhfExtended)
			switch message {
			case wmKeyDown, wmSysKeyDown:
				if !down[id] {
					down[id] = true
					if key := keyCodeFor(keyboard.ScanCode, keyboard.Flags&llkhfExtended != 0); key != "" {
						i.emit(inputEvent{Code: key, Ctrl: keyPressed(vkControl), Shift: keyPressed(vkShift), Alt: keyPressed(vkMenu), Meta: keyPressed(vkLWin) || keyPressed(vkRWin)})
					}
				}
			case wmKeyUp, wmSysKeyUp:
				delete(down, id)
			}
		}
		result, _, _ := callNextHookEx.Call(0, code, message, data)
		return result
	})
	hook, _, _ := setWindowsHookEx.Call(whKeyboardLL, callback, 0, 0)
	if hook == 0 {
		ready <- errors.New("キーボード入力の監視を開始できません")
		return
	}
	defer unhookWindowsHookEx.Call(hook)
	i.threadID, _, _ = getCurrentThreadID.Call()
	var message windowsMessage
	peekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmNoRemove)
	ready <- nil
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			return
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&message)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (i *windowsInputs) openMIDI() {
	i.midiCallback = syscall.NewCallback(func(handle, message, _, packed, _ uintptr) uintptr {
		if message == mimData {
			i.handleMIDI(handle, uint32(packed))
		}
		return 0
	})
	count, _, _ := midiInGetNumDevs.Call()
	for device := uintptr(0); device < count; device++ {
		var handle uintptr
		result, _, _ := midiInOpen.Call(uintptr(unsafe.Pointer(&handle)), device, i.midiCallback, 0, callbackFunction)
		if result != 0 {
			continue
		}
		result, _, _ = midiInStart.Call(handle)
		if result != 0 {
			midiInClose.Call(handle)
			continue
		}
		i.midiHandles = append(i.midiHandles, handle)
	}
}

func (i *windowsInputs) handleMIDI(handle uintptr, packed uint32) {
	status, data1, value := uint8(packed), uint8(packed>>8), uint8(packed>>16)
	kind := status & 0xf0
	if kind == 0x80 {
		status = status&0x0f | 0x90
		kind = 0x90
		value = 0
	}
	if kind != 0x90 && kind != 0xb0 {
		return
	}
	binding := midiBinding{Status: status, Data1: data1}
	key := struct {
		handle uintptr
		midiBinding
	}{handle, binding}
	i.midiMu.Lock()
	if value == 0 {
		delete(i.midiActive, key)
		i.midiMu.Unlock()
		return
	}
	if i.midiActive[key] {
		i.midiMu.Unlock()
		return
	}
	i.midiActive[key] = true
	i.midiMu.Unlock()
	i.emit(inputEvent{MIDI: &binding})
}

func (i *windowsInputs) emit(event inputEvent) {
	select {
	case i.events <- event:
	default:
	}
}

func (i *windowsInputs) stop() {
	i.stopOnce.Do(func() {
		for _, handle := range i.midiHandles {
			midiInStop.Call(handle)
			midiInReset.Call(handle)
			midiInClose.Call(handle)
		}
		postThreadMessage.Call(i.threadID, wmQuit, 0, 0)
	})
}

func keyPressed(virtualKey uintptr) bool {
	state, _, _ := getAsyncKeyState.Call(virtualKey)
	return uint16(state)&0x8000 != 0
}

func keyCodeFor(scan uint32, extended bool) string {
	if extended {
		switch scan {
		case 0x1c:
			return "NumpadEnter"
		case 0x1d:
			return "ControlRight"
		case 0x35:
			return "NumpadDivide"
		case 0x37:
			return "PrintScreen"
		case 0x38:
			return "AltRight"
		case 0x47:
			return "Home"
		case 0x48:
			return "ArrowUp"
		case 0x49:
			return "PageUp"
		case 0x4b:
			return "ArrowLeft"
		case 0x4d:
			return "ArrowRight"
		case 0x4f:
			return "End"
		case 0x50:
			return "ArrowDown"
		case 0x51:
			return "PageDown"
		case 0x52:
			return "Insert"
		case 0x53:
			return "Delete"
		case 0x5b:
			return "MetaLeft"
		case 0x5c:
			return "MetaRight"
		case 0x5d:
			return "ContextMenu"
		}
	}
	return windowsScanCodes[scan]
}
