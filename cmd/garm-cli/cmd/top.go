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
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math/rand"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	apiClientController "github.com/cloudbase/garm/client/controller_info"
	apiClientInstances "github.com/cloudbase/garm/client/instances"
	apiClientJobs "github.com/cloudbase/garm/client/jobs"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	wsEvents "github.com/cloudbase/garm/workers/websocket/events"
	"github.com/cloudbase/garm/workers/websocket/metrics"
)

// changePayload mirrors database/common.ChangePayload. It is redeclared here
// (rather than imported) because ChangePayload carries the payload as an
// opaque interface{}; the dashboard needs the raw bytes so it can decode them
// based on the entity type.
type changePayload struct {
	EntityType dbCommon.DatabaseEntityType `json:"entity-type"`
	Operation  dbCommon.OperationType      `json:"operation"`
	Payload    json.RawMessage             `json:"payload"`
}

// topEventTypes are the entity types the dashboard subscribes to.
var topEventTypes = []dbCommon.DatabaseEntityType{
	dbCommon.RepositoryEntityType,
	dbCommon.OrganizationEntityType,
	dbCommon.EnterpriseEntityType,
	dbCommon.ForgeInstanceEntityType,
	dbCommon.PoolEntityType,
	dbCommon.ScaleSetEntityType,
	dbCommon.InstanceEntityType,
	dbCommon.JobEntityType,
	dbCommon.ControllerEntityType,
}

// connState describes the dashboard's connection to the GARM server.
type connState int

const (
	connConnecting connState = iota
	connConnected
	connReconnecting
)

// topState holds the mutable state updated by WebSocket handlers.
type topState struct {
	mu           sync.Mutex
	instances    map[string]params.Instance // keyed by instance ID
	jobs         map[int64]params.Job       // keyed by job ID
	lastSnapshot *metrics.MetricsSnapshot   // latest metrics snapshot, patched by events
	controller   *params.ControllerInfo

	conn       connState
	connDetail string    // e.g. "retrying in 4s"
	lastUpdate time.Time // wall clock of the last processed frame

	// Instances and jobs are event-sourced: unlike entities/pools they are
	// not part of the periodic snapshot, so a REST seed reconciles them (on
	// connect and periodically). Events that arrive while a seed is in
	// flight are fresher than the seed response; touched* records the keys
	// they modified or deleted so reconcile leaves those alone.
	seeding          bool
	touchedInstances map[string]struct{}
	touchedJobs      map[int64]struct{}
}

func newTopState() *topState {
	return &topState{
		instances: make(map[string]params.Instance),
		jobs:      make(map[int64]params.Job),
	}
}

func (s *topState) setConn(state connState, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = state
	s.connDetail = detail
}

// beginSeed marks the start of a REST seed. Until reconcile (or abortSeed)
// is called, applyChange records which keys events have modified.
func (s *topState) beginSeed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seeding = true
	s.touchedInstances = make(map[string]struct{})
	s.touchedJobs = make(map[int64]struct{})
}

func (s *topState) abortSeed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seeding = false
	s.touchedInstances = nil
	s.touchedJobs = nil
}

// reconcile folds a seed response into the state: entries the seed lists are
// upserted, entries it does not list are removed. Keys touched by events
// since beginSeed are skipped entirely — the event stream is fresher than
// the REST response.
func (s *topState) reconcile(instances []params.Instance, jobs []params.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seenInstances := make(map[string]struct{}, len(instances))
	for _, inst := range instances {
		if inst.ID == "" {
			continue
		}
		seenInstances[inst.ID] = struct{}{}
		if _, touched := s.touchedInstances[inst.ID]; !touched {
			s.instances[inst.ID] = inst
		}
	}
	for id := range s.instances {
		if _, seen := seenInstances[id]; seen {
			continue
		}
		if _, touched := s.touchedInstances[id]; touched {
			continue
		}
		delete(s.instances, id)
	}

	seenJobs := make(map[int64]struct{}, len(jobs))
	for _, j := range jobs {
		if j.ID == 0 {
			continue
		}
		seenJobs[j.ID] = struct{}{}
		if _, touched := s.touchedJobs[j.ID]; !touched {
			s.jobs[j.ID] = j
		}
	}
	for id := range s.jobs {
		if _, seen := seenJobs[id]; seen {
			continue
		}
		if _, touched := s.touchedJobs[id]; touched {
			continue
		}
		delete(s.jobs, id)
	}

	s.seeding = false
	s.touchedInstances = nil
	s.touchedJobs = nil
	s.lastUpdate = time.Now()
}

