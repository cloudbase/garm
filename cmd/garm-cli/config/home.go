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

package config

import (
	"os"

	"github.com/pkg/errors"
)

func ensureHomeDir(folder string) error {
	if _, err := os.Stat(folder); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "checking home dir")
		}

		if err := os.MkdirAll(folder, 0o710); err != nil {
			return errors.Wrapf(err, "creating %s", folder)
		}
	}

	return nil
}
