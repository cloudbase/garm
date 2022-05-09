//go:build !windows
// +build !windows

package exec

import (
	"golang.org/x/sys/unix"
)

func IsExecutable(path string) bool {
	if unix.Access(path, unix.X_OK) == nil {
		return true
	}

	return false
}
