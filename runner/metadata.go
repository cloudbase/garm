package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

var systemdUnitTemplate = `[Unit]
Description=GitHub Actions Runner ({{.ServiceName}})
After=network.target

[Service]
ExecStart=/home/{{.RunAsUser}}/actions-runner/runsvc.sh
User={{.RunAsUser}}
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
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}
	return instance, nil
}

func (r *Runner) GetRunnerServiceName(ctx context.Context) (string, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return "", runnerErrors.ErrUnauthorized
	}
	var entity params.GithubEntity

	switch {
	case instance.PoolID != "":
		pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to get pool",
				"pool_id", instance.PoolID)
			return "", errors.Wrap(err, "fetching pool")
		}
		entity, err = pool.GetEntity()
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to get pool entity",
				"pool_id", instance.PoolID)
			return "", errors.Wrap(err, "fetching pool entity")
		}
	case instance.ScaleSetID != 0:
		scaleSet, err := r.store.GetScaleSetByID(r.ctx, instance.ScaleSetID)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to get scale set",
				"scale_set_id", instance.ScaleSetID)
			return "", errors.Wrap(err, "fetching scale set")
		}
		entity, err = scaleSet.GetEntity()
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to get scale set entity",
				"scale_set_id", instance.ScaleSetID)
			return "", errors.Wrap(err, "fetching scale set entity")
		}
	default:
		return "", errors.New("instance not associated with a pool or scale set")
	}

	tpl := "actions.runner.%s.%s"
	var serviceName string
	switch entity.EntityType {
	case params.GithubEntityTypeEnterprise:
		serviceName = fmt.Sprintf(tpl, entity.Owner, instance.Name)
	case params.GithubEntityTypeOrganization:
		serviceName = fmt.Sprintf(tpl, entity.Owner, instance.Name)
	case params.GithubEntityTypeRepository:
		serviceName = fmt.Sprintf(tpl, fmt.Sprintf("%s-%s", entity.Owner, entity.Name), instance.Name)
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
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
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
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
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

	if _, err := r.store.UpdateInstance(r.ctx, instance.Name, updateParams); err != nil {
		return "", errors.Wrap(err, "setting token_fetched for instance")
	}

	if err := r.store.AddInstanceEvent(ctx, instance.Name, params.FetchTokenEvent, params.EventInfo, "runner registration token was retrieved"); err != nil {
		return "", errors.Wrap(err, "recording event")
	}

	return token, nil
}

func (r *Runner) GetRootCertificateBundle(ctx context.Context) (params.CertificateBundle, error) {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return params.CertificateBundle{}, runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return params.CertificateBundle{}, errors.Wrap(err, "fetching pool manager for instance")
	}

	bundle, err := poolMgr.RootCABundle()
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get root CA bundle",
			"instance", instance.Name,
			"pool_manager", poolMgr.ID())
		// The root CA bundle is invalid. Return an empty bundle to the runner and log the event.
		return params.CertificateBundle{
			RootCertificates: make(map[string][]byte),
		}, nil
	}
	return bundle, nil
}
