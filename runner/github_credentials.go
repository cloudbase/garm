package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ListCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	// Get the credentials from the store. The cache is always updated after the database successfully
	// commits the transaction that created/updated the credentials.
	// If we create a set of credentials then immediately after we call ListCredentials,
	// there is a posibillity that not all creds will be in the cache.
	creds, err := r.store.ListGithubCredentials(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching github credentials")
	}

	// If we do have cache, update the rate limit for each credential. The rate limits are queried
	// every 30 seconds and set in cache.
	credsCache := cache.GetAllGithubCredentialsAsMap()
	for idx, cred := range creds {
		inCache, ok := credsCache[cred.ID]
		if ok {
			creds[idx].RateLimit = inCache.RateLimit
		}
	}
	return creds, nil
}

func (r *Runner) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to validate github credentials params")
	}

	creds, err := r.store.CreateGithubCredentials(ctx, param)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to create github credentials")
	}

	return creds, nil
}

func (r *Runner) GetGithubCredentials(ctx context.Context, id uint) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	creds, err := r.store.GetGithubCredentials(ctx, id, true)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to get github credentials")
	}

	cached, ok := cache.GetGithubCredentials((creds.ID))
	if ok {
		creds.RateLimit = cached.RateLimit
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

func (r *Runner) UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to validate github credentials params")
	}

	newCreds, err := r.store.UpdateGithubCredentials(ctx, id, param)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "failed to update github credentials")
	}

	return newCreds, nil
}
