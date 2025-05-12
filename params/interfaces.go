package params

import "time"

// EntityGetter is implemented by all github entities (repositories, organizations and enterprises).
// It defines the GetEntity() function which returns a github entity.
type EntityGetter interface {
	GetEntity() (ForgeEntity, error)
}

type IDGetter interface {
	GetID() uint
}

type CreationDateGetter interface {
	GetCreatedAt() time.Time
}

type ForgeCredentialsGetter interface {
	GetForgeCredentials() ForgeCredentials
}
