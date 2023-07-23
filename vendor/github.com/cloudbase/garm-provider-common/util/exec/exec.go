package exec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/pkg/errors"
)

func Exec(ctx context.Context, providerBin string, stdinData []byte, environ []string) ([]byte, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c := exec.CommandContext(ctx, providerBin)
	c.Env = environ
	c.Stdin = bytes.NewBuffer(stdinData)
	c.Stdout = stdout
	c.Stderr = stderr

	if err := c.Run(); err != nil {
		return nil, errors.Wrapf(err, "provider binary failed with stdout: %s; stderr: %s", stdout.String(), stderr.String())
	}

	return stdout.Bytes(), nil
}
