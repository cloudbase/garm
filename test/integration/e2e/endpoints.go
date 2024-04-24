package e2e

func MustDefaultGithubEndpoint() {
	ep := GetGithubEndpoint("github.com")
	if ep == nil {
		panic("Default GitHub endpoint not found")
	}

	if ep.Name != "github.com" {
		panic("Default GitHub endpoint name mismatch")
	}
}
