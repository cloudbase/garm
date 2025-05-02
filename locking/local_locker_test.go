package locking

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type LockerTestSuite struct {
	suite.Suite

	mux *keyMutex
}

func (l *LockerTestSuite) SetupTest() {
	l.mux = &keyMutex{}
	err := RegisterLocker(l.mux)
	l.Require().NoError(err, "should register the locker")
}

func (l *LockerTestSuite) TearDownTest() {
	l.mux = nil
	locker = nil
}

func (l *LockerTestSuite) TestLocalLockerLockUnlock() {
	l.mux.Lock("test", "test-identifier")
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)
	l.mux.Unlock("test", true)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().False(ok)
	l.Require().Nil(mux)
	l.mux.Unlock("test", false)
}

func (l *LockerTestSuite) TestLocalLockerTryLock() {
	locked := l.mux.TryLock("test", "test-identifier")
	l.Require().True(locked)
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	locked = l.mux.TryLock("test", "another-identifier2")
	l.Require().False(locked)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux = mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	l.mux.Unlock("test", true)
	locked = l.mux.TryLock("test", "another-identifier2")
	l.Require().True(locked)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux = mux.(*lockWithIdent)
	l.Require().Equal("another-identifier2", keyMux.ident)
	l.mux.Unlock("test", true)
}

func (l *LockerTestSuite) TestLocalLockertLockedBy() {
	l.mux.Lock("test", "test-identifier")
	identifier, ok := l.mux.LockedBy("test")
	l.Require().True(ok)
	l.Require().Equal("test-identifier", identifier)
	l.mux.Unlock("test", true)
	identifier, ok = l.mux.LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)

	l.mux.Lock("test", "test-identifier")
	identifier, ok = l.mux.LockedBy("test")
	l.Require().True(ok)
	l.Require().Equal("test-identifier", identifier)
	l.mux.Unlock("test", false)
	identifier, ok = l.mux.LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)
}

func (l *LockerTestSuite) TestLockerPanicsIfNotInitialized() {
	locker = nil
	l.Require().Panics(
		func() {
			Lock("test", "test-identifier")
		},
		"Lock should panic if locker is not initialized",
	)

	l.Require().Panics(
		func() {
			TryLock("test", "test-identifier")
		},
		"TryLock should panic if locker is not initialized",
	)

	l.Require().Panics(
		func() {
			Unlock("test", false)
		},
		"Unlock should panic if locker is not initialized",
	)

	l.Require().Panics(
		func() {
			Delete("test")
		},
		"Delete should panic if locker is not initialized",
	)

	l.Require().Panics(
		func() {
			LockedBy("test")
		},
		"LockedBy should panic if locker is not initialized",
	)
}

func (l *LockerTestSuite) TestLockerAlreadyRegistered() {
	err := RegisterLocker(l.mux)
	l.Require().Error(err, "should not be able to register the same locker again")
	l.Require().Equal("locker already registered", err.Error())
}

func (l *LockerTestSuite) TestLockerDelete() {
	Lock("test", "test-identifier")
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	Delete("test")
	mux, ok = l.mux.muxes.Load("test")
	l.Require().False(ok)
	l.Require().Nil(mux)

	identifier, ok := l.mux.LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)
}

func (l *LockerTestSuite) TestLockUnlock() {
	Lock("test", "test-identifier")
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	Unlock("test", true)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().False(ok)
	l.Require().Nil(mux)

	identifier, ok := l.mux.LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)
}

func (l *LockerTestSuite) TestLockUnlockWithoutRemove() {
	Lock("test", "test-identifier")
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	Unlock("test", false)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux = mux.(*lockWithIdent)
	l.Require().Equal("", keyMux.ident)

	identifier, ok := l.mux.LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)
}

func (l *LockerTestSuite) TestTryLock() {
	locked := TryLock("test", "test-identifier")
	l.Require().True(locked)
	mux, ok := l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux := mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	locked = TryLock("test", "another-identifier2")
	l.Require().False(locked)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux = mux.(*lockWithIdent)
	l.Require().Equal("test-identifier", keyMux.ident)

	Unlock("test", true)
	locked = TryLock("test", "another-identifier2")
	l.Require().True(locked)
	mux, ok = l.mux.muxes.Load("test")
	l.Require().True(ok)
	keyMux = mux.(*lockWithIdent)
	l.Require().Equal("another-identifier2", keyMux.ident)
	Unlock("test", true)
}

func (l *LockerTestSuite) TestLockedBy() {
	Lock("test", "test-identifier")
	identifier, ok := LockedBy("test")
	l.Require().True(ok)
	l.Require().Equal("test-identifier", identifier)
	Unlock("test", true)
	identifier, ok = LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)

	Lock("test", "test-identifier2")
	identifier, ok = LockedBy("test")
	l.Require().True(ok)
	l.Require().Equal("test-identifier2", identifier)
	Unlock("test", false)
	identifier, ok = LockedBy("test")
	l.Require().False(ok)
	l.Require().Equal("", identifier)
}

func TestLockerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(LockerTestSuite))
}
