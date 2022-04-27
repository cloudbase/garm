package runner

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"runner-manager/config"
	"runner-manager/database"
	dbCommon "runner-manager/database/common"
	gErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	"runner-manager/runner/providers"
	"runner-manager/util"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewRunner(ctx context.Context, cfg config.Config) (*Runner, error) {
	// ghc, err := util.GithubClient(ctx, cfg.Github.OAuth2Token)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "getting github client")
	// }

	providers, err := providers.LoadProvidersFromConfig(ctx, cfg, "")
	if err != nil {
		return nil, errors.Wrap(err, "loading providers")
	}
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	runner := &Runner{
		ctx:    ctx,
		config: cfg,
		db:     db,
		// ghc:       ghc,
		providers: providers,
	}

	if err := runner.ensureSSHKeys(); err != nil {
		return nil, errors.Wrap(err, "ensuring SSH keys")
	}

	return runner, nil
}

type Runner struct {
	mux sync.Mutex

	ctx context.Context
	// ghc *github.Client
	db dbCommon.Store

	controllerID string

	config        config.Config
	repositories  map[string]common.PoolManager
	organizations map[string]common.PoolManager
	providers     map[string]common.Provider
}

func (r *Runner) loadPools() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	// repos, err := r.db.ListRepositories(r.ctx)
	// if err != nil {
	// 	return errors.Wrap(err, "fetching repositories")
	// }

	return nil
}

func (r *Runner) findRepoPool(owner, name string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	// key := fmt.Sprintf("%s/%s", owner, name)
	// if repo, ok := r.repositories[key]; ok {
	// 	return pool, nil
	// }

	// repo, err := r.db.GetRepository(r.ctx, owner, name)
	// r.repositories[key] = repo
	return nil, errors.Wrapf(gErrors.ErrNotFound, "repository %s not configured", name)
}

func (r *Runner) findOrgPool(name string) (common.PoolManager, error) {
	if pool, ok := r.organizations[name]; ok {
		return pool, nil
	}
	return nil, errors.Wrapf(gErrors.ErrNotFound, "organization %s not configured", name)
}

func (r *Runner) validateHookBody(signature, secret string, body []byte) error {
	if secret == "" {
		// A secret was not set. Skip validation of body.
		return nil
	}

	if signature == "" {
		// A secret was set in our config, but a signature was not received
		// from Github. Authentication of the body cannot be done.
		return gErrors.NewUnauthorizedError("missing github signature")
	}

	sigParts := strings.SplitN(signature, "=", 2)
	if len(sigParts) != 2 {
		// We expect the signature from github to be of the format:
		// hashType=hashValue
		// ie: sha256=1fc917c7ad66487470e466c0ad40ddd45b9f7730a4b43e1b2542627f0596bbdc
		return gErrors.NewBadRequestError("invalid signature format")
	}

	var hashFunc func() hash.Hash
	switch sigParts[0] {
	case "sha256":
		hashFunc = sha256.New
	case "sha1":
		hashFunc = sha1.New
	default:
		return gErrors.NewBadRequestError("unknown signature type")
	}

	mac := hmac.New(hashFunc, []byte(secret))
	_, err := mac.Write(body)
	if err != nil {
		return errors.Wrap(err, "failed to compute sha256")
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigParts[1]), []byte(expectedMAC)) {
		return gErrors.NewUnauthorizedError("signature missmatch")
	}

	return nil
}

func (r *Runner) DispatchWorkflowJob(hookTargetType, signature string, jobData []byte) error {
	if jobData == nil || len(jobData) == 0 {
		return gErrors.NewBadRequestError("missing job data")
	}

	var job params.WorkflowJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		return errors.Wrapf(gErrors.ErrBadRequest, "invalid job data: %s", err)
	}

	var poolManager common.PoolManager
	var err error

	switch HookTargetType(hookTargetType) {
	case RepoHook:
		poolManager, err = r.findRepoPool(job.Repository.Owner.Login, job.Repository.Name)
	case OrganizationHook:
		poolManager, err = r.findOrgPool(job.Organization.Login)
	default:
		return gErrors.NewBadRequestError("cannot handle hook target type %s", hookTargetType)
	}

	if err != nil {
		// We don't have a repository or organization configured that
		// can handle this workflow job.
		return errors.Wrap(err, "fetching poolManager")
	}

	// We found a pool. Validate the webhook job. If a secret is configured,
	// we make sure that the source of this workflow job is valid.
	secret := poolManager.WebhookSecret()
	if err := r.validateHookBody(signature, secret, jobData); err != nil {
		return errors.Wrap(err, "validating webhook data")
	}

	return nil
}

func (r *Runner) sshDir() string {
	return filepath.Join(r.config.Default.ConfigDir, "ssh")
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
