package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateGithubEndpoint(ctx context.Context, param params.CreateGithubEndpointParams) (params.GithubEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "failed to validate github endpoint params")
	}

	ep, err := r.store.CreateGithubEndpoint(ctx, param)
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "failed to create github endpoint")
	}

	return ep, nil
}

func (r *Runner) GetGithubEndpoint(ctx context.Context, name string) (params.GithubEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubEndpoint{}, runnerErrors.ErrUnauthorized
	}
	endpoint, err := r.store.GetGithubEndpoint(ctx, name)
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "failed to get github endpoint")
	}

	return endpoint, nil
}

func (r *Runner) DeleteGithubEndpoint(ctx context.Context, name string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	err := r.store.DeleteGithubEndpoint(ctx, name)
	if err != nil {
		return errors.Wrap(err, "failed to delete github endpoint")
	}

	return nil
}

func (r *Runner) UpdateGithubEndpoint(ctx context.Context, name string, param params.UpdateGithubEndpointParams) (params.GithubEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.GithubEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "failed to validate github endpoint params")
	}

	newEp, err := r.store.UpdateGithubEndpoint(ctx, name, param)
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "failed to update github endpoint")
	}
	return newEp, nil
}

func (r *Runner) ListGithubEndpoints(ctx context.Context) ([]params.GithubEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	endpoints, err := r.store.ListGithubEndpoints(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list github endpoints")
	}

	return endpoints, nil
}
