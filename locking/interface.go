package locking

import "time"

type Locker interface {
	TryLock(key, identifier string) bool
	Lock(key, identifier string)
	LockedBy(key string) (string, bool)
	Unlock(key string, remove bool)
	Delete(key string)
}

type InstanceDeleteBackoff interface {
	ShouldProcess(key string) (bool, time.Time)
	Delete(key string)
	RecordFailure(key string)
}
