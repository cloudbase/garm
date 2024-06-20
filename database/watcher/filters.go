package watcher

import (
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

type idGetter interface {
	GetID() string
}

// WithAny returns a filter function that returns true if any of the provided filters return true.
// This filter is useful if for example you want to watch for update operations on any of the supplied
// entities.
// Example:
//
//	// Watch for any update operation on repositories or organizations
//	consumer.SetFilters(
//		watcher.WithOperationTypeFilter(common.UpdateOperation),
//		watcher.WithAny(
//			watcher.WithEntityTypeFilter(common.RepositoryEntityType),
//			watcher.WithEntityTypeFilter(common.OrganizationEntityType),
//	))
func WithAny(filters ...dbCommon.PayloadFilterFunc) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		for _, filter := range filters {
			if filter(payload) {
				return true
			}
		}
		return false
	}
}

// WithEntityTypeFilter returns a filter function that filters payloads by entity type.
// The filter function returns true if the payload's entity type matches the provided entity type.
func WithEntityTypeFilter(entityType dbCommon.DatabaseEntityType) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		return payload.EntityType == entityType
	}
}

// WithOperationTypeFilter returns a filter function that filters payloads by operation type.
func WithOperationTypeFilter(operationType dbCommon.OperationType) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		return payload.Operation == operationType
	}
}

// WithEntityPoolFilter returns true if the change payload is a pool that belongs to the
// supplied Github entity. This is useful when an entity worker wants to watch for changes
// in pools that belong to it.
func WithEntityPoolFilter(ghEntity params.GithubEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		switch payload.EntityType {
		case dbCommon.PoolEntityType:
			pool, ok := payload.Payload.(params.Pool)
			if !ok {
				return false
			}
			switch ghEntity.EntityType {
			case params.GithubEntityTypeRepository:
				if pool.RepoID != ghEntity.ID {
					return false
				}
			case params.GithubEntityTypeOrganization:
				if pool.OrgID != ghEntity.ID {
					return false
				}
			case params.GithubEntityTypeEnterprise:
				if pool.EnterpriseID != ghEntity.ID {
					return false
				}
			default:
				return false
			}
			return true
		default:
			return false
		}
	}
}

// WithEntityFilter returns a filter function that filters payloads by entity.
// Change payloads that match the entity type and ID will return true.
func WithEntityFilter(entity params.GithubEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if params.GithubEntityType(payload.EntityType) != entity.EntityType {
			return false
		}
		var ent idGetter
		var ok bool
		switch payload.EntityType {
		case dbCommon.RepositoryEntityType:
			ent, ok = payload.Payload.(params.Repository)
		case dbCommon.OrganizationEntityType:
			ent, ok = payload.Payload.(params.Organization)
		case dbCommon.EnterpriseEntityType:
			ent, ok = payload.Payload.(params.Enterprise)
		default:
			return false
		}
		if !ok {
			return false
		}
		return ent.GetID() == entity.ID
	}
}

func WithEntityJobFilter(ghEntity params.GithubEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		switch payload.EntityType {
		case dbCommon.JobEntityType:
			job, ok := payload.Payload.(params.Job)
			if !ok {
				return false
			}

			switch ghEntity.EntityType {
			case params.GithubEntityTypeRepository:
				if job.RepoID != nil && job.RepoID.String() != ghEntity.ID {
					return false
				}
			case params.GithubEntityTypeOrganization:
				if job.OrgID != nil && job.OrgID.String() != ghEntity.ID {
					return false
				}
			case params.GithubEntityTypeEnterprise:
				if job.EnterpriseID != nil && job.EnterpriseID.String() != ghEntity.ID {
					return false
				}
			default:
				return false
			}

			return true
		default:
			return false
		}
	}
}
