package watcher

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

type IDGetter interface {
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

// WithAll returns a filter function that returns true if all of the provided filters return true.
func WithAll(filters ...dbCommon.PayloadFilterFunc) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		for _, filter := range filters {
			if !filter(payload) {
				return false
			}
		}
		return true
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
func WithEntityPoolFilter(ghEntity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		switch payload.EntityType {
		case dbCommon.PoolEntityType:
			pool, ok := payload.Payload.(params.Pool)
			if !ok {
				return false
			}
			switch ghEntity.EntityType {
			case params.ForgeEntityTypeRepository:
				return pool.RepoID == ghEntity.ID
			case params.ForgeEntityTypeOrganization:
				return pool.OrgID == ghEntity.ID
			case params.ForgeEntityTypeEnterprise:
				return pool.EnterpriseID == ghEntity.ID
			default:
				return false
			}
		default:
			return false
		}
	}
}

// WithEntityPoolFilter returns true if the change payload is a pool that belongs to the
// supplied Github entity. This is useful when an entity worker wants to watch for changes
// in pools that belong to it.
func WithEntityScaleSetFilter(ghEntity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		switch payload.EntityType {
		case dbCommon.ScaleSetEntityType:
			scaleSet, ok := payload.Payload.(params.ScaleSet)
			if !ok {
				return false
			}
			switch ghEntity.EntityType {
			case params.ForgeEntityTypeRepository:
				return scaleSet.RepoID == ghEntity.ID
			case params.ForgeEntityTypeOrganization:
				return scaleSet.OrgID == ghEntity.ID
			case params.ForgeEntityTypeEnterprise:
				return scaleSet.EnterpriseID == ghEntity.ID
			default:
				return false
			}
		default:
			return false
		}
	}
}

// WithEntityFilter returns a filter function that filters payloads by entity.
// Change payloads that match the entity type and ID will return true.
func WithEntityFilter(entity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if params.ForgeEntityType(payload.EntityType) != entity.EntityType {
			return false
		}
		var ent IDGetter
		var ok bool
		switch payload.EntityType {
		case dbCommon.RepositoryEntityType:
			if entity.EntityType != params.ForgeEntityTypeRepository {
				return false
			}
			ent, ok = payload.Payload.(params.Repository)
		case dbCommon.OrganizationEntityType:
			if entity.EntityType != params.ForgeEntityTypeOrganization {
				return false
			}
			ent, ok = payload.Payload.(params.Organization)
		case dbCommon.EnterpriseEntityType:
			if entity.EntityType != params.ForgeEntityTypeEnterprise {
				return false
			}
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

func WithEntityJobFilter(ghEntity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		switch payload.EntityType {
		case dbCommon.JobEntityType:
			job, ok := payload.Payload.(params.Job)
			if !ok {
				return false
			}

			switch ghEntity.EntityType {
			case params.ForgeEntityTypeRepository:
				if job.RepoID != nil && job.RepoID.String() != ghEntity.ID {
					return false
				}
			case params.ForgeEntityTypeOrganization:
				if job.OrgID != nil && job.OrgID.String() != ghEntity.ID {
					return false
				}
			case params.ForgeEntityTypeEnterprise:
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

// WithGithubCredentialsFilter returns a filter function that filters payloads by Github credentials.
func WithForgeCredentialsFilter(creds params.ForgeCredentials) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		var idGetter params.IDGetter
		var ok bool
		switch payload.EntityType {
		case dbCommon.GithubCredentialsEntityType:
			idGetter, ok = payload.Payload.(params.ForgeCredentials)
		default:
			return false
		}
		if !ok {
			return false
		}
		return idGetter.GetID() == creds.GetID()
	}
}

// WithUserIDFilter returns a filter function that filters payloads by user ID.
func WithUserIDFilter(userID string) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if payload.EntityType != dbCommon.UserEntityType {
			return false
		}
		userPayload, ok := payload.Payload.(params.User)
		if !ok {
			return false
		}
		return userPayload.ID == userID
	}
}

// WithNone returns a filter function that always returns false.
func WithNone() dbCommon.PayloadFilterFunc {
	return func(_ dbCommon.ChangePayload) bool {
		return false
	}
}

// WithEverything returns a filter function that always returns true.
func WithEverything() dbCommon.PayloadFilterFunc {
	return func(_ dbCommon.ChangePayload) bool {
		return true
	}
}

// WithExcludeEntityTypeFilter returns a filter function that filters payloads by excluding
// the provided entity type.
func WithExcludeEntityTypeFilter(entityType dbCommon.DatabaseEntityType) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		return payload.EntityType != entityType
	}
}

// WithScaleSetFilter returns a filter function that matches a particular scale set.
func WithScaleSetFilter(scaleset params.ScaleSet) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if payload.EntityType != dbCommon.ScaleSetEntityType {
			return false
		}

		ss, ok := payload.Payload.(params.ScaleSet)
		if !ok {
			return false
		}

		return ss.ID == scaleset.ID
	}
}

func WithScaleSetInstanceFilter(scaleset params.ScaleSet) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if payload.EntityType != dbCommon.InstanceEntityType {
			return false
		}

		instance, ok := payload.Payload.(params.Instance)
		if !ok || instance.ScaleSetID == 0 {
			return false
		}

		return instance.ScaleSetID == scaleset.ID
	}
}

// EntityTypeCallbackFilter is a callback function that takes a ChangePayload and returns a boolean.
// This callback type is used in the WithEntityTypeAndCallbackFilter (and potentially others) when
// a filter needs to delegate logic to a specific callback function.
type EntityTypeCallbackFilter func(payload dbCommon.ChangePayload) (bool, error)

// WithEntityTypeAndCallbackFilter returns a filter function that filters payloads by entity type and the
// result of a callback function.
func WithEntityTypeAndCallbackFilter(entityType dbCommon.DatabaseEntityType, callback EntityTypeCallbackFilter) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if payload.EntityType != entityType {
			return false
		}

		ok, err := callback(payload)
		if err != nil {
			return false
		}
		return ok
	}
}

func WithInstanceStatusFilter(statuses ...commonParams.InstanceStatus) dbCommon.PayloadFilterFunc {
	return func(payload dbCommon.ChangePayload) bool {
		if payload.EntityType != dbCommon.InstanceEntityType {
			return false
		}
		instance, ok := payload.Payload.(params.Instance)
		if !ok {
			return false
		}
		if len(statuses) == 0 {
			return false
		}
		for _, status := range statuses {
			if instance.Status == status {
				return true
			}
		}
		return false
	}
}
