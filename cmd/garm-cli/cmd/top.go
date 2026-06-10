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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/websocket"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	apiClientInstances "github.com/cloudbase/garm/client/instances"
	apiClientJobs "github.com/cloudbase/garm/client/jobs"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/workers/websocket/metrics"
)

// changePayload mirrors database/common.ChangePayload, with the payload kept
// raw so it can be decoded based on the entity type.
type changePayload struct {
	EntityType dbCommon.DatabaseEntityType `json:"entity-type"`
	Operation  dbCommon.OperationType      `json:"operation"`
	Payload    json.RawMessage             `json:"payload"`
}

// eventFilter and eventOptions mirror the filter options accepted by the
// events WebSocket endpoint (workers/websocket/events). An empty operations
// list subscribes to all operations for that entity type.
type eventFilter struct {
	EntityType dbCommon.DatabaseEntityType `json:"entity-type"`
	Operations []dbCommon.OperationType    `json:"operations,omitempty"`
}

type eventOptions struct {
	Filters []eventFilter `json:"filters"`
}

// topEventTypes are the entity types the dashboard subscribes to.
var topEventTypes = []dbCommon.DatabaseEntityType{
	dbCommon.RepositoryEntityType,
	dbCommon.OrganizationEntityType,
	dbCommon.EnterpriseEntityType,
	dbCommon.PoolEntityType,
	dbCommon.ScaleSetEntityType,
	dbCommon.InstanceEntityType,
	dbCommon.JobEntityType,
}

// topState holds the mutable state updated by WebSocket handlers.
type topState struct {
	mu           sync.Mutex
	instances    map[string]params.Instance // keyed by instance ID
	jobs         map[int64]params.Job       // keyed by job ID
	lastSnapshot *metrics.MetricsSnapshot   // latest metrics snapshot, patched by events
}

func newTopState() *topState {
	return &topState{
		instances: make(map[string]params.Instance),
		jobs:      make(map[int64]params.Job),
	}
}

