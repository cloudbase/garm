// Copyright 2025 Cloudbase Solutions SRL
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

import "fmt"

var (
	ErrProducerClosed            = fmt.Errorf("producer is closed")
	ErrProducerTimeoutErr        = fmt.Errorf("producer timeout error")
	ErrProducerAlreadyRegistered = fmt.Errorf("producer already registered")
	ErrConsumerAlreadyRegistered = fmt.Errorf("consumer already registered")
	ErrWatcherAlreadyStarted     = fmt.Errorf("watcher already started")
	ErrWatcherNotInitialized     = fmt.Errorf("watcher not initialized")
	ErrInvalidOperationType      = fmt.Errorf("invalid operation")
	ErrInvalidEntityType         = fmt.Errorf("invalid entity type")
	ErrNoFiltersProvided         = fmt.Errorf("no filters provided")
)
