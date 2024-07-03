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
	case common.RepositoryEntityType, common.OrganizationEntityType, common.EnterpriseEntityType, common.PoolEntityType, common.UserEntityType, common.InstanceEntityType, common.JobEntityType, common.ControllerEntityType, common.GithubCredentialsEntityType, common.GithubEndpointEntityType:
	default:
		return nil
	}
	return nil
}

type Options struct {
	SendEverything bool     `json:"send_everything"`
	Filters        []Filter `json:"filters"`
}
