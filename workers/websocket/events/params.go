// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
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
		common.GiteaCredentialsEntityType, common.ScaleSetEntityType, common.GithubEndpointEntityType,
		common.TemplateEntityType, common.FileObjectEntityType:
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
