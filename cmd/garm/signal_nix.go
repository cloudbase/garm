//go:build !windows
// +build !windows

package main

import (
	"os"
	"syscall"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}