// seed populates the initial instance and job lists from the API. The metrics
// snapshot arrives via WebSocket shortly after connecting.
func (s *topState) seed() error {
	instResp, err := apiCli.Instances.ListInstances(apiClientInstances.NewListInstancesParams(), authToken)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}
	jobsResp, err := apiCli.Jobs.ListJobs(apiClientJobs.NewListJobsParams(), authToken)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inst := range instResp.Payload {
		if inst.ID != "" {
			s.instances[inst.ID] = inst
		}
	}
	for _, j := range jobsResp.Payload {
		if j.ID != 0 {
			s.jobs[j.ID] = j
		}
	}
	return nil
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
	}
	if s.lastSnapshot != nil {
		data.entities = slices.Clone(s.lastSnapshot.Entities)
		data.pools = slices.Clone(s.lastSnapshot.Pools)
		data.scaleSets = slices.Clone(s.lastSnapshot.ScaleSets)
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

	isDelete := cp.Operation == dbCommon.DeleteOperation
	switch cp.EntityType {
	case dbCommon.InstanceEntityType:
		var inst params.Instance
		if err := json.Unmarshal(cp.Payload, &inst); err != nil || inst.ID == "" {
			return
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
		if isDelete {
			delete(s.jobs, job.ID)
		} else {
			s.jobs[job.ID] = job
		}
	case dbCommon.PoolEntityType:
		if s.lastSnapshot == nil {
			return
		}
		var pool params.Pool
		if err := json.Unmarshal(cp.Payload, &pool); err != nil || pool.ID == "" {
			return
		}
		s.lastSnapshot.Pools = applyEvent(s.lastSnapshot.Pools, poolToMetrics(pool),
			func(p metrics.MetricsPool) bool { return p.ID == pool.ID }, isDelete)
	case dbCommon.ScaleSetEntityType:
		if s.lastSnapshot == nil {
			return
		}
		var ss params.ScaleSet
		if err := json.Unmarshal(cp.Payload, &ss); err != nil || ss.ID == 0 {
			return
		}
		s.lastSnapshot.ScaleSets = applyEvent(s.lastSnapshot.ScaleSets, scaleSetToMetrics(ss),
			func(m metrics.MetricsScaleSet) bool { return m.ID == ss.ID }, isDelete)
	case dbCommon.RepositoryEntityType, dbCommon.OrganizationEntityType, dbCommon.EnterpriseEntityType:
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

// topUI holds the dashboard widgets.
type topUI struct {
	header    *tview.TextView
	summary   *tview.TextView
	entities  *tview.Table
	pools     *tview.Table
	instances *tview.Table
	jobs      *tview.Table
	root      tview.Primitive
}

func newTopUI(app *tview.Application) *topUI {
	// Explicit dark color scheme so the TUI looks consistent regardless of
	// light/dark terminal theme.
	bgColor := tcell.Color235 // #262626 - dark gray
	borderColor := tcell.ColorLightGray

	tview.Styles.PrimitiveBackgroundColor = bgColor
	tview.Styles.ContrastBackgroundColor = bgColor
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.BorderColor = borderColor
	tview.Styles.TitleColor = tcell.ColorWhite

	newPanelTable := func(title string) *tview.Table {
		table := tview.NewTable().
			SetBorders(false).
			SetSelectable(true, false).
			SetFixed(1, 0)
		table.SetBorder(true).
			SetTitle(title).
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)
		return table
	}

	ui := &topUI{
		header:    tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		summary:   tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		entities:  newPanelTable(" Entities "),
		pools:     newPanelTable(" Pools & Scale Sets "),
		instances: newPanelTable(" Instances "),
		jobs:      newPanelTable(" Jobs "),
	}
	ui.header.SetBackgroundColor(bgColor)
	ui.summary.SetBorder(true).
		SetTitle(" Summary ").
		SetTitleAlign(tview.AlignLeft).
		SetBackgroundColor(bgColor)

	footer := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	footer.SetBackgroundColor(bgColor)
	footer.SetText("[yellow]Tab/Shift+Tab[white]: switch panel  [yellow]↑↓[white]: scroll  [yellow]q[white]: quit")

	// Two-column layout: left (entities + pools) | right (instances + jobs)
	leftCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.entities, 0, 1, true).
		AddItem(ui.pools, 0, 1, false)

	rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.instances, 0, 1, false).
		AddItem(ui.jobs, 0, 1, false)

	columns := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftCol, 0, 1, true).
		AddItem(rightCol, 0, 1, false)

	ui.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.header, 1, 0, false).
		AddItem(ui.summary, 5, 0, false).
		AddItem(columns, 0, 1, true).
		AddItem(footer, 1, 0, false)

	// Panel focus cycling
	panels := []*tview.Table{ui.entities, ui.pools, ui.instances, ui.jobs}
	focusIndex := 0
	focusPanel := func(idx int) {
		focusIndex = idx
		for i, p := range panels {
			color := borderColor
			if i == idx {
				color = tcell.ColorDodgerBlue
			}
			p.SetBorderColor(color)
		}
		app.SetFocus(panels[idx])
	}
	focusPanel(0)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'q' || event.Rune() == 'Q':
			app.Stop()
			return nil
		case event.Key() == tcell.KeyTab:
			focusPanel((focusIndex + 1) % len(panels))
			return nil
		case event.Key() == tcell.KeyBacktab:
			focusPanel((focusIndex - 1 + len(panels)) % len(panels))
			return nil
		}
		return event
	})

	return ui
}

// render redraws the whole dashboard. It must run on the UI goroutine (either
// before the application starts or via app.QueueUpdateDraw).
func (ui *topUI) render(data renderData) {
	status := "connected"
	if !data.haveSnapshot {
		status = "connecting"
	}
	updateHeader(ui.header, mgr.BaseURL, status)

	if data.haveSnapshot {
		renderSummary(ui.summary, data)
	} else {
		ui.summary.SetText(" [gray]Waiting for the first metrics snapshot...")
	}
	renderEntitiesTable(ui.entities, data.entities)
	renderPoolsTable(ui.pools, data.pools, data.scaleSets)
	renderInstancesTable(ui.instances, data.instances)
	renderJobsTable(ui.jobs, data.jobs)
}

