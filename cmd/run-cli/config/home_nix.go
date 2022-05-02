//go:build !windows
// +build !windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func getHomeDir() (string, error) {
	home := os.Getenv("HOME")

	if home == "" {
		return "", fmt.Errorf("failed to get home folder")
	}

	return filepath.Join(home, ".local", "share", DefaultAppFolder), nil
}
