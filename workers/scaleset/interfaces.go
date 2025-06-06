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
package scaleset

import (
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

type scaleSetHelper interface {
	GetScaleSet() params.ScaleSet
	GetScaleSetClient() (*scalesets.ScaleSetClient, error)
	SetLastMessageID(id int64) error
	SetDesiredRunnerCount(count int) error
	Owner() string
	HandleJobsCompleted(jobs []params.ScaleSetJobMessage) error
	HandleJobsStarted(jobs []params.ScaleSetJobMessage) error
	HandleJobsAvailable(jobs []params.ScaleSetJobMessage) error
}
