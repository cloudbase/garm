// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
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
