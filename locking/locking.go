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

package locking

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
)

var locker Locker

var lockerMux = sync.Mutex{}

func TryLock(key, identifier string) (ok bool) {
	if locker == nil {
		panic("no locker is registered")
	}

	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to try lock", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("try lock returned", "key", key, "identifier", identifier, "locked", ok, "caller", fmt.Sprintf("%s:%d", filename, line))

	ok = locker.TryLock(key, identifier)
	return ok
}

func Lock(key, identifier string) {
	if locker == nil {
		panic("no locker is registered")
	}

	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to lock", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("lock acquired", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))

	locker.Lock(key, identifier)
}

func LockWithContext(ctx context.Context, key, identifier string) error {
	if locker == nil {
		panic("no locker is registered")
	}

	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to lock with context", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("lock acquired", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))

	return locker.LockWithContext(ctx, key, identifier)
}

func Unlock(key string, remove bool) {
	if locker == nil {
		panic("no locker is registered")
	}

	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to unlock", "key", key, "remove", remove, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("unlock completed", "key", key, "remove", remove, "caller", fmt.Sprintf("%s:%d", filename, line))
	locker.Unlock(key, remove)
}

func LockedBy(key string) (string, bool) {
	if locker == nil {
		panic("no locker is registered")
	}

	return locker.LockedBy(key)
}

func Delete(key string) {
	if locker == nil {
		panic("no locker is registered")
	}

	locker.Delete(key)
}

func RegisterLocker(lock Locker) error {
	lockerMux.Lock()
	defer lockerMux.Unlock()

	if locker != nil {
		return fmt.Errorf("locker already registered")
	}

	locker = lock
	return nil
}