var topCmd = &cobra.Command{
	Use:          "top",
	SilenceUsage: true,
	Short:        "Live dashboard of GARM metrics",
	Long:         `Interactive terminal UI showing live GARM metrics, refreshed every 5 seconds via WebSocket.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()

		state := newTopState()
		if err := state.seed(); err != nil {
			return err
		}

		app := tview.NewApplication()
		ui := newTopUI(app)
		renderAll := func() { ui.render(state.copyData()) }
		// Initial paint so the seeded data is visible before the first
		// metrics snapshot arrives.
		renderAll()

		metricsHandler := func(_ int, msg []byte) error {
			var snap metrics.MetricsSnapshot
			if err := json.Unmarshal(msg, &snap); err != nil {
				return nil // tolerate malformed frames
			}
			state.mu.Lock()
			state.lastSnapshot = &snap
			state.mu.Unlock()
			app.QueueUpdateDraw(renderAll)
			return nil
		}
		metricsReader, err := garmWs.NewReader(ctx, mgr.BaseURL, "/api/v1/ws/metrics", mgr.Token, metricsHandler)
		if err != nil {
			return fmt.Errorf("failed to connect to metrics WebSocket: %w", err)
		}
		if err := metricsReader.Start(); err != nil {
			return fmt.Errorf("failed to start metrics reader: %w", err)
		}
		defer metricsReader.Stop()

		eventsHandler := func(_ int, msg []byte) error {
			var cp changePayload
			if err := json.Unmarshal(msg, &cp); err != nil {
				return nil // tolerate malformed frames
			}
			state.applyChange(cp)
			app.QueueUpdateDraw(renderAll)
			return nil
		}
		eventsReader, err := garmWs.NewReader(ctx, mgr.BaseURL, "/api/v1/ws/events", mgr.Token, eventsHandler)
		if err != nil {
			return fmt.Errorf("failed to connect to events WebSocket: %w", err)
		}
		if err := eventsReader.Start(); err != nil {
			return fmt.Errorf("failed to start events reader: %w", err)
		}
		defer eventsReader.Stop()

		// Subscribe to all entity types relevant to the TUI.
		filters := make([]eventFilter, 0, len(topEventTypes))
		for _, entityType := range topEventTypes {
			filters = append(filters, eventFilter{EntityType: entityType})
		}
		filterMsg, err := json.Marshal(eventOptions{Filters: filters})
		if err != nil {
			return fmt.Errorf("failed to encode events filter: %w", err)
		}
		if err := eventsReader.WriteMessage(websocket.TextMessage, filterMsg); err != nil {
			return fmt.Errorf("failed to send events filter: %w", err)
		}

		// Stop the TUI when either WebSocket dies or the context is canceled.
		go func() {
			select {
			case <-metricsReader.Done():
			case <-eventsReader.Done():
			case <-ctx.Done():
			}
			app.Stop()
		}()

		if err := app.SetRoot(ui.root, true).Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		// Distinguish a user-initiated quit from a dropped connection.
		if ctx.Err() == nil {
			select {
			case <-metricsReader.Done():
				return errors.New("connection to the GARM metrics stream was lost")
			case <-eventsReader.Done():
				return errors.New("connection to the GARM events stream was lost")
			default:
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func updateHeader(header *tview.TextView, baseURL, status string) {
	statusColor := "[green]"
	if status == "connecting" {
		statusColor = "[yellow]"
	}
	header.SetText(fmt.Sprintf(
		" [::b]GARM Top[::-]  │  %s  │  %s%s[white]  │  %s",
		baseURL, statusColor, status, time.Now().Format("15:04:05"),
	))
}

func renderSummary(view *tview.TextView, data renderData) {
	repos, orgs, ents := 0, 0, 0
	for _, e := range data.entities {
		switch dbCommon.DatabaseEntityType(e.Type) {
		case dbCommon.RepositoryEntityType:
			repos++
		case dbCommon.OrganizationEntityType:
			orgs++
		case dbCommon.EnterpriseEntityType:
			ents++
		}
	}

	// Runner status buckets from the metrics snapshot.
	buckets := map[string]int{}
	countRunners := func(statusCounts map[string]int) {
		for status, count := range statusCounts {
			cat, ok := runnerStatusCategory[params.RunnerStatus(status)]
			if !ok {
				cat = "other"
			}
			buckets[cat] += count
		}
	}
	for _, p := range data.pools {
		countRunners(p.RunnerStatusCounts)
	}
	for _, ss := range data.scaleSets {
		countRunners(ss.RunnerStatusCounts)
	}

	queuedCount, inProgressCount, completedCount := 0, 0, 0
	for _, j := range data.jobs {
		switch params.JobStatus(j.Status) {
		case params.JobStatusQueued:
			queuedCount++
		case params.JobStatusInProgress:
			inProgressCount++
		case params.JobStatusCompleted:
			completedCount++
		}
	}

	line1 := fmt.Sprintf(
		" [blue]Repos:[white] %d   [green]Orgs:[white] %d   [purple]Enterprises:[white] %d   [white]Pools:[white] %d   [white]Scale Sets:[white] %d   [white]Instances:[white] %d",
		repos, orgs, ents, len(data.pools), len(data.scaleSets), len(data.instances),
	)

	runnerLine := " "
	for _, bucket := range []struct {
		key, label, color string
	}{
		{"active", "Active", "green"},
		{"idle", "Idle", "blue"},
		{"pending", "Pending", "yellow"},
		{"offline", "Offline", "red"},
		{"other", "Other", "gray"},
	} {
		if count := buckets[bucket.key]; count > 0 {
			runnerLine += fmt.Sprintf("[%s]%s:[white] %d   ", bucket.color, bucket.label, count)
		}
	}
	if runnerLine == " " {
		runnerLine = " [gray]No runners"
	}

	jobLine := fmt.Sprintf(
		" [white]Jobs: [yellow]%d queued[white], [green]%d running[white], [gray]%d completed",
		queuedCount, inProgressCount, completedCount,
	)

	view.SetText(line1 + "\n" + runnerLine + "\n" + jobLine)
}

// setTableHeader sets the header row. Columns from rightAlignFrom on are
// right-aligned; pass a negative value to left-align everything.
func setTableHeader(table *tview.Table, headers []string, rightAlignFrom int) {
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		if rightAlignFrom >= 0 && i >= rightAlignFrom {
			cell.SetAlign(tview.AlignRight)
		}
		table.SetCell(0, i, cell)
	}
}

func setEmptyMessage(table *tview.Table, msg string) {
	table.SetCell(1, 0, tview.NewTableCell(msg).
		SetTextColor(tcell.ColorGray).SetExpansion(1))
}

// renderEntitiesTable renders the entities panel. It sorts the slice in place.
func renderEntitiesTable(table *tview.Table, entities []metrics.MetricsEntity) {
	table.Clear()
	setTableHeader(table, []string{"NAME", "TYPE", "ENDPOINT", "POOLS", "SCALESETS", "HEALTH"}, 3)

	if len(entities) == 0 {
		setEmptyMessage(table, "No entities configured")
		return
	}

	slices.SortFunc(entities, func(a, b metrics.MetricsEntity) int {
		return cmp.Or(
			cmp.Compare(b.PoolCount+b.ScaleSetCount, a.PoolCount+a.ScaleSetCount),
			cmp.Compare(a.Name, b.Name),
		)
	})

	for row, e := range entities {
		r := row + 1
		typeLabel := e.Type
		typeColor := tcell.ColorWhite
		switch dbCommon.DatabaseEntityType(e.Type) {
		case dbCommon.RepositoryEntityType:
			typeLabel = "repo"
			typeColor = tcell.ColorDodgerBlue
		case dbCommon.OrganizationEntityType:
			typeLabel = "org"
			typeColor = tcell.ColorGreen
		case dbCommon.EnterpriseEntityType:
			typeLabel = "ent"
			typeColor = tcell.ColorMediumPurple
		}

		healthColor, healthStr := tcell.ColorGreen, "✓"
		if !e.Healthy {
			healthColor, healthStr = tcell.ColorRed, "✗"
		}

		table.SetCell(r, 0, tview.NewTableCell(e.Name).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(typeLabel).SetTextColor(typeColor).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(e.Endpoint).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d", e.PoolCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d", e.ScaleSetCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(healthStr).SetTextColor(healthColor).SetAlign(tview.AlignRight).SetExpansion(1))
	}
}

// renderCapacityRow renders one pool or scale set row.
func renderCapacityRow(table *tview.Table, row int, name, provider, osType string, current, maxRunners int, enabled bool) {
	utilization := 0
	if maxRunners > 0 {
		utilization = current * 100 / maxRunners
	}
	capColor := tcell.ColorGreen
	switch {
	case utilization >= 90:
		capColor = tcell.ColorRed
	case utilization >= 70:
		capColor = tcell.ColorYellow
	}

	status, statusColor, nameColor := "enabled", tcell.ColorGreen, tcell.ColorWhite
	if !enabled {
		status, statusColor, nameColor = "disabled", tcell.ColorGray, tcell.ColorGray
	}

	table.SetCell(row, 0, tview.NewTableCell(name).SetTextColor(nameColor).SetExpansion(1))
	table.SetCell(row, 1, tview.NewTableCell(provider).SetExpansion(1))
	table.SetCell(row, 2, tview.NewTableCell(osType).SetExpansion(1))
	table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d/%d", current, maxRunners)).SetAlign(tview.AlignRight).SetExpansion(1))
	table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%d%%", utilization)).SetTextColor(capColor).SetAlign(tview.AlignRight).SetExpansion(1))
	table.SetCell(row, 5, tview.NewTableCell(status).SetTextColor(statusColor).SetAlign(tview.AlignRight).SetExpansion(1))
}

// renderPoolsTable renders the pools and scale sets panel. It sorts the
// slices in place.
func renderPoolsTable(table *tview.Table, pools []metrics.MetricsPool, scaleSets []metrics.MetricsScaleSet) {
	table.Clear()
	setTableHeader(table, []string{"NAME", "PROVIDER", "OS", "RUNNERS", "CAP", "STATUS"}, 3)

	if len(pools) == 0 && len(scaleSets) == 0 {
		setEmptyMessage(table, "No pools or scale sets configured")
		return
	}

	// Enabled first, then by runner count, with a stable ID/name tiebreaker
	// so rows do not jump around between refreshes.
	slices.SortFunc(pools, func(a, b metrics.MetricsPool) int {
		if a.Enabled != b.Enabled {
			if a.Enabled {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(sumCounts(b.RunnerCounts), sumCounts(a.RunnerCounts)),
			cmp.Compare(a.ID, b.ID),
		)
	})
	slices.SortFunc(scaleSets, func(a, b metrics.MetricsScaleSet) int {
		if a.Enabled != b.Enabled {
			if a.Enabled {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(sumCounts(b.RunnerCounts), sumCounts(a.RunnerCounts)),
			cmp.Compare(a.ID, b.ID),
		)
	})

	row := 1
	for _, p := range pools {
		renderCapacityRow(table, row, topPoolDisplayName(p), p.ProviderName, p.OSType,
			sumCounts(p.RunnerCounts), int(p.MaxRunners), p.Enabled)
		row++
	}
	for _, ss := range scaleSets {
		name := ss.Name
		if name == "" {
			name = fmt.Sprintf("scaleset-%d", ss.ID)
		}
		renderCapacityRow(table, row, name, ss.ProviderName, ss.OSType,
			sumCounts(ss.RunnerCounts), int(ss.MaxRunners), ss.Enabled)
		row++
	}
}

// renderInstancesTable renders the instances panel. It sorts the slice in
// place.
func renderInstancesTable(table *tview.Table, instances []params.Instance) {
	table.Clear()
	setTableHeader(table, []string{"NAME", "STATUS", "RUNNER", "PROVIDER", "OS", "POOL/SS", "AGE"}, -1)

	if len(instances) == 0 {
		setEmptyMessage(table, "No instances")
		return
	}

	// Sort: running first, then by creation time desc, then by name so rows
	// do not jump around between refreshes.
	slices.SortFunc(instances, func(a, b params.Instance) int {
		return cmp.Or(
			cmp.Compare(instanceStatusPriorities[a.Status], instanceStatusPriorities[b.Status]),
			b.CreatedAt.Compare(a.CreatedAt),
			cmp.Compare(a.Name, b.Name),
		)
	})

	for row, inst := range instances {
		r := row + 1

		runnerStr := string(inst.RunnerStatus)
		runnerColor := runnerStatusColors[inst.RunnerStatus]
		if runnerStr == "" {
			runnerStr = "-"
			runnerColor = tcell.ColorGray
		}

		poolRef := "-"
		switch {
		case inst.ScaleSetID > 0:
			poolRef = fmt.Sprintf("ss-%d", inst.ScaleSetID)
		case inst.PoolID != "":
			poolRef = shortID(inst.PoolID, 8)
		}

		name := inst.Name
		if name == "" {
			name = shortID(inst.ID, 12)
		}

		table.SetCell(r, 0, tview.NewTableCell(name).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(string(inst.Status)).SetTextColor(instanceStatusColors[inst.Status]).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(runnerStr).SetTextColor(runnerColor).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(inst.ProviderName).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(string(inst.OSType)).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(poolRef).SetExpansion(1))
		table.SetCell(r, 6, tview.NewTableCell(formatDuration(time.Since(inst.CreatedAt))).SetExpansion(1))
	}
}

// renderJobsTable renders the jobs panel. It sorts the slice in place.
func renderJobsTable(table *tview.Table, jobs []params.Job) {
	table.Clear()
	setTableHeader(table, []string{"NAME", "STATUS", "REPO", "RUNNER", "LABELS", "AGE"}, -1)

	if len(jobs) == 0 {
		setEmptyMessage(table, "No jobs")
		return
	}

	// Sort: in_progress first, then queued, then completed; within group by
	// update time desc, with the ID as a stable tiebreaker.
	slices.SortFunc(jobs, func(a, b params.Job) int {
		return cmp.Or(
			cmp.Compare(jobStatusPriorities[a.Status], jobStatusPriorities[b.Status]),
			b.UpdatedAt.Compare(a.UpdatedAt),
			cmp.Compare(a.ID, b.ID),
		)
	})

	for row, job := range jobs {
		r := row + 1

		statusStr := job.Status
		statusColor := jobStatusColors[statusStr]
		if job.Conclusion != "" && params.JobStatus(job.Status) == params.JobStatusCompleted {
			statusStr = job.Conclusion
			statusColor = jobConclusionColors[job.Conclusion]
		}

		repoStr := ""
		if job.RepositoryOwner != "" && job.RepositoryName != "" {
			repoStr = job.RepositoryOwner + "/" + job.RepositoryName
		}

		runnerStr := job.RunnerName
		if runnerStr == "" {
			runnerStr = "-"
		}

		table.SetCell(r, 0, tview.NewTableCell(truncate(job.Name, 40)).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(repoStr).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(runnerStr).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(truncate(strings.Join(job.Labels, ","), 30)).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(formatDuration(time.Since(job.CreatedAt))).SetExpansion(1))
	}
}

// --- Helpers ---

func topPoolDisplayName(p metrics.MetricsPool) string {
	entityName := cmp.Or(p.RepoName, p.OrgName, p.EnterpriseName)
	if entityName != "" {
		return entityName + " / " + shortID(p.ID, 8)
	}
	return shortID(p.ID, 8)
}

func shortID(id string, maxLen int) string {
	if len(id) > maxLen {
		return id[:maxLen]
	}
	return id
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, v := range counts {
		total += v
	}
	return total
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// runnerStatusCategory maps runner statuses to summary bucket names.
var runnerStatusCategory = map[params.RunnerStatus]string{
	params.RunnerActive:     "active",
	params.RunnerIdle:       "idle",
	params.RunnerOnline:     "idle",
	params.RunnerOffline:    "offline",
	params.RunnerTerminated: "offline",
	params.RunnerFailed:     "offline",
	params.RunnerPending:    "pending",
	params.RunnerInstalling: "pending",
}

var instanceStatusPriorities = map[commonParams.InstanceStatus]int{
	commonParams.InstanceRunning:            0,
	commonParams.InstancePendingCreate:      1,
	commonParams.InstanceCreating:           1,
	commonParams.InstanceStopped:            2,
	commonParams.InstanceError:              3,
	commonParams.InstancePendingDelete:      3,
	commonParams.InstancePendingForceDelete: 3,
	commonParams.InstanceDeleting:           3,
}

var instanceStatusColors = map[commonParams.InstanceStatus]tcell.Color{
	commonParams.InstanceRunning:            tcell.ColorGreen,
	commonParams.InstancePendingCreate:      tcell.ColorYellow,
	commonParams.InstanceCreating:           tcell.ColorYellow,
	commonParams.InstanceStopped:            tcell.ColorGray,
	commonParams.InstanceError:              tcell.ColorRed,
	commonParams.InstancePendingDelete:      tcell.ColorOrangeRed,
	commonParams.InstancePendingForceDelete: tcell.ColorOrangeRed,
	commonParams.InstanceDeleting:           tcell.ColorOrangeRed,
}

var runnerStatusColors = map[params.RunnerStatus]tcell.Color{
	params.RunnerActive:     tcell.ColorGreen,
	params.RunnerIdle:       tcell.ColorDodgerBlue,
	params.RunnerOnline:     tcell.ColorDodgerBlue,
	params.RunnerOffline:    tcell.ColorRed,
	params.RunnerTerminated: tcell.ColorRed,
	params.RunnerFailed:     tcell.ColorRed,
	params.RunnerPending:    tcell.ColorYellow,
	params.RunnerInstalling: tcell.ColorYellow,
}

var jobStatusPriorities = map[string]int{
	string(params.JobStatusInProgress): 0,
	string(params.JobStatusQueued):     1,
	string(params.JobStatusCompleted):  2,
}

var jobStatusColors = map[string]tcell.Color{
	string(params.JobStatusInProgress): tcell.ColorGreen,
	string(params.JobStatusQueued):     tcell.ColorYellow,
	string(params.JobStatusCompleted):  tcell.ColorGray,
}

var jobConclusionColors = map[string]tcell.Color{
	"success":   tcell.ColorGreen,
	"failure":   tcell.ColorRed,
	"cancelled": tcell.ColorOrangeRed,
	"timed_out": tcell.ColorRed,
}

func poolToMetrics(p params.Pool) metrics.MetricsPool {
	return metrics.MetricsPool{
		ID:                 p.ID,
		ProviderName:       p.ProviderName,
		OSType:             string(p.OSType),
		MaxRunners:         p.MaxRunners,
		Enabled:            p.Enabled,
		RepoName:           p.RepoName,
		OrgName:            p.OrgName,
		EnterpriseName:     p.EnterpriseName,
		RunnerCounts:       map[string]int{},
		RunnerStatusCounts: map[string]int{},
	}
}

func scaleSetToMetrics(ss params.ScaleSet) metrics.MetricsScaleSet {
	return metrics.MetricsScaleSet{
		ID:                 ss.ID,
		Name:               ss.Name,
		ProviderName:       ss.ProviderName,
		OSType:             string(ss.OSType),
		MaxRunners:         ss.MaxRunners,
		Enabled:            ss.Enabled,
		RepoName:           ss.RepoName,
		OrgName:            ss.OrgName,
		EnterpriseName:     ss.EnterpriseName,
		RunnerCounts:       map[string]int{},
		RunnerStatusCounts: map[string]int{},
	}
}

func entityEventToMetrics(entityType dbCommon.DatabaseEntityType, payload json.RawMessage) metrics.MetricsEntity {
	switch entityType {
	case dbCommon.RepositoryEntityType:
		var r params.Repository
		if err := json.Unmarshal(payload, &r); err != nil || r.ID == "" {
			return metrics.MetricsEntity{}
		}
		name := r.Name
		if r.Owner != "" {
			name = r.Owner + "/" + r.Name
		}
		return metrics.MetricsEntity{
			ID:       r.ID,
			Name:     name,
			Type:     string(entityType),
			Endpoint: r.Endpoint.Name,
			Healthy:  r.PoolManagerStatus.IsRunning,
		}
	case dbCommon.OrganizationEntityType:
		var o params.Organization
		if err := json.Unmarshal(payload, &o); err != nil || o.ID == "" {
			return metrics.MetricsEntity{}
		}
		return metrics.MetricsEntity{
			ID:       o.ID,
			Name:     o.Name,
			Type:     string(entityType),
			Endpoint: o.Endpoint.Name,
			Healthy:  o.PoolManagerStatus.IsRunning,
		}
	case dbCommon.EnterpriseEntityType:
		var e params.Enterprise
		if err := json.Unmarshal(payload, &e); err != nil || e.ID == "" {
			return metrics.MetricsEntity{}
		}
		return metrics.MetricsEntity{
			ID:       e.ID,
			Name:     e.Name,
			Type:     string(entityType),
			Endpoint: e.Endpoint.Name,
			Healthy:  e.PoolManagerStatus.IsRunning,
		}
	}
	return metrics.MetricsEntity{}
}
