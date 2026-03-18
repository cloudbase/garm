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
	"os/signal"
	"sort"
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
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/workers/websocket/metrics"
)

const (
	jobQueued     = "queued"
	jobInProgress = "in_progress"
	jobCompleted  = "completed"

	opDelete = "delete"

	// Event entity type strings (as used in the events WebSocket).
	evtRepository   = "repository"
	evtOrganization = "organization"
)

// topState holds the mutable state updated by WebSocket handlers.
type topState struct {
	mu           sync.Mutex
	instances    map[string]params.Instance // keyed by instance ID
	jobs         map[int64]params.Job       // keyed by job ID
	lastSnapshot *metrics.MetricsSnapshot   // latest metrics snapshot, patched by events
}

// changePayload mirrors database/common.ChangePayload for JSON decoding.
type changePayload struct {
	EntityType string          `json:"entity-type"`
	Operation  string          `json:"operation"`
	Payload    json.RawMessage `json:"payload"`
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

		app := tview.NewApplication()
		state := &topState{
			instances: make(map[string]params.Instance),
			jobs:      make(map[int64]params.Job),
		}

		// --- Seed initial data from API ---

		if resp, err := apiCli.Instances.ListInstances(apiClientInstances.NewListInstancesParams(), authToken); err == nil {
			for _, inst := range resp.Payload {
				if inst.ID != "" {
					state.instances[inst.ID] = inst
				}
			}
		}
		if resp, err := apiCli.Jobs.ListJobs(apiClientJobs.NewListJobsParams(), authToken); err == nil {
			for _, j := range resp.Payload {
				if j.ID != 0 {
					state.jobs[j.ID] = j
				}
			}
		}

		// --- Build TUI layout ---

		// Explicit dark color scheme so the TUI looks consistent
		// regardless of light/dark terminal theme.
		bgColor := tcell.Color235 // #262626 - dark gray
		fgColor := tcell.ColorWhite
		borderColor := tcell.ColorLightGray

		tview.Styles.PrimitiveBackgroundColor = bgColor
		tview.Styles.ContrastBackgroundColor = bgColor
		tview.Styles.PrimaryTextColor = fgColor
		tview.Styles.BorderColor = borderColor
		tview.Styles.TitleColor = fgColor

		header := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignLeft)
		header.SetBorder(false).SetBackgroundColor(bgColor)

		summary := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignLeft)
		summary.SetBorder(true).
			SetTitle(" Summary ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)

		entitiesTable := tview.NewTable().
			SetBorders(false).
			SetSelectable(true, false).
			SetFixed(1, 0)
		entitiesTable.SetBorder(true).
			SetTitle(" Entities ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)

		poolsTable := tview.NewTable().
			SetBorders(false).
			SetSelectable(true, false).
			SetFixed(1, 0)
		poolsTable.SetBorder(true).
			SetTitle(" Pools & Scale Sets ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)

		instancesTable := tview.NewTable().
			SetBorders(false).
			SetSelectable(true, false).
			SetFixed(1, 0)
		instancesTable.SetBorder(true).
			SetTitle(" Instances ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)

		jobsTable := tview.NewTable().
			SetBorders(false).
			SetSelectable(true, false).
			SetFixed(1, 0)
		jobsTable.SetBorder(true).
			SetTitle(" Jobs ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(bgColor)

		footer := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		footer.SetBorder(false).SetBackgroundColor(bgColor)
		footer.SetText("[yellow]Tab[white]: switch panel  [yellow]↑↓[white]: scroll  [yellow]q[white]: quit")

		// Two-column layout: left (entities + pools) | right (instances + jobs)
		leftCol := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(entitiesTable, 0, 1, true).
			AddItem(poolsTable, 0, 1, false)

		rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(instancesTable, 0, 1, false).
			AddItem(jobsTable, 0, 1, false)

		columns := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(leftCol, 0, 1, true).
			AddItem(rightCol, 0, 1, false)

		layout := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(header, 1, 0, false).
			AddItem(summary, 5, 0, false).
			AddItem(columns, 0, 1, true).
			AddItem(footer, 1, 0, false)

		// Panel focus cycling
		panels := []tview.Primitive{entitiesTable, poolsTable, instancesTable, jobsTable}
		panelBorders := []*tview.Table{entitiesTable, poolsTable, instancesTable, jobsTable}
		focusIndex := 0

		setFocus := func(idx int) {
			for i, p := range panelBorders {
				if i == idx {
					p.SetBorderColor(tcell.ColorDodgerBlue)
				} else {
					p.SetBorderColor(tcell.ColorWhite)
				}
			}
			app.SetFocus(panels[idx])
		}
		setFocus(0)

		updateHeader(header, mgr.BaseURL, "connecting")

		// Keybindings
		app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch {
			case event.Rune() == 'q':
				app.Stop()
				return nil
			case event.Key() == tcell.KeyTab:
				focusIndex = (focusIndex + 1) % len(panels)
				setFocus(focusIndex)
				return nil
			case event.Key() == tcell.KeyBacktab:
				focusIndex = (focusIndex - 1 + len(panels)) % len(panels)
				setFocus(focusIndex)
				return nil
			}
			return event
		})

		// --- Metrics WebSocket (5s snapshots) ---

		metricsConnected := false

		// renderAll re-renders the full TUI using the latest snapshot and state.
		// Must be called via app.QueueUpdateDraw.
		renderAll := func() {
			state.mu.Lock()
			snap := state.lastSnapshot
			if snap == nil {
				state.mu.Unlock()
				return
			}
			instances := make([]params.Instance, 0, len(state.instances))
			for _, inst := range state.instances {
				instances = append(instances, inst)
			}
			jobs := make([]params.Job, 0, len(state.jobs))
			for _, j := range state.jobs {
				jobs = append(jobs, j)
			}
			state.mu.Unlock()

			updateHeader(header, mgr.BaseURL, "connected")
			renderSummary(summary, snap, len(instances), jobs)
			renderEntitiesTable(entitiesTable, snap.Entities)
			renderPoolsTable(poolsTable, snap.Pools, snap.ScaleSets)
			renderInstancesTable(instancesTable, instances)
			renderJobsTable(jobsTable, jobs)
		}

		metricsHandler := func(_ int, msg []byte) error {
			var snap metrics.MetricsSnapshot
			if err := json.Unmarshal(msg, &snap); err != nil {
				return nil
			}

			state.mu.Lock()
			state.lastSnapshot = &snap
			state.mu.Unlock()

			metricsConnected = true
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

		// --- Events WebSocket (all entity types) ---

		eventsHandler := func(_ int, msg []byte) error {
			var cp changePayload
			if err := json.Unmarshal(msg, &cp); err != nil {
				return nil
			}

			state.mu.Lock()
			switch cp.EntityType {
			case "instance":
				if cp.Operation == opDelete {
					var inst params.Instance
					if err := json.Unmarshal(cp.Payload, &inst); err == nil && inst.ID != "" {
						delete(state.instances, inst.ID)
					}
				} else {
					var inst params.Instance
					if err := json.Unmarshal(cp.Payload, &inst); err == nil && inst.ID != "" {
						state.instances[inst.ID] = inst
					}
				}
			case "job":
				if cp.Operation == opDelete {
					var job params.Job
					if err := json.Unmarshal(cp.Payload, &job); err == nil && job.ID != 0 {
						delete(state.jobs, job.ID)
					}
				} else {
					var job params.Job
					if err := json.Unmarshal(cp.Payload, &job); err == nil && job.ID != 0 {
						state.jobs[job.ID] = job
					}
				}
			case "pool":
				if state.lastSnapshot != nil {
					var pool params.Pool
					if err := json.Unmarshal(cp.Payload, &pool); err == nil && pool.ID != "" {
						if cp.Operation == opDelete {
							filtered := make([]metrics.MetricsPool, 0, len(state.lastSnapshot.Pools))
							for _, p := range state.lastSnapshot.Pools {
								if p.ID != pool.ID {
									filtered = append(filtered, p)
								}
							}
							state.lastSnapshot.Pools = filtered
						} else {
							found := false
							for i, p := range state.lastSnapshot.Pools {
								if p.ID == pool.ID {
									state.lastSnapshot.Pools[i] = poolToMetrics(pool)
									found = true
									break
								}
							}
							if !found {
								state.lastSnapshot.Pools = append(state.lastSnapshot.Pools, poolToMetrics(pool))
							}
						}
					}
				}
			case "scaleset":
				if state.lastSnapshot != nil {
					var ss params.ScaleSet
					if err := json.Unmarshal(cp.Payload, &ss); err == nil && ss.ID != 0 {
						if cp.Operation == opDelete {
							filtered := make([]metrics.MetricsScaleSet, 0, len(state.lastSnapshot.ScaleSets))
							for _, s := range state.lastSnapshot.ScaleSets {
								if s.ID != ss.ID {
									filtered = append(filtered, s)
								}
							}
							state.lastSnapshot.ScaleSets = filtered
						} else {
							found := false
							for i, s := range state.lastSnapshot.ScaleSets {
								if s.ID == ss.ID {
									state.lastSnapshot.ScaleSets[i] = scaleSetToMetrics(ss)
									found = true
									break
								}
							}
							if !found {
								state.lastSnapshot.ScaleSets = append(state.lastSnapshot.ScaleSets, scaleSetToMetrics(ss))
							}
						}
					}
				}
			case evtRepository, evtOrganization, entityTypeEnterprise:
				if state.lastSnapshot != nil {
					entity := entityEventToMetrics(cp.EntityType, cp.Payload)
					if entity.ID != "" {
						if cp.Operation == opDelete {
							filtered := make([]metrics.MetricsEntity, 0, len(state.lastSnapshot.Entities))
							for _, e := range state.lastSnapshot.Entities {
								if e.ID != entity.ID {
									filtered = append(filtered, e)
								}
							}
							state.lastSnapshot.Entities = filtered
						} else {
							found := false
							for i, e := range state.lastSnapshot.Entities {
								if e.ID == entity.ID {
									// Preserve pool/scaleset counts from snapshot, update the rest
									entity.PoolCount = e.PoolCount
									entity.ScaleSetCount = e.ScaleSetCount
									state.lastSnapshot.Entities[i] = entity
									found = true
									break
								}
							}
							if !found {
								state.lastSnapshot.Entities = append(state.lastSnapshot.Entities, entity)
							}
						}
					}
				}
			}
			state.mu.Unlock()

			// Trigger immediate re-render for any entity type change
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

		// Send filter to events WebSocket — subscribe to all entity types relevant to the TUI
		eventsFilter := `{"filters":[` +
			`{"entity-type":"repository","operations":["create","update","delete"]},` +
			`{"entity-type":"organization","operations":["create","update","delete"]},` +
			`{"entity-type":"enterprise","operations":["create","update","delete"]},` +
			`{"entity-type":"pool","operations":["create","update","delete"]},` +
			`{"entity-type":"scaleset","operations":["create","update","delete"]},` +
			`{"entity-type":"instance","operations":["create","update","delete"]},` +
			`{"entity-type":"job","operations":["create","update","delete"]}]}`
		if err := eventsReader.WriteMessage(websocket.TextMessage, []byte(eventsFilter)); err != nil {
			return fmt.Errorf("failed to send events filter: %w", err)
		}

		// Watch for disconnect
		go func() {
			<-metricsReader.Done()
			if metricsConnected {
				app.QueueUpdateDraw(func() {
					updateHeader(header, mgr.BaseURL, "disconnected")
				})
			}
			time.Sleep(2 * time.Second)
			app.Stop()
		}()

		go func() {
			<-ctx.Done()
			metricsReader.Stop()
			eventsReader.Stop()
			app.Stop()
		}()

		if err := app.SetRoot(layout, true).EnableMouse(false).Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func updateHeader(header *tview.TextView, baseURL, status string) {
	now := time.Now().Format("15:04:05")
	statusColor := "[green]"
	switch status {
	case "connecting":
		statusColor = "[yellow]"
	case "disconnected":
		statusColor = "[red]"
	}
	header.SetText(fmt.Sprintf(
		" [bold][white]GARM Top[white]  │  %s  │  %s%s[white]  │  %s",
		baseURL, statusColor, status, now,
	))
}

func renderSummary(view *tview.TextView, snap *metrics.MetricsSnapshot, instanceCount int, jobs []params.Job) {
	repos, orgs, ents := 0, 0, 0
	for _, e := range snap.Entities {
		switch e.Type {
		case evtRepository:
			repos++
		case evtOrganization:
			orgs++
		case entityTypeEnterprise:
			ents++
		}
	}

	totalPools := len(snap.Pools)
	totalScaleSets := len(snap.ScaleSets)

	// Runner status from metrics snapshot
	buckets := map[string]int{}
	for _, p := range snap.Pools {
		for status, count := range p.RunnerStatusCounts {
			cat, ok := runnerStatusCategory[params.RunnerStatus(status)]
			if !ok {
				cat = "other"
			}
			buckets[cat] += count
		}
	}
	for _, ss := range snap.ScaleSets {
		for status, count := range ss.RunnerStatusCounts {
			cat, ok := runnerStatusCategory[params.RunnerStatus(status)]
			if !ok {
				cat = "other"
			}
			buckets[cat] += count
		}
	}
	active, idle, offline, pending, other := buckets["active"], buckets["idle"], buckets["offline"], buckets["pending"], buckets["other"]

	// Job counts
	queuedCount, inProgressCount, completedCount := 0, 0, 0
	for _, j := range jobs {
		switch j.Status {
		case jobQueued:
			queuedCount++
		case jobInProgress:
			inProgressCount++
		case jobCompleted:
			completedCount++
		}
	}

	line1 := fmt.Sprintf(
		" [blue]Repos:[white] %d   [green]Orgs:[white] %d   [purple]Enterprises:[white] %d   [white]Pools:[white] %d   [white]Scale Sets:[white] %d   [white]Instances:[white] %d",
		repos, orgs, ents, totalPools, totalScaleSets, instanceCount,
	)

	runnerLine := " "
	if active > 0 {
		runnerLine += fmt.Sprintf("[green]Active:[white] %d   ", active)
	}
	if idle > 0 {
		runnerLine += fmt.Sprintf("[blue]Idle:[white] %d   ", idle)
	}
	if pending > 0 {
		runnerLine += fmt.Sprintf("[yellow]Pending:[white] %d   ", pending)
	}
	if offline > 0 {
		runnerLine += fmt.Sprintf("[red]Offline:[white] %d   ", offline)
	}
	if other > 0 {
		runnerLine += fmt.Sprintf("[gray]Other:[white] %d   ", other)
	}

	jobLine := fmt.Sprintf(
		" [white]Jobs: [yellow]%d queued[white], [green]%d running[white], [gray]%d completed",
		queuedCount, inProgressCount, completedCount,
	)

	view.SetText(line1 + "\n" + runnerLine + "\n" + jobLine)
}

func renderEntitiesTable(table *tview.Table, entities []metrics.MetricsEntity) {
	table.Clear()

	headers := []string{"NAME", "TYPE", "ENDPOINT", "POOLS", "SCALESETS", "HEALTH"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		if i >= 3 {
			cell.SetAlign(tview.AlignRight)
		}
		table.SetCell(0, i, cell)
	}

	sorted := make([]metrics.MetricsEntity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool {
		ti := sorted[i].PoolCount + sorted[i].ScaleSetCount
		tj := sorted[j].PoolCount + sorted[j].ScaleSetCount
		return ti > tj
	})

	for row, e := range sorted {
		r := row + 1
		typeLabel := e.Type
		typeColor := tcell.ColorWhite
		switch e.Type {
		case evtRepository:
			typeLabel = "repo"
			typeColor = tcell.ColorDodgerBlue
		case evtOrganization:
			typeLabel = "org"
			typeColor = tcell.ColorGreen
		case entityTypeEnterprise:
			typeLabel = "ent"
			typeColor = tcell.ColorMediumPurple
		}

		healthColor := tcell.ColorGreen
		healthStr := "✓"
		if !e.Healthy {
			healthColor = tcell.ColorRed
			healthStr = "✗"
		}

		table.SetCell(r, 0, tview.NewTableCell(e.Name).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(typeLabel).SetTextColor(typeColor).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(e.Endpoint).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d", e.PoolCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d", e.ScaleSetCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(healthStr).SetTextColor(healthColor).SetAlign(tview.AlignRight).SetExpansion(1))
	}

	if len(entities) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("No entities configured").
			SetTextColor(tcell.ColorGray).SetExpansion(1))
	}
}

func renderPoolsTable(table *tview.Table, pools []metrics.MetricsPool, scaleSets []metrics.MetricsScaleSet) {
	table.Clear()

	headers := []string{"NAME", "PROVIDER", "OS", "RUNNERS", "CAP", "STATUS"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		if i >= 3 {
			cell.SetAlign(tview.AlignRight)
		}
		table.SetCell(0, i, cell)
	}

	row := 1

	sortedPools := make([]metrics.MetricsPool, len(pools))
	copy(sortedPools, pools)
	sort.Slice(sortedPools, func(i, j int) bool {
		if sortedPools[i].Enabled != sortedPools[j].Enabled {
			return sortedPools[i].Enabled
		}
		ci := sumCounts(sortedPools[i].RunnerCounts)
		cj := sumCounts(sortedPools[j].RunnerCounts)
		return ci > cj
	})

	for _, p := range sortedPools {
		name := topPoolDisplayName(p)
		current := sumCounts(p.RunnerCounts)
		maxRunners := int(p.MaxRunners)
		utilization := 0
		if maxRunners > 0 {
			utilization = current * 100 / maxRunners
		}

		runnersStr := fmt.Sprintf("%d/%d", current, maxRunners)
		capStr := fmt.Sprintf("%d%%", utilization)
		capColor := tcell.ColorGreen
		if utilization >= 90 {
			capColor = tcell.ColorRed
		} else if utilization >= 70 {
			capColor = tcell.ColorYellow
		}

		statusStr := "enabled"
		statusColor := tcell.ColorGreen
		nameColor := tcell.ColorWhite
		if !p.Enabled {
			statusStr = "disabled"
			statusColor = tcell.ColorGray
			nameColor = tcell.ColorGray
		}

		table.SetCell(row, 0, tview.NewTableCell(name).SetTextColor(nameColor).SetExpansion(1))
		table.SetCell(row, 1, tview.NewTableCell(p.ProviderName).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(p.OSType).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(runnersStr).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(capStr).SetTextColor(capColor).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetAlign(tview.AlignRight).SetExpansion(1))
		row++
	}

	for _, ss := range scaleSets {
		name := ss.Name
		if name == "" {
			name = fmt.Sprintf("scaleset-%d", ss.ID)
		}

		current := sumCounts(ss.RunnerCounts)
		maxRunners := int(ss.MaxRunners)
		utilization := 0
		if maxRunners > 0 {
			utilization = current * 100 / maxRunners
		}

		runnersStr := fmt.Sprintf("%d/%d", current, maxRunners)
		capStr := fmt.Sprintf("%d%%", utilization)
		capColor := tcell.ColorGreen
		if utilization >= 90 {
			capColor = tcell.ColorRed
		} else if utilization >= 70 {
			capColor = tcell.ColorYellow
		}

		statusStr := "enabled"
		statusColor := tcell.ColorGreen
		nameColor := tcell.ColorWhite
		if !ss.Enabled {
			statusStr = "disabled"
			statusColor = tcell.ColorGray
			nameColor = tcell.ColorGray
		}

		table.SetCell(row, 0, tview.NewTableCell(name).SetTextColor(nameColor).SetExpansion(1))
		table.SetCell(row, 1, tview.NewTableCell(ss.ProviderName).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(ss.OSType).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(runnersStr).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(capStr).SetTextColor(capColor).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetAlign(tview.AlignRight).SetExpansion(1))
		row++
	}

	if len(pools) == 0 && len(scaleSets) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("No pools or scale sets configured").
			SetTextColor(tcell.ColorGray).SetExpansion(1))
	}
}

func renderInstancesTable(table *tview.Table, instances []params.Instance) {
	table.Clear()

	headers := []string{"NAME", "STATUS", "RUNNER", "PROVIDER", "OS", "POOL/SS", "AGE"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		table.SetCell(0, i, cell)
	}

	// Sort: running first, then by creation time desc
	sorted := make([]params.Instance, len(instances))
	copy(sorted, instances)
	sort.Slice(sorted, func(i, j int) bool {
		si := instanceStatusPriorities[sorted[i].Status]
		sj := instanceStatusPriorities[sorted[j].Status]
		if si != sj {
			return si < sj
		}
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	for row, inst := range sorted {
		r := row + 1

		statusStr := string(inst.Status)
		statusColor := instanceStatusColors[inst.Status]

		runnerStr := string(inst.RunnerStatus)
		runnerColor := runnerStatusColors[inst.RunnerStatus]
		if runnerStr == "" {
			runnerStr = "-"
			runnerColor = tcell.ColorGray
		}

		poolRef := inst.PoolID
		if len(poolRef) > 8 {
			poolRef = poolRef[:8]
		}
		if inst.ScaleSetID > 0 {
			poolRef = fmt.Sprintf("ss-%d", inst.ScaleSetID)
		}
		if poolRef == "" {
			poolRef = "-"
		}

		age := time.Since(inst.CreatedAt)
		ageStr := formatDuration(age)

		name := inst.Name
		if name == "" {
			name = inst.ID
			if len(name) > 12 {
				name = name[:12]
			}
		}

		table.SetCell(r, 0, tview.NewTableCell(name).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(runnerStr).SetTextColor(runnerColor).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(inst.ProviderName).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(string(inst.OSType)).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(poolRef).SetExpansion(1))
		table.SetCell(r, 6, tview.NewTableCell(ageStr).SetExpansion(1))
	}

	if len(instances) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("No instances (waiting for events...)").
			SetTextColor(tcell.ColorGray).SetExpansion(1))
	}
}

func renderJobsTable(table *tview.Table, jobs []params.Job) {
	table.Clear()

	headers := []string{"NAME", "STATUS", "REPO", "RUNNER", "LABELS", "AGE"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		table.SetCell(0, i, cell)
	}

	// Sort: in_progress first, then queued, then completed; within group by time desc
	sorted := make([]params.Job, len(jobs))
	copy(sorted, jobs)
	sort.Slice(sorted, func(i, j int) bool {
		si := jobStatusPriorities[sorted[i].Status]
		sj := jobStatusPriorities[sorted[j].Status]
		if si != sj {
			return si < sj
		}
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	for row, job := range sorted {
		r := row + 1

		statusStr := job.Status
		statusColor := jobStatusColors[statusStr]
		if job.Conclusion != "" && job.Status == jobCompleted {
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

		labelsStr := ""
		if len(job.Labels) > 0 {
			labelsStr = truncateLabels(job.Labels, 30)
		}

		age := time.Since(job.CreatedAt)
		ageStr := formatDuration(age)

		name := job.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}

		table.SetCell(r, 0, tview.NewTableCell(name).SetExpansion(1))
		table.SetCell(r, 1, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetExpansion(1))
		table.SetCell(r, 2, tview.NewTableCell(repoStr).SetExpansion(1))
		table.SetCell(r, 3, tview.NewTableCell(runnerStr).SetExpansion(1))
		table.SetCell(r, 4, tview.NewTableCell(labelsStr).SetExpansion(1))
		table.SetCell(r, 5, tview.NewTableCell(ageStr).SetExpansion(1))
	}

	if len(jobs) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("No jobs (waiting for events...)").
			SetTextColor(tcell.ColorGray).SetExpansion(1))
	}
}

// --- Helpers ---

func topPoolDisplayName(p metrics.MetricsPool) string {
	entityName := p.RepoName
	if entityName == "" {
		entityName = p.OrgName
	}
	if entityName == "" {
		entityName = p.EnterpriseName
	}

	shortID := p.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	if entityName != "" {
		return entityName + " / " + shortID
	}
	return shortID
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, v := range counts {
		total += v
	}
	return total
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
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
	jobInProgress: 0,
	jobQueued:     1,
	jobCompleted:  2,
}

var jobStatusColors = map[string]tcell.Color{
	jobInProgress: tcell.ColorGreen,
	jobQueued:     tcell.ColorYellow,
	jobCompleted:  tcell.ColorGray,
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

func entityEventToMetrics(entityType string, payload json.RawMessage) metrics.MetricsEntity {
	switch entityType {
	case evtRepository:
		var r params.Repository
		if err := json.Unmarshal(payload, &r); err == nil && r.ID != "" {
			name := r.Name
			if r.Owner != "" {
				name = r.Owner + "/" + r.Name
			}
			return metrics.MetricsEntity{
				ID:       r.ID,
				Name:     name,
				Type:     evtRepository,
				Endpoint: r.Endpoint.Name,
				Healthy:  r.PoolManagerStatus.IsRunning,
			}
		}
	case evtOrganization:
		var o params.Organization
		if err := json.Unmarshal(payload, &o); err == nil && o.ID != "" {
			return metrics.MetricsEntity{
				ID:       o.ID,
				Name:     o.Name,
				Type:     evtOrganization,
				Endpoint: o.Endpoint.Name,
				Healthy:  o.PoolManagerStatus.IsRunning,
			}
		}
	case entityTypeEnterprise:
		var e params.Enterprise
		if err := json.Unmarshal(payload, &e); err == nil && e.ID != "" {
			return metrics.MetricsEntity{
				ID:       e.ID,
				Name:     e.Name,
				Type:     entityTypeEnterprise,
				Endpoint: e.Endpoint.Name,
				Healthy:  e.PoolManagerStatus.IsRunning,
			}
		}
	}
	return metrics.MetricsEntity{}
}

func truncateLabels(labels []string, maxLen int) string {
	result := ""
	for i, l := range labels {
		if i > 0 {
			result += ","
		}
		if len(result)+len(l) > maxLen {
			result += "..."
			break
		}
		result += l
	}
	return result
}
