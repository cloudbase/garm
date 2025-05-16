package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v71/github"
	"github.com/pkg/errors"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
)

type createGiteaHookOptions struct {
	Type                string            `json:"type"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	BranchFilter        string            `json:"branch_filter"`
	Active              bool              `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
}

func (g *githubClient) createGiteaRepoHook(ctx context.Context, owner, name string, hook *github.Hook) (ret *github.Hook, err error) {
	u := fmt.Sprintf("repos/%v/%v/hooks", owner, name)
	createOpts := &createGiteaHookOptions{
		Type:         "gitea",
		Events:       hook.Events,
		Active:       hook.GetActive(),
		BranchFilter: "*",
		Config: map[string]string{
			"content_type": hook.GetConfig().GetContentType(),
			"url":          hook.GetConfig().GetURL(),
			"http_method":  "post",
			"secret":       hook.GetConfig().GetSecret(),
		},
	}

	req, err := g.cli.NewRequest(http.MethodPost, u, createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	hook = new(github.Hook)
	_, err = g.cli.Do(ctx, req, hook)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	return hook, nil
}

func (g *githubClient) createGiteaOrgHook(ctx context.Context, owner string, hook *github.Hook) (ret *github.Hook, err error) {
	u := fmt.Sprintf("orgs/%v/hooks", owner)
	createOpts := &createGiteaHookOptions{
		Type:         "gitea",
		Events:       hook.Events,
		Active:       hook.GetActive(),
		BranchFilter: "*",
		Config: map[string]string{
			"content_type": hook.GetConfig().GetContentType(),
			"url":          hook.GetConfig().GetURL(),
			"http_method":  "post",
			"secret":       hook.GetConfig().GetSecret(),
		},
	}

	req, err := g.cli.NewRequest(http.MethodPost, u, createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	hook = new(github.Hook)
	_, err = g.cli.Do(ctx, req, hook)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	return hook, nil
}

func (g *githubClient) createGiteaEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"CreateHook",          // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"CreateHook",          // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, err = g.createGiteaRepoHook(ctx, g.entity.Owner, g.entity.Name, hook)
	case params.ForgeEntityTypeOrganization:
		ret, err = g.createGiteaOrgHook(ctx, g.entity.Owner, hook)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}