func (s *topState) setSnapshot(snap *metrics.MetricsSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSnapshot = snap
	s.lastUpdate = time.Now()
}

func (s *topState) setController(info params.ControllerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.controller = &info
}

// renderData is a self-contained copy of the dashboard state, safe to render
// without holding the state lock.
type renderData struct {
	haveSnapshot bool
	entities     []metrics.MetricsEntity
	pools        []metrics.MetricsPool
	scaleSets    []metrics.MetricsScaleSet
	instances    []params.Instance
	jobs         []params.Job
	controller   *params.ControllerInfo
	conn         connState
	connDetail   string
	lastUpdate   time.Time
}

// copyData snapshots the state for rendering. The slices are cloned because
// the event handlers patch the snapshot in place while rendering happens on
// the UI goroutine.
func (s *topState) copyData() renderData {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := renderData{
		haveSnapshot: s.lastSnapshot != nil,
		instances:    slices.Collect(maps.Values(s.instances)),
		jobs:         slices.Collect(maps.Values(s.jobs)),
		conn:         s.conn,
		connDetail:   s.connDetail,
		lastUpdate:   s.lastUpdate,
	}
	if s.lastSnapshot != nil {
		data.entities = slices.Clone(s.lastSnapshot.Entities)
		data.pools = slices.Clone(s.lastSnapshot.Pools)
		data.scaleSets = slices.Clone(s.lastSnapshot.ScaleSets)
	}
	if s.controller != nil {
		ctrl := *s.controller
		data.controller = &ctrl
	}
	return data
}

// applyEvent returns the list with item upserted (matched element replaced or
// appended) or, when isDelete is set, with matching elements removed.
func applyEvent[E any](list []E, item E, match func(E) bool, isDelete bool) []E {
	if isDelete {
		return slices.DeleteFunc(list, match)
	}
	if i := slices.IndexFunc(list, match); i >= 0 {
		list[i] = item
		return list
	}
	return append(list, item)
}

// applyChange folds a single WebSocket event into the state. Pool, scale set
// and entity events patch the latest metrics snapshot; until the first
// snapshot arrives they are dropped, as the snapshot will include them anyway.
func (s *topState) applyChange(cp changePayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUpdate = time.Now()

	isDelete := cp.Operation == dbCommon.DeleteOperation
	switch cp.EntityType {
	case dbCommon.InstanceEntityType:
		var inst params.Instance
		if err := json.Unmarshal(cp.Payload, &inst); err != nil || inst.ID == "" {
			return
		}
		if s.seeding {
			s.touchedInstances[inst.ID] = struct{}{}
		}
		if isDelete {
			delete(s.instances, inst.ID)
		} else {
			s.instances[inst.ID] = inst
		}
	case dbCommon.JobEntityType:
		var job params.Job
		if err := json.Unmarshal(cp.Payload, &job); err != nil || job.ID == 0 {
			return
		}
		if s.seeding {
			s.touchedJobs[job.ID] = struct{}{}
		}
		if isDelete {
			delete(s.jobs, job.ID)
		} else {
			s.jobs[job.ID] = job
		}
	case dbCommon.ControllerEntityType:
		var info params.ControllerInfo
		if err := json.Unmarshal(cp.Payload, &info); err != nil {
			return
		}
		s.controller = &info
	case dbCommon.PoolEntityType:
		if s.lastSnapshot == nil {
			return
		}
		var pool params.Pool
		if err := json.Unmarshal(cp.Payload, &pool); err != nil || pool.ID == "" {
			return
		}
		item := poolToMetrics(pool)
		// Pool events carry the forge instance only by ID; keep the name
		// resolved by the snapshot.
		if i := slices.IndexFunc(s.lastSnapshot.Pools, func(p metrics.MetricsPool) bool { return p.ID == pool.ID }); i >= 0 {
			item.ForgeInstanceName = s.lastSnapshot.Pools[i].ForgeInstanceName
		}
		s.lastSnapshot.Pools = applyEvent(s.lastSnapshot.Pools, item,
			func(p metrics.MetricsPool) bool { return p.ID == pool.ID }, isDelete)
	case dbCommon.ScaleSetEntityType:
		var ss params.ScaleSet
		if err := json.Unmarshal(cp.Payload, &ss); err != nil || ss.ID == 0 {
			return
		}
		if s.lastSnapshot == nil {
			return
		}
		s.lastSnapshot.ScaleSets = applyEvent(s.lastSnapshot.ScaleSets, scaleSetToMetrics(ss),
			func(m metrics.MetricsScaleSet) bool { return m.ID == ss.ID }, isDelete)
	case dbCommon.RepositoryEntityType, dbCommon.OrganizationEntityType, dbCommon.EnterpriseEntityType,
		dbCommon.ForgeInstanceEntityType:
		if s.lastSnapshot == nil {
			return
		}
		entity := entityEventToMetrics(cp.EntityType, cp.Payload)
		if entity.ID == "" {
			return
		}
		match := func(e metrics.MetricsEntity) bool { return e.ID == entity.ID }
		// Entity events do not carry pool/scale set counts; preserve the
		// counts from the snapshot.
		if i := slices.IndexFunc(s.lastSnapshot.Entities, match); i >= 0 {
			entity.PoolCount = s.lastSnapshot.Entities[i].PoolCount
			entity.ScaleSetCount = s.lastSnapshot.Entities[i].ScaleSetCount
		}
		s.lastSnapshot.Entities = applyEvent(s.lastSnapshot.Entities, entity, match, isDelete)
	}
}

