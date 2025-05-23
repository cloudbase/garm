// Copyright 2023 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/mattn/go-isatty"
)

type ExecutionCommand string

const (
	CreateInstanceCommand     ExecutionCommand = "CreateInstance"
	DeleteInstanceCommand     ExecutionCommand = "DeleteInstance"
	GetInstanceCommand        ExecutionCommand = "GetInstance"
	ListInstancesCommand      ExecutionCommand = "ListInstances"
	StartInstanceCommand      ExecutionCommand = "StartInstance"
	StopInstanceCommand       ExecutionCommand = "StopInstance"
	RemoveAllInstancesCommand ExecutionCommand = "RemoveAllInstances"
	GetVersionCommand         ExecutionCommand = "GetVersion"
)

// V0.1.1 commands
const (
	GetSupportedInterfaceVersionsCommand ExecutionCommand = "GetSupportedInterfaceVersions"
	ValidatePoolInfoCommand              ExecutionCommand = "ValidatePoolInfo"
	GetConfigJSONSchemaCommand           ExecutionCommand = "GetConfigJSONSchema"
	GetExtraSpecsJSONSchemaCommand       ExecutionCommand = "GetExtraSpecsJSONSchema"
)

const (
	// ExitCodeNotFound is an exit code that indicates a Not Found error
	ExitCodeNotFound int = 30
	// ExitCodeDuplicate is an exit code that indicates a duplicate error
	ExitCodeDuplicate int = 31
)

func GetBoostrapParamsFromStdin(c ExecutionCommand) (params.BootstrapInstance, error) {
	var bootstrapParams params.BootstrapInstance
	if c == CreateInstanceCommand {
		if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			return params.BootstrapInstance{}, fmt.Errorf("%s requires data passed into stdin", CreateInstanceCommand)
		}

		var data bytes.Buffer
		if _, err := io.Copy(&data, os.Stdin); err != nil {
			return params.BootstrapInstance{}, fmt.Errorf("failed to copy bootstrap params")
		}

		if data.Len() == 0 {
			return params.BootstrapInstance{}, fmt.Errorf("%s requires data passed into stdin", CreateInstanceCommand)
		}

		if err := json.Unmarshal(data.Bytes(), &bootstrapParams); err != nil {
			return params.BootstrapInstance{}, fmt.Errorf("failed to decode instance params: %w", err)
		}
		if bootstrapParams.ExtraSpecs == nil {
			// Initialize ExtraSpecs as an empty JSON object
			bootstrapParams.ExtraSpecs = json.RawMessage([]byte("{}"))
		}

		return bootstrapParams, nil
	}

	// If the command is not CreateInstance, we don't need to read from stdin
	return params.BootstrapInstance{}, nil
}

func ResolveErrorToExitCode(err error) int {
	if err != nil {
		if errors.Is(err, gErrors.ErrNotFound) {
			return ExitCodeNotFound
		} else if errors.Is(err, gErrors.ErrDuplicateEntity) {
			return ExitCodeDuplicate
		}
		return 1
	}
	return 0
}
