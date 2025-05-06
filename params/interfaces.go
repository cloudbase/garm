package params

// EntityGetter is implemented by all github entities (repositories, organizations and enterprises).
// It defines the GetEntity() function which returns a github entity.
type EntityGetter interface {
	GetEntity() (GithubEntity, error)
}
