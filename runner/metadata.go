package runner

import (
	"context"
	"log"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/pkg/errors"
)

func (r *Runner) GetRootCertificateBundle(ctx context.Context) (params.CertificateBundle, error) {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		log.Printf("failed to get instance params: %s", err)
		return params.CertificateBundle{}, runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return params.CertificateBundle{}, errors.Wrap(err, "fetching pool manager for instance")
	}

	bundle, err := poolMgr.RootCABundle()
	if err != nil {
		log.Printf("failed to get root CA bundle: %s", err)
		// The root CA bundle is invalid. Return an empty bundle to the runner and log the event.
		return params.CertificateBundle{
			RootCertificates: make(map[string][]byte),
		}, nil
	}
	return bundle, nil
}
