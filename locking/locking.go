package locking

import (
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
