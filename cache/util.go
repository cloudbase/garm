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
package cache

import (
	"sort"

	"github.com/cloudbase/garm/params"
)

func sortByID[T params.IDGetter](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].GetID() < s[j].GetID()
	})
}

func sortByCreationDate[T params.CreationDateGetter](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].GetCreatedAt().Before(s[j].GetCreatedAt())
	})
}
