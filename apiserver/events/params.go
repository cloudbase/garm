package events

import (
	"github.com/cloudbase/garm/database/common"
)

type Filter struct {
	Operations []common.OperationType    `json:"operations,omitempty" jsonschema:"title=operations,description=A list of operations to filter on,enum=create,enum=update,enum=delete"`
	EntityType common.DatabaseEntityType `json:"entity-type,omitempty" jsonschema:"title=entity type,description=The type of entity to filter on,enum=repository,enum=organization,enum=enterprise,enum=pool,enum=user,enum=instance,enum=job,enum=controller,enum=github_credentials,enum=github_endpoint"`
}

func (f Filter) Validate() error {
	switch f.EntityType {
	case common.RepositoryEntityType, common.OrganizationEntityType, common.EnterpriseEntityType,
		common.PoolEntityType, common.UserEntityType, common.InstanceEntityType,
		common.JobEntityType, common.ControllerEntityType, common.GithubCredentialsEntityType,
		common.GiteaCredentialsEntityType, common.ScaleSetEntityType, common.GithubEndpointEntityType:
	default:
		return common.ErrInvalidEntityType
	}

	for _, op := range f.Operations {
		switch op {
		case common.CreateOperation, common.UpdateOperation, common.DeleteOperation:
		default:
			return common.ErrInvalidOperationType
		}
	}
	return nil
}

type Options struct {
	SendEverything bool     `json:"send-everything,omitempty" jsonschema:"title=send everything, description=send all events,default=false"`
	Filters        []Filter `json:"filters,omitempty" jsonschema:"title=filters,description=A list of filters to apply to the events. This is ignored when send-everything is true"`
}

func (o Options) Validate() error {
	if o.SendEverything {
		return nil
	}
	if len(o.Filters) == 0 {
		return common.ErrNoFiltersProvided
	}
	for _, f := range o.Filters {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	return nil
}
