//go:build windows && !linux
// +build windows,!linux

package main

import "os"

var signals = []os.Signal{
	os.Interrupt,
}
