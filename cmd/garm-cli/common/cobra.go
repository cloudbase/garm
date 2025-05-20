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
package common

import "fmt"

type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
)

func (o *OutputFormat) String() string {
	if o == nil {
		return ""
	}
	return string(*o)
}

func (o *OutputFormat) Set(value string) error {
	switch value {
	case "table", "json":
		*o = OutputFormat(value)
	default:
		return fmt.Errorf("allowed formats are: json, table")
	}
	return nil
}

func (o *OutputFormat) Type() string {
	return "string"
}
