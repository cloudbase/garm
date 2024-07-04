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
