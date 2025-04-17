package locking

import "time"

// TODO(gabriel-samfira): needs owner attribute.
type Locker interface {
	TryLock(key string) bool
	Lock(key string)
	Unlock(key string, remove bool)
	Delete(key string)
}

type InstanceDeleteBackoff interface {
	ShouldProcess(key string) (bool, time.Time)
	Delete(key string)
	RecordFailure(key string)
}
