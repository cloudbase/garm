//go:build !windows
// +build !windows

package util

import (
	"os"
)

func getTempDir(baseDir string) (string, error) {
	dir := baseDir
	if baseDir == "" {
		envTmp := os.Getenv("TMPDIR")
		if envTmp == "" {
			envTmp = "/tmp"
		}
		dir = envTmp
	}

	return dir, nil
}
