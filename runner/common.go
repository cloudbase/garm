package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ResolveForgeCredentialByName(ctx context.Context, credentialsName string) (params.ForgeCredentials, error) {
	githubCred, err := r.store.GetGithubCredentialsByName(ctx, credentialsName, false)
	if err != nil && !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching github credentials")
	}
	giteaCred, err := r.store.GetGiteaCredentialsByName(ctx, credentialsName, false)
	if err != nil && !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching gitea credentials")
	}
	if githubCred.ID != 0 && giteaCred.ID != 0 {
		return params.ForgeCredentials{}, runnerErrors.NewBadRequestError("credentials %s are defined for both GitHub and Gitea, please specify the forge type", credentialsName)
	}
	if githubCred.ID != 0 {
		return githubCred, nil
	}
	if giteaCred.ID != 0 {
		return giteaCred, nil
	}
	return params.ForgeCredentials{}, runnerErrors.NewBadRequestError("credentials %s not found", credentialsName)
}
