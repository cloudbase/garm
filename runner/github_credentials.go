package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ListCredentials(ctx context.Context) ([]params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	creds, err := r.store.ListGithubCredentials(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching github credentials")
	}

	return creds, nil
}

func (r *Runner) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "failed to validate github credentials params")
	}

	creds, err := r.store.CreateGithubCredentials(ctx, param)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "failed to create github credentials")
	}

	return creds, nil
}

func (r *Runner) GetGithubCredentials(ctx context.Context, id uint) (params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubCredentials{}, runnerErrors.ErrUnauthorized
	}

	creds, err := r.store.GetGithubCredentials(ctx, id, true)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "failed to get github credentials")
	}

	return creds, nil
}

func (r *Runner) DeleteGithubCredentials(ctx context.Context, id uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteGithubCredentials(ctx, id); err != nil {
		return errors.Wrap(err, "failed to delete github credentials")
	}

	return nil
}

func (r *Runner) UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "failed to validate github credentials params")
	}

	newCreds, err := r.store.UpdateGithubCredentials(ctx, id, param)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "failed to update github credentials")
	}

	return newCreds, nil
}
