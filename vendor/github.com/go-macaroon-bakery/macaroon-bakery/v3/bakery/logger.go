package bakery

import (
	"context"
)

// Logger is used by the bakery to log informational messages
// about bakery operations.
type Logger interface {
	Infof(ctx context.Context, f string, args ...interface{})
	Debugf(ctx context.Context, f string, args ...interface{})
}

// DefaultLogger returns a Logger instance that does nothing.
//
// Deprecated: DefaultLogger exists for historical compatibility
// only. Previously it logged using github.com/juju/loggo.
func DefaultLogger(name string) Logger {
	return nopLogger{}
}

type nopLogger struct{}

// Debugf implements Logger.Debugf.
func (nopLogger) Debugf(context.Context, string, ...interface{}) {}

// Debugf implements Logger.Infof.
func (nopLogger) Infof(context.Context, string, ...interface{}) {}
