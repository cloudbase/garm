//go:build integration
// +build integration

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
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	wsEvents "github.com/cloudbase/garm/workers/websocket/events"
)

// dbEvent is a single database change event received from GARM's events
// websocket endpoint. The payload is decoded on demand, based on EntityType.
type dbEvent struct {
	EntityType dbCommon.DatabaseEntityType `json:"entity-type"`
	Operation  dbCommon.OperationType      `json:"operation"`
	Payload    json.RawMessage             `json:"payload"`
}

func (e dbEvent) job() (params.Job, error) {
	var job params.Job
	if e.EntityType != dbCommon.JobEntityType {
		return job, fmt.Errorf("event is not a job event")
	}
	if err := json.Unmarshal(e.Payload, &job); err != nil {
		return job, fmt.Errorf("failed to decode job payload: %w", err)
	}
	return job, nil
}

func (e dbEvent) instance() (params.Instance, error) {
	var instance params.Instance
	if e.EntityType != dbCommon.InstanceEntityType {
		return instance, fmt.Errorf("event is not an instance event")
	}
	if err := json.Unmarshal(e.Payload, &instance); err != nil {
		return instance, fmt.Errorf("failed to decode instance payload: %w", err)
	}
	return instance, nil
}

// eventConsumer subscribes to GARM's /api/v1/ws/events endpoint and
// accumulates every event it receives, for the entire lifetime of the test
// suite. Tests wait for conditions over the accumulated events: a waiter
// scans everything received so far and then blocks for new events, so no
// state transition can be missed, no matter how fast it happens. To scope a
// wait to a particular action, grab a marker with mark() before performing
// the action and pass it to waitFor.
//
// The server drops events for websocket clients that fall behind (the send
// buffer is bounded), so the message handler must stay cheap. We only
// append to a slice; all decoding and matching happens on the waiter side.
type eventConsumer struct {
	reader *garmWs.Reader

	mux     sync.Mutex
	events  []dbEvent
	err     error
	arrived chan struct{}
}

// startEventConsumer connects to the events endpoint and subscribes to all
// events. It is meant to be called once, from SetupSuite.
func (suite *GarmSuite) startEventConsumer() *eventConsumer {
	consumer := &eventConsumer{
		arrived: make(chan struct{}),
	}

	reader, err := garmWs.NewReader(context.Background(), suite.baseURL, "/api/v1/ws/events", suite.apiToken, consumer.handleMessage)
	suite.Require().NoError(err, "error creating events websocket reader")
	suite.Require().NoError(reader.Start(), "error starting events websocket reader")
	consumer.reader = reader

	options := wsEvents.Options{
		SendEverything: true,
	}
	asJSON, err := json.Marshal(options)
	suite.Require().NoError(err, "error marshaling event filters")
	suite.Require().NoError(consumer.reader.WriteMessage(websocket.TextMessage, asJSON), "error sending event filters")

	return consumer
}

func (w *eventConsumer) handleMessage(_ int, msg []byte) error {
	var event dbEvent
	err := json.Unmarshal(msg, &event)

	w.mux.Lock()
	defer w.mux.Unlock()

	if err == nil && event.EntityType != "" && event.Operation != "" {
		w.events = append(w.events, event)
	} else {
		// Anything that is not a change payload is an error: the events
		// endpoint only ever sends change payloads, or a plain text error
		// (for example "invalid filter") right before it disconnects us.
		// Note that unmarshaling alone can not tell the two apart, since
		// any JSON object unmarshals into dbEvent just fine, leaving all
		// fields empty. Record the error so that waiters fail fast with
		// the actual cause, instead of timing out on a subscription that
		// will never deliver anything.
		w.err = fmt.Errorf("unexpected message from the events endpoint: %s", string(msg))
	}

	close(w.arrived)
	w.arrived = make(chan struct{})
	return nil
}

func (w *eventConsumer) Stop() {
	w.reader.Stop()
}

// mark returns the position of the next event that will arrive. Grab a
// marker before performing an action, then pass it to waitFor to only
// consider events generated after that point.
func (w *eventConsumer) mark() int {
	w.mux.Lock()
	defer w.mux.Unlock()
	return len(w.events)
}

// waitFor runs match against every event received since the given marker
// and against every new event, in arrival order, until match returns true
// or the timeout expires. Matchers are only ever called from the caller's
// goroutine, so they may safely mutate captured state.
func (w *eventConsumer) waitFor(what string, since int, timeout time.Duration, match func(dbEvent) bool) (dbEvent, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	next := since
	for {
		w.mux.Lock()
		pending := w.events[next:]
		next = len(w.events)
		consumerErr := w.err
		arrived := w.arrived
		w.mux.Unlock()

		// Drain events received before any error; they are still valid.
		for _, event := range pending {
			if match(event) {
				return event, nil
			}
		}

		if consumerErr != nil {
			return dbEvent{}, fmt.Errorf("waiting for %s: %w", what, consumerErr)
		}

		select {
		case <-arrived:
		case <-w.reader.Done():
			// The connection is gone and no more events can arrive. Give
			// the handler a chance to record a server-sent error message
			// first, then report whatever we know.
			w.mux.Lock()
			consumerErr = w.err
			hasPending := next < len(w.events)
			w.mux.Unlock()
			if hasPending {
				continue
			}
			if consumerErr == nil {
				consumerErr = fmt.Errorf("events websocket connection closed")
			}
			return dbEvent{}, fmt.Errorf("waiting for %s: %w", what, consumerErr)
		case <-deadline.C:
			return dbEvent{}, fmt.Errorf("timed out waiting for %s", what)
		}
	}
}
