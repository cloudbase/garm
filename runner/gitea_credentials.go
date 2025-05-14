package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ListGiteaCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	// Get the credentials from the store. The cache is always updated after the database successfully
	// commits the transaction that created/updated the credentials.
	// If we create a set of credentials then immediately after we call ListGiteaCredentials,
	// there is a posibillity that not all creds will be in the cache.
	creds, err := r.store.ListGiteaCredentials(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching gitea credentials")
	}
	return creds, nil
}

func (r *Runner) CreateGiteaCredentials(ctx context.Context, param params.CreateGiteaCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to validate gitea credentials params")
	}

	creds, err := r.store.CreateGiteaCredentials(ctx, param)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to create gitea credentials")
	}

	return creds, nil
}

func (r *Runner) GetGiteaCredentials(ctx context.Context, id uint) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	creds, err := r.store.GetGiteaCredentials(ctx, id, true)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to get gitea credentials")
	}

	return creds, nil
}

func (r *Runner) DeleteGiteaCredentials(ctx context.Context, id uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteGiteaCredentials(ctx, id); err != nil {
		return errors.Wrap(err, "failed to delete gitea credentials")
	}

	return nil
}

func (r *Runner) UpdateGiteaCredentials(ctx context.Context, id uint, param params.UpdateGiteaCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to validate gitea credentials params")
	}

	newCreds, err := r.store.UpdateGiteaCredentials(ctx, id, param)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to update gitea credentials")
	}

	return newCreds, nil
}
