// Copyright 2022 Cloudbase Solutions SRL
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
	"errors"
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/nbutton23/zxcvbn-go"
)

func PromptPassword(label string, compareTo string) (string, error) {
	if label == "" {
		label = "Password"
	}
	validate := func(input string) error {
		passwordStenght := zxcvbn.PasswordStrength(input, nil)
		if passwordStenght.Score < 4 {
			return errors.New("password is too weak")
		}
		if compareTo != "" && compareTo != input {
			return errors.New("passwords do not match")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    label,
		Validate: validate,
		Mask:     '*',
	}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}

func PromptString(label string, a ...interface{}) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("empty input not allowed")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    fmt.Sprintf(label, a...),
		Validate: validate,
	}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}
