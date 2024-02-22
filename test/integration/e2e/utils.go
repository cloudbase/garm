package e2e

import (
	"encoding/json"
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
