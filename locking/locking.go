package locking

import (
	"fmt"
	"sync"
)

var locker Locker
var lockerMux = sync.Mutex{}

func TryLock(key string) (bool, error) {
	if locker == nil {
		return false, fmt.Errorf("no locker is registered")
	}

	return locker.TryLock(key), nil
}
func Unlock(key string, remove bool) error {
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
