package e2e

import (
	"encoding/json"
	"log"
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

func expectAPIStatusCode(err error, expectedCode int) {
	if err == nil {
		panic("expected error")
	}
	apiErr, ok := err.(apiCodeGetter)
	if !ok {
		log.Fatalf("expected API error, got %v (%T)", err, err)
	}
	if !apiErr.IsCode(expectedCode) {
		log.Fatalf("expected status code %d: %v", expectedCode, err)
	}
}
