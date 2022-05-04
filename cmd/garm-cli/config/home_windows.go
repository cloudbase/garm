//go:build windows && !linux
// +build windows,!linux

package config

import (
	"os"
)

func getHomeDir() (string, error) {
	appData := os.Getenv("APPDATA")

	if appData == "" {
		return "", fmt.Errorf("failed to get home folder")
	}

	return filepath.Join(appData, DefaultAppFolder), nil
}
