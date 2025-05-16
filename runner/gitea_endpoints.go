package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateGiteaEndpoint(ctx context.Context, param params.CreateGiteaEndpointParams) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to validate gitea endpoint params")
	}

	ep, err := r.store.CreateGiteaEndpoint(ctx, param)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to create gitea endpoint")
	}

	return ep, nil
}

func (r *Runner) GetGiteaEndpoint(ctx context.Context, name string) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}
	endpoint, err := r.store.GetGiteaEndpoint(ctx, name)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to get gitea endpoint")
	}

	return endpoint, nil
}

func (r *Runner) DeleteGiteaEndpoint(ctx context.Context, name string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	err := r.store.DeleteGiteaEndpoint(ctx, name)
	if err != nil {
		return errors.Wrap(err, "failed to delete gitea endpoint")
	}

	return nil
}

func (r *Runner) UpdateGiteaEndpoint(ctx context.Context, name string, param params.UpdateGiteaEndpointParams) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to validate gitea endpoint params")
	}

	newEp, err := r.store.UpdateGiteaEndpoint(ctx, name, param)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to update gitea endpoint")
	}
	return newEp, nil
}

func (r *Runner) ListGiteaEndpoints(ctx context.Context) ([]params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	endpoints, err := r.store.ListGiteaEndpoints(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list gitea endpoints")
	}

	return endpoints, nil
}
