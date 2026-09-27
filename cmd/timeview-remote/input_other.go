//go:build !windows

package main

import "errors"

func startPlatformInputs(chan<- inputEvent) (func(), int, error) {
	return nil, 0, errors.New("listenによるキーパッド・MIDI入力はWindows版だけに対応しています")
}
