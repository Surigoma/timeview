//go:build !cgo || (!windows && !darwin && !linux)

package main

import "errors"

func startPlatformInputs(chan<- inputEvent) (func(), int, error) {
	return nil, 0, errors.New("listenにはCGO対応版のtimeview-remoteが必要です")
}
