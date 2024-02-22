//go:build !windows
// +build !windows

package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func getHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "fetching home dir")
	}

	return filepath.Join(home, ".local", "share", DefaultAppFolder), nil
}
