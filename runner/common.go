package runner

import (
	"context"
	"strings"

	"github.com/google/uuid"
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

func (r *Runner) ResolveRepositoryID(ctx context.Context, repoID string) (string, error) {
	if _, err := uuid.Parse(repoID); err == nil {
		return repoID, nil
	}
	if repoID == "" {
		return "", runnerErrors.NewBadRequestError("repository ID cannot be empty")
	}
	comp := strings.SplitN(repoID, "/", 2)
	repo, err := r.store.GetRepository(ctx, comp[0], comp[1], "")
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return "", runnerErrors.NewBadRequestError("repository %s not found", repoID)
		}
		return "", errors.Wrapf(err, "fetching repository %s", repoID)
	}
	return repo.ID, nil
}
