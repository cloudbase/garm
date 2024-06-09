package integration

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

func printJSONResponse(resp interface{}) error {
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	slog.Info(string(b))
	return nil
}

type apiCodeGetter interface {
	IsCode(code int) bool
}

func expectAPIStatusCode(err error, expectedCode int) error {
	if err == nil {
		return fmt.Errorf("expected error, got nil")
	}
	apiErr, ok := err.(apiCodeGetter)
	if !ok {
		return fmt.Errorf("expected API error, got %v (%T)", err, err)
	}
	if !apiErr.IsCode(expectedCode) {
		return fmt.Errorf("expected status code %d: %v", expectedCode, err)
	}

	return nil
}
