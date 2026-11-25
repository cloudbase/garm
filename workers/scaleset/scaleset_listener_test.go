// Copyright 2026 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

//go:build testing

package scaleset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

// stubScaleSetHelper implements just enough of scaleSetHelper to exercise
// statistics handling, and records what was persisted.
type stubScaleSetHelper struct {
	scaleSet params.ScaleSet

	statistics []params.RunnerScaleSetStatistic
	started    [][]params.ScaleSetJobMessage
}

func (s *stubScaleSetHelper) GetScaleSet() params.ScaleSet { return s.scaleSet }
func (s *stubScaleSetHelper) GetScaleSetClient() (*scalesets.ScaleSetClient, error) {
	return nil, nil
}
func (s *stubScaleSetHelper) SetLastMessageID(_ int64) error { return nil }
func (s *stubScaleSetHelper) SetRunnerStatistics(stats params.RunnerScaleSetStatistic) error {
	s.statistics = append(s.statistics, stats)
	return nil
}

func (s *stubScaleSetHelper) Owner() string                                           { return "test-owner" }
func (s *stubScaleSetHelper) HandleJobsCompleted(_ []params.ScaleSetJobMessage) error { return nil }
func (s *stubScaleSetHelper) HandleJobsStarted(jobs []params.ScaleSetJobMessage) error {
	s.started = append(s.started, jobs)
	return nil
}
func (s *stubScaleSetHelper) HandleJobsAvailable(_ []params.ScaleSetJobMessage) error { return nil }

func newTestListener() (*scaleSetListener, *stubScaleSetHelper) {
	helper := &stubScaleSetHelper{
		scaleSet: params.ScaleSet{
			ID:           7,
			ScaleSetID:   42,
			Name:         "test-scaleset",
			ProviderName: "test-provider",
		},
	}
	return newListener(context.Background(), helper), helper
}

// GitHub attaches its statistics to every message, whatever the type. They
// are the only ungated view of the scale set's queue (TotalAvailableJobs is
// not capped by the capacity we advertise), so they must be persisted even
// for messages the listener otherwise ignores. Otherwise the API serves a
// stale number for as long as the scale set sees nothing but such messages.
func TestHandleSessionMessagePersistsStatisticsForNonJobMessages(t *testing.T) {
	l, helper := newTestListener()

	l.handleSessionMessage(params.RunnerScaleSetMessage{
		MessageID:   1,
		MessageType: "SomeOtherMessageType",
		Statistics: &params.RunnerScaleSetStatistic{
			TotalAvailableJobs: 99,
			TotalAssignedJobs:  10,
			TotalIdleRunners:   3,
		},
	})

	require.Len(t, helper.statistics, 1)
	got := helper.statistics[0]
	require.Equal(t, 99, got.TotalAvailableJobs)
	require.Equal(t, 10, got.TotalAssignedJobs)
	require.Equal(t, 3, got.TotalIdleRunners)
	require.Empty(t, helper.started, "a non job message must not be handled as jobs")
}

func TestHandleSessionMessageNilStatisticsIsNoop(t *testing.T) {
	l, helper := newTestListener()

	l.handleSessionMessage(params.RunnerScaleSetMessage{
		MessageID:   1,
		MessageType: "SomeOtherMessageType",
	})

	require.Empty(t, helper.statistics)
}
