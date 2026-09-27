//go:build windows

package main

import "testing"

func TestWindowsKeyCodes(t *testing.T) {
	for _, test := range []struct {
		scan     uint32
		extended bool
		want     string
	}{
		{0x1c, true, "NumpadEnter"},
		{0x1c, false, "Enter"},
		{0x35, true, "NumpadDivide"},
		{0x4f, false, "Numpad1"},
		{0x4f, true, "End"},
		{0x1f, false, "KeyS"},
	} {
		if got := keyCodeFor(test.scan, test.extended); got != test.want {
			t.Fatalf("keyCodeFor(%x, %t) = %q, want %q", test.scan, test.extended, got, test.want)
		}
	}
}

func TestWindowsMIDILatchesUntilRelease(t *testing.T) {
	events := make(chan inputEvent, 2)
	inputs := windowsInputs{
		events: events,
		midiActive: make(map[struct {
			handle uintptr
			midiBinding
		}]bool),
	}
	press := uint32(0x90 | 36<<8 | 127<<16)
	inputs.handleMIDI(1, press)
	inputs.handleMIDI(1, press)
	if len(events) != 1 {
		t.Fatalf("events after repeated press = %d", len(events))
	}
	inputs.handleMIDI(1, uint32(0x80|36<<8))
	inputs.handleMIDI(1, press)
	if len(events) != 2 {
		t.Fatalf("events after release and press = %d", len(events))
	}
}
