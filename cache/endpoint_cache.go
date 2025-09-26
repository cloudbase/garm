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
	"sync"

	"github.com/cloudbase/garm/params"
)

var endpointCache *EndpointCache

func init() {
	epCache := &EndpointCache{
		endpoints: make(map[string]params.ForgeEndpoint),
	}
	endpointCache = epCache
}

type EndpointCache struct {
	endpoints map[string]params.ForgeEndpoint
	mux       sync.Mutex
}

func (e *EndpointCache) SetEndpoint(ep params.ForgeEndpoint) {
	e.mux.Lock()
	defer e.mux.Unlock()

	e.endpoints[ep.Name] = ep
	UpdateCredentialsUsingEndpoint(ep)
}

func (e *EndpointCache) GetEndpoint(epName string) (params.ForgeEndpoint, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	ep, ok := e.endpoints[epName]
	if ok {
		return ep, true
	}
	return params.ForgeEndpoint{}, false
}

func (e *EndpointCache) RemoveEndpoint(epName string) {
	e.mux.Lock()
	defer e.mux.Unlock()

	delete(e.endpoints, epName)
}

func SetEndpoint(ep params.ForgeEndpoint) {
	endpointCache.SetEndpoint(ep)
}

func GetEndpoint(epName string) (params.ForgeEndpoint, bool) {
	return endpointCache.GetEndpoint(epName)
}

func RemoveEndpoint(epName string) {
	endpointCache.RemoveEndpoint(epName)
}
