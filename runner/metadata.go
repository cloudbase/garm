package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/pkg/errors"
)

var systemdUnitTemplate = `[Unit]
Description=GitHub Actions Runner ({{.ServiceName}})
After=network.target

[Service]
ExecStart=/home/{{.RunAsUser}}/actions-runner/runsvc.sh
User=runner
WorkingDirectory=/home/{{.RunAsUser}}/actions-runner
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=5min

[Install]
WantedBy=multi-user.target
`

func validateInstanceState(ctx context.Context) (params.Instance, error) {
	if !auth.InstanceHasJITConfig(ctx) {
		return params.Instance{}, fmt.Errorf("instance not configured for JIT: %w", runnerErrors.ErrNotFound)
	}

	status := auth.InstanceRunnerStatus(ctx)
	if status != params.RunnerPending && status != params.RunnerInstalling {
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}

	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}
	return instance, nil
}

func (r *Runner) GetRunnerServiceName(ctx context.Context) (string, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return "", runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
	if err != nil {
		log.Printf("failed to get pool: %s", err)
		return "", errors.Wrap(err, "fetching pool")
	}

	tpl := "actions.runner.%s.%s"
	var serviceName string
	switch pool.PoolType() {
	case params.EnterprisePool:
		serviceName = fmt.Sprintf(tpl, pool.EnterpriseName, instance.Name)
	case params.OrganizationPool:
		serviceName = fmt.Sprintf(tpl, pool.OrgName, instance.Name)
	case params.RepositoryPool:
		serviceName = fmt.Sprintf(tpl, strings.Replace(pool.RepoName, "/", "-", -1), instance.Name)
	}
	return serviceName, nil
}

func (r *Runner) GenerateSystemdUnitFile(ctx context.Context, runAsUser string) ([]byte, error) {
	serviceName, err := r.GetRunnerServiceName(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching runner service name")
	}

	unitTemplate, err := template.New("").Parse(systemdUnitTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "parsing template")
	}

	if runAsUser == "" {
		runAsUser = defaults.DefaultUser
	}

	data := struct {
		ServiceName string
		RunAsUser   string
	}{
		ServiceName: serviceName,
		RunAsUser:   runAsUser,
	}

	var unitFile bytes.Buffer
	if err := unitTemplate.Execute(&unitFile, data); err != nil {
		return nil, errors.Wrap(err, "executing template")
	}
	return unitFile.Bytes(), nil
}

func (r *Runner) GetJITConfigFile(ctx context.Context, file string) ([]byte, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return nil, runnerErrors.ErrUnauthorized
	}
	jitConfig := instance.JitConfiguration
	contents, ok := jitConfig[file]
	if !ok {
		return nil, errors.Wrap(runnerErrors.ErrNotFound, "retrieving file")
	}

	decoded, err := base64.StdEncoding.DecodeString(contents)
	if err != nil {
		return nil, errors.Wrap(err, "decoding file contents")
	}

	return decoded, nil
}

func (r *Runner) GetInstanceGithubRegistrationToken(ctx context.Context) (string, error) {
	// Check if this instance already fetched a registration token or if it was configured using
	// the new Just In Time runner feature. If we're still using the old way of configuring a runner,
	// we only allow an instance to fetch one token. If the instance fails to bootstrap after a token
	// is fetched, we reset the token fetched field when re-queueing the instance.
	if auth.InstanceTokenFetched(ctx) || auth.InstanceHasJITConfig(ctx) {
		return "", runnerErrors.ErrUnauthorized
	}

	status := auth.InstanceRunnerStatus(ctx)
	if status != params.RunnerPending && status != params.RunnerInstalling {
		return "", runnerErrors.ErrUnauthorized
	}

	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return "", runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return "", errors.Wrap(err, "fetching pool manager for instance")
	}

	token, err := poolMgr.GithubRunnerRegistrationToken()
	if err != nil {
		return "", errors.Wrap(err, "fetching runner token")
	}

	tokenFetched := true
	updateParams := params.UpdateInstanceParams{
		TokenFetched: &tokenFetched,
	}

	if _, err := r.store.UpdateInstance(r.ctx, instance.ID, updateParams); err != nil {
		return "", errors.Wrap(err, "setting token_fetched for instance")
	}

	if err := r.store.AddInstanceEvent(ctx, instance.ID, params.FetchTokenEvent, params.EventInfo, "runner registration token was retrieved"); err != nil {
		return "", errors.Wrap(err, "recording event")
	}

	return token, nil
}

func (r *Runner) GetRootCertificateBundle(ctx context.Context) (params.CertificateBundle, error) {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return params.CertificateBundle{}, runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return params.CertificateBundle{}, errors.Wrap(err, "fetching pool manager for instance")
	}

	bundle, err := poolMgr.RootCABundle()
	if err != nil {
		log.Printf("failed to get root CA bundle: %s", err)
		// The root CA bundle is invalid. Return an empty bundle to the runner and log the event.
		return params.CertificateBundle{
			RootCertificates: make(map[string][]byte),
		}, nil
	}
	return bundle, nil
}
