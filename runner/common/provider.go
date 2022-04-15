package common

import (
	"context"

	"github.com/google/go-github/v43/github"
)

type Provider interface {
	// CreateInstance creates a new compute instance in the provider.
	CreateInstance(ctx context.Context, runnerType string, tools github.RunnerApplicationDownload) error
	// Delete instance will delete the instance in a provider.
	DeleteInstance(ctx context.Context, instance string) error
	// ListInstances will list all instances for a provider.
	ListInstances(ctx context.Context) error
	// RemoveAllInstances will remove all instances created by this provider.
	RemoveAllInstances(ctx context.Context) error
	// Status returns the status of one instance.
	Status(ctx context.Context, instance string) error
	// Stop shuts down the instance.
	Stop(ctx context.Context, instance string) error
	// Start boots up an instance.
	Start(ctx context.Context, instance string) error
}
