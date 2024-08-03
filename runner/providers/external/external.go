package external

import (
	"context"
	"fmt"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/runner/common"
	v010 "github.com/cloudbase/garm/runner/providers/v0.1.0"
	v011 "github.com/cloudbase/garm/runner/providers/v0.1.1"
)

// NewProvider selects the provider based on the interface version
func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	switch cfg.External.InterfaceVersion {
	case common.Version010, "":
		return v010.NewProvider(ctx, cfg, controllerID)
	case common.Version011:
		return v011.NewProvider(ctx, cfg, controllerID)
	default:
		return nil, fmt.Errorf("unsupported interface version: %s", cfg.External.InterfaceVersion)
	}
}
