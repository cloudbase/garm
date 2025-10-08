package util

import (
	"fmt"
	"os"
)

func getTempDir(baseDir string) (string, error) {
	dir := baseDir
	if baseDir == "" {
		envTmp := os.Getenv("TEMP")
		if envTmp == "" {
			envTmp = os.Getenv("TMP")
		}
		dir = envTmp
	}

	if dir == "" {
		return "", fmt.Errorf("failed to determine destination dir")
	}
	return dir, nil
}
