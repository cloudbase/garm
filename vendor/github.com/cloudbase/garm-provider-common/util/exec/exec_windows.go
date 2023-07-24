package exec

import (
	"os"
	"strings"
)

func IsExecutable(path string) bool {
	pathExt := os.Getenv("PATHEXT")
	execList := strings.Split(pathExt, ";")
	for _, ext := range execList {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}
