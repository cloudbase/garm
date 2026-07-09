// Copyright 2026 Cloudbase Solutions SRL
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

package cache

import (
	"github.com/cloudbase/garm/params"
)

var proxyCache = newKeyedCache[uint, params.Proxy](0)

func SetProxyCache(proxy params.Proxy) {
	proxyCache.Set(proxy.ID, proxy)
}

func GetProxy(id uint) (params.Proxy, bool) {
	return proxyCache.Get(id)
}

func ListProxies() []params.Proxy {
	ret := proxyCache.List()
	sortByID(ret)
	return ret
}

func DeleteProxy(id uint) {
	proxyCache.Delete(id)
}
