package config

import (
	"os"

	"github.com/pkg/errors"
)

func ensureHomeDir(folder string) error {
	if _, err := os.Stat(folder); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "checking home dir")
		}

		if err := os.MkdirAll(folder, 0o710); err != nil {
			return errors.Wrapf(err, "creating %s", folder)
		}
	}

	return nil
}