// seedTop fetches the instance, job and controller state over REST and
// reconciles it into the state. Events applied between beginSeed and
// reconcile win over the REST response.
func seedTop(state *topState) error {
	state.beginSeed()
	instResp, err := apiCli.Instances.ListInstances(apiClientInstances.NewListInstancesParams(), authToken)
	if err != nil {
		state.abortSeed()
		return fmt.Errorf("failed to list instances: %w", err)
	}
	jobsResp, err := apiCli.Jobs.ListJobs(apiClientJobs.NewListJobsParams(), authToken)
	if err != nil {
		state.abortSeed()
		return fmt.Errorf("failed to list jobs: %w", err)
	}
	state.reconcile(instResp.Payload, jobsResp.Payload)

	// Controller info is nice-to-have header decoration; it also arrives
	// via controller events, so a failure here is not fatal.
	if ctrlResp, err := apiCli.ControllerInfo.ControllerInfo(apiClientController.NewControllerInfoParams(), authToken); err == nil {
		state.setController(ctrlResp.Payload)
	}
	return nil
}

// isAuthError reports whether a WebSocket dial error looks like an expired
// or rejected token. The reader formats the HTTP status into the error text,
// which is all we have to go on.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized")
}

const topReseedInterval = 60 * time.Second

// runTopStreams maintains the WebSocket connections for the dashboard,
// reconnecting with backoff when they drop. It blocks until ctx is canceled
// or an authentication error occurs (returned as a fatal error). notify
// requests a UI repaint.
func runTopStreams(ctx context.Context, state *topState, notify func()) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		state.setConn(connConnecting, "")
		notify()

		err := runTopStreamsOnce(ctx, state, func() {
			backoff = time.Second
			state.setConn(connConnected, "")
			notify()
		})
		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutting down; stream errors are expected
		}
		if isAuthError(err) {
			return fmt.Errorf("session rejected by the server (token expired?): run `garm-cli profile login` and start top again")
		}

		detail := fmt.Sprintf("retrying in %s", backoff.Round(time.Second))
		if err != nil {
			detail = fmt.Sprintf("%s — retrying in %s", connErrorSummary(err), backoff.Round(time.Second))
		}
		state.setConn(connReconnecting, detail)
		notify()

		// Full backoff plus up to 25% jitter, so a fleet of dashboards does
		// not stampede a restarting server.
		wait := backoff + time.Duration(rand.Int63n(int64(backoff/4+1))) // #nosec G404 - jitter, not crypto
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// connErrorSummary trims a websocket dial/read error down to something that
// fits in the dashboard header.
func connErrorSummary(err error) string {
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx >= 0 && idx+2 < len(msg) {
		msg = msg[idx+2:]
	}
	const maxLen = 60
	if len(msg) > maxLen {
		msg = msg[:maxLen]
	}
	return msg
}

