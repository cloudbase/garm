package execution

import (
	"errors"

	gErrors "github.com/cloudbase/garm/errors"
)

const (
	// ExitCodeNotFound is an exit code that indicates a Not Found error
	ExitCodeNotFound int = 30
	// ExitCodeDuplicate is an exit code that indicates a duplicate error
	ExitCodeDuplicate int = 31
)

func ResolveErrorToExitCode(err error) int {
	if err != nil {
		if errors.Is(err, gErrors.ErrNotFound) {
			return ExitCodeNotFound
		} else if errors.Is(err, gErrors.ErrDuplicateEntity) {
			return ExitCodeDuplicate
		}
		return 1
	}
	return 0
}
