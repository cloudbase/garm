package e2e

import (
	"encoding/json"
	"log"
)

func printJsonResponse(resp interface{}) error {
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	log.Println(string(b))
	return nil
}
