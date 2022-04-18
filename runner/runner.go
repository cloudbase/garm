package runner

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runner-manager/config"
	"runner-manager/runner/common"
	"runner-manager/util"
	"sync"

	"github.com/google/go-github/v43/github"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewRunner(ctx context.Context, cfg *config.Config) (*Runner, error) {
	runner := &Runner{
		ctx:    ctx,
		config: cfg,
	}

	if err := runner.ensureSSHKeys(); err != nil {
		return nil, errors.Wrap(err, "ensuring SSH keys")
	}

	return runner, nil
}

type Runner struct {
	mux sync.Mutex

	ctx context.Context
	ghc *github.Client

	config *config.Config
	pools  []common.PoolManager
}

func (r *Runner) sshDir() string {
	return filepath.Join(r.config.ConfigDir, "ssh")
}

func (r *Runner) sshKeyPath() string {
	keyPath := filepath.Join(r.sshDir(), "runner_rsa_key")
	return keyPath
}

func (r *Runner) sshPubKeyPath() string {
	keyPath := filepath.Join(r.sshDir(), "runner_rsa_key.pub")
	return keyPath
}

func (r *Runner) parseSSHKey() (ssh.Signer, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	key, err := ioutil.ReadFile(r.sshKeyPath())
	if err != nil {
		return nil, errors.Wrapf(err, "reading private key %s", r.sshKeyPath())
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing private key %s", r.sshKeyPath())
	}

	return signer, nil
}

func (r *Runner) sshPubKey() ([]byte, error) {
	key, err := ioutil.ReadFile(r.sshPubKeyPath())
	if err != nil {
		return nil, errors.Wrapf(err, "reading public key %s", r.sshPubKeyPath())
	}
	return key, nil
}

func (r *Runner) ensureSSHKeys() error {
	sshDir := r.sshDir()

	if _, err := os.Stat(sshDir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrapf(err, "checking SSH dir %s", sshDir)
		}
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			return errors.Wrapf(err, "creating ssh dir %s", sshDir)
		}
	}

	privKeyFile := r.sshKeyPath()
	pubKeyFile := r.sshPubKeyPath()

	if _, err := os.Stat(privKeyFile); err == nil {
		return nil
	}

	pubKey, privKey, err := util.GenerateSSHKeyPair()
	if err != nil {
		errors.Wrap(err, "generating keypair")
	}

	if err := ioutil.WriteFile(privKeyFile, privKey, 0o600); err != nil {
		return errors.Wrap(err, "writing private key")
	}

	if err := ioutil.WriteFile(pubKeyFile, pubKey, 0o600); err != nil {
		return errors.Wrap(err, "writing public key")
	}

	return nil
}
