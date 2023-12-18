package pool

import (
	"net/url"
	"strings"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
	"github.com/google/go-github/v57/github"
	"github.com/pkg/errors"
)

func validateHookRequest(controllerID, baseURL string, allHooks []*github.Hook, req *github.Hook) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrap(err, "parsing webhook url")
	}

	partialMatches := []string{}
	for _, hook := range allHooks {
		hookURL, ok := hook.Config["url"].(string)
		if !ok {
			continue
		}
		hookURL = strings.ToLower(hookURL)

		if hook.Config["url"] == req.Config["url"] {
			return runnerErrors.NewConflictError("hook already installed")
		} else if strings.Contains(hookURL, controllerID) || strings.Contains(hookURL, parsed.Hostname()) {
			partialMatches = append(partialMatches, hook.Config["url"].(string))
		}
	}

	if len(partialMatches) > 0 {
		return runnerErrors.NewConflictError("a webhook containing the controller ID or hostname of this contreoller is already installed on this repository")
	}

	return nil
}

func hookToParamsHookInfo(hook *github.Hook) params.HookInfo {
	var hookURL string
	url, ok := hook.Config["url"]
	if ok {
		hookURL = url.(string)
	}

	var insecureSSL bool
	insecureSSLConfig, ok := hook.Config["insecure_ssl"]
	if ok {
		if insecureSSLConfig.(string) == "1" {
			insecureSSL = true
		}
	}

	return params.HookInfo{
		ID:          *hook.ID,
		URL:         hookURL,
		Events:      hook.Events,
		Active:      *hook.Active,
		InsecureSSL: insecureSSL,
	}
}