// runTopStreamsOnce establishes the events and metrics streams, seeds the
// state and waits for either stream to die, reseeding periodically. It
// subscribes to events before seeding so no change is lost in between.
// onConnected is called once both streams are up and the seed succeeded.
func runTopStreamsOnce(ctx context.Context, state *topState, onConnected func()) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventsHandler := func(_ int, msg []byte) error {
		var cp changePayload
		if err := json.Unmarshal(msg, &cp); err != nil {
			return nil //nolint:nilerr // tolerate malformed frames
		}
		state.applyChange(cp)
		return nil
	}
	eventsReader, err := garmWs.NewReader(streamCtx, mgr.BaseURL, "/api/v1/ws/events", mgr.Token, eventsHandler)
	if err != nil {
		return fmt.Errorf("failed to connect to events WebSocket: %w", err)
	}
	if err := eventsReader.Start(); err != nil {
		return fmt.Errorf("failed to connect to events WebSocket: %w", err)
	}
	defer eventsReader.Stop()

	filters := make([]wsEvents.Filter, 0, len(topEventTypes))
	for _, entityType := range topEventTypes {
		filters = append(filters, wsEvents.Filter{EntityType: entityType})
	}
	filterMsg, err := json.Marshal(wsEvents.Options{Filters: filters})
	if err != nil {
		return fmt.Errorf("failed to encode events filter: %w", err)
	}
	if err := eventsReader.WriteMessage(websocket.TextMessage, filterMsg); err != nil {
		return fmt.Errorf("failed to send events filter: %w", err)
	}

	metricsHandler := func(_ int, msg []byte) error {
		var snap metrics.MetricsSnapshot
		if err := json.Unmarshal(msg, &snap); err != nil {
			return nil //nolint:nilerr // tolerate malformed frames
		}
		state.setSnapshot(&snap)
		return nil
	}
	metricsReader, err := garmWs.NewReader(streamCtx, mgr.BaseURL, "/api/v1/ws/metrics", mgr.Token, metricsHandler)
	if err != nil {
		return fmt.Errorf("failed to connect to metrics WebSocket: %w", err)
	}
	if err := metricsReader.Start(); err != nil {
		return fmt.Errorf("failed to connect to metrics WebSocket: %w", err)
	}
	defer metricsReader.Stop()

	if err := seedTop(state); err != nil {
		return err
	}
	onConnected()

	reseed := time.NewTicker(topReseedInterval)
	defer reseed.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-eventsReader.Done():
			return fmt.Errorf("events stream closed")
		case <-metricsReader.Done():
			return fmt.Errorf("metrics stream closed")
		case <-reseed.C:
			// Periodic reconciliation heals anything a missed event left
			// behind. Errors are transient by definition here: the streams
			// are still up, so just try again next tick.
			_ = seedTop(state)
		}
	}
}

var topCmd = &cobra.Command{
	Use:          "top",
	SilenceUsage: true,
	Short:        "Live dashboard of GARM metrics",
	Long: `Interactive terminal UI showing live GARM state, fed by the events
WebSocket and 5-second metrics snapshots. The connection is re-established
automatically if it drops.

Keys: Tab/1-4 switch panels, Enter shows details, / filters the focused
panel, z zooms, l tails the server log, d/e run actions, ? shows help,
q quits.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		state := newTopState()
		app := tview.NewApplication()
		ui := newTopUI(app, state)
		ui.ctx = ctx // scopes background helpers like the log stream

		// Rendering is coalesced: handlers mark the state dirty and a single
		// goroutine repaints, so event storms cannot back up the WebSocket
		// readers behind the UI loop. The ticker keeps clocks and AGE
		// columns moving between frames.
		renderRequests := make(chan struct{}, 1)
		notify := func() {
			select {
			case renderRequests <- struct{}{}:
			default:
			}
		}
		ui.notify = notify

		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-renderRequests:
				case <-ticker.C:
				}
				app.QueueUpdateDraw(func() { ui.render(state.copyData()) })
				// Rate-limit repaints; anything that arrives in between is
				// absorbed by the dirty flag.
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}()

		var fatalMu sync.Mutex
		var fatalErr error
		go func() {
			err := runTopStreams(ctx, state, notify)
			fatalMu.Lock()
			fatalErr = err
			fatalMu.Unlock()
			if err != nil {
				app.Stop()
			}
		}()

		go func() {
			<-ctx.Done()
			app.Stop()
		}()

		ui.render(state.copyData())
		if err := app.SetRoot(ui.pages, true).EnableMouse(true).Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		cancel()

		fatalMu.Lock()
		defer fatalMu.Unlock()
		return fatalErr
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}
