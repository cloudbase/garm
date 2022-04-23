package runner

type HookTargetType string

const (
	RepoHook         HookTargetType = "repository"
	OrganizationHook HookTargetType = "organization"
)
