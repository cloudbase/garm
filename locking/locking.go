package locking

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
)

var locker Locker
var lockerMux = sync.Mutex{}

func TryLock(key, identifier string) (ok bool, err error) {
	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to try lock", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("try lock returned", "key", key, "identifier", identifier, "locked", ok, "caller", fmt.Sprintf("%s:%d", filename, line))
	if locker == nil {
		return false, fmt.Errorf("no locker is registered")
	}

	ok = locker.TryLock(key, identifier)
	return ok, nil
}

func Lock(key, identifier string) {
	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to lock", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))
	defer slog.Debug("lock acquired", "key", key, "identifier", identifier, "caller", fmt.Sprintf("%s:%d", filename, line))

	if locker == nil {
		panic("no locker is registered")
	}

	locker.Lock(key, identifier)
}

func Unlock(key string, remove bool) error {
	_, filename, line, _ := runtime.Caller(1)
	slog.Debug("attempting to unlock", "key", key, "remove", remove, "caller", fmt.Sprintf("%s:%d", filename, line))
	if locker == nil {
		return fmt.Errorf("no locker is registered")
	}

	locker.Unlock(key, remove)
	return nil
}

func Delete(key string) error {
	if locker == nil {
		return fmt.Errorf("no locker is registered")
	}

	locker.Delete(key)
	return nil
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
