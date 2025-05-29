package controllers

import (
	"net/url"

	controllerErrors "github.com/cloudbase/garm-provider-common/errors"
)

func unescapeVars(vars map[string]string) (map[string]string, error) {
	unescapedVars := make(map[string]string, len(vars))
	for key, value := range vars {
		unescapedValue, err := url.PathUnescape(value)
		if err != nil {
			return nil, controllerErrors.NewBadRequestError("invalid repository ID: %s", err.Error())
		} else {
			unescapedVars[key] = unescapedValue
		}
	}
	return unescapedVars, nil
}
