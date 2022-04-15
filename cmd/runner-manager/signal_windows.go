//go:build windows && ignore
// +build windows,ignore

package main

import "os"

var signals = []os.Signal{
	os.Interrupt,
}
