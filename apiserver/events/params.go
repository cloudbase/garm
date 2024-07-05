package events

import (
	"github.com/cloudbase/garm/database/common"
)

type Filter struct {
	Operations []common.OperationType    `json:"operations"`
	EntityType common.DatabaseEntityType `json:"entity_type"`
}

func (f Filter) Validate() error {
	switch f.EntityType {
	case common.RepositoryEntityType, common.OrganizationEntityType, common.EnterpriseEntityType,
		common.PoolEntityType, common.UserEntityType, common.InstanceEntityType,
		common.JobEntityType, common.ControllerEntityType, common.GithubCredentialsEntityType,
		common.GithubEndpointEntityType:
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
	SendEverything bool     `json:"send_everything"`
	Filters        []Filter `json:"filters"`
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
