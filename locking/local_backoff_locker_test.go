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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type LockerBackoffTestSuite struct {
	suite.Suite

	locker *instanceDeleteBackoff
}

func (l *LockerBackoffTestSuite) SetupTest() {
	l.locker = &instanceDeleteBackoff{}
}

func (l *LockerBackoffTestSuite) TearDownTest() {
	l.locker = nil
}

func (l *LockerBackoffTestSuite) TestShouldProcess() {
	shouldProcess, deadline := l.locker.ShouldProcess("test")
	l.Require().True(shouldProcess)
	l.Require().Equal(time.Time{}, deadline)

	l.locker.muxes.Store("test", &instanceBackOff{
		backoffSeconds:          0,
		lastRecordedFailureTime: time.Time{},
	})

	shouldProcess, deadline = l.locker.ShouldProcess("test")
	l.Require().True(shouldProcess)
	l.Require().Equal(time.Time{}, deadline)

	l.locker.muxes.Store("test", &instanceBackOff{
		backoffSeconds:          100,
		lastRecordedFailureTime: time.Now().UTC(),
	})

	shouldProcess, deadline = l.locker.ShouldProcess("test")
	l.Require().False(shouldProcess)
	l.Require().NotEqual(time.Time{}, deadline)
}

func (l *LockerBackoffTestSuite) TestRecordFailure() {
	l.locker.RecordFailure("test")

	mux, ok := l.locker.muxes.Load("test")
	l.Require().True(ok)
	ib := mux.(*instanceBackOff)
	l.Require().NotNil(ib)
	l.Require().NotEqual(time.Time{}, ib.lastRecordedFailureTime)
	l.Require().Equal(float64(5), ib.backoffSeconds)

	l.locker.RecordFailure("test")
	mux, ok = l.locker.muxes.Load("test")
	l.Require().True(ok)
	ib = mux.(*instanceBackOff)
	l.Require().NotNil(ib)
	l.Require().NotEqual(time.Time{}, ib.lastRecordedFailureTime)
	l.Require().Equal(7.5, ib.backoffSeconds)

	l.locker.Delete("test")
	mux, ok = l.locker.muxes.Load("test")
	l.Require().False(ok)
	l.Require().Nil(mux)
}

func TestBackoffTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(LockerBackoffTestSuite))
}
