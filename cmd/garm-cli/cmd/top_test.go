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
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/workers/websocket/metrics"
)

func instanceEvent(t *testing.T, op dbCommon.OperationType, inst params.Instance) changePayload {
	t.Helper()
	payload, err := json.Marshal(inst)
	require.NoError(t, err)
	return changePayload{EntityType: dbCommon.InstanceEntityType, Operation: op, Payload: payload}
}

func jobEvent(t *testing.T, op dbCommon.OperationType, job params.Job) changePayload {
	t.Helper()
	payload, err := json.Marshal(job)
	require.NoError(t, err)
	return changePayload{EntityType: dbCommon.JobEntityType, Operation: op, Payload: payload}
}

func TestApplyChangeInstancesAndJobs(t *testing.T) {
	state := newTopState()

	state.applyChange(instanceEvent(t, dbCommon.CreateOperation, params.Instance{ID: "i1", Name: "runner-1"}))
	state.applyChange(instanceEvent(t, dbCommon.UpdateOperation, params.Instance{ID: "i1", Name: "runner-1-renamed"}))
	state.applyChange(jobEvent(t, dbCommon.CreateOperation, params.Job{ID: 7, Name: "build"}))

	data := state.copyData()
	require.Len(t, data.instances, 1)
	require.Equal(t, "runner-1-renamed", data.instances[0].Name)
	require.Len(t, data.jobs, 1)

	state.applyChange(instanceEvent(t, dbCommon.DeleteOperation, params.Instance{ID: "i1"}))
	state.applyChange(jobEvent(t, dbCommon.DeleteOperation, params.Job{ID: 7}))
	data = state.copyData()
	require.Empty(t, data.instances)
	require.Empty(t, data.jobs)

	// Malformed payloads and missing IDs are dropped without effect.
	state.applyChange(changePayload{EntityType: dbCommon.InstanceEntityType, Payload: []byte("{invalid")})
	state.applyChange(instanceEvent(t, dbCommon.CreateOperation, params.Instance{Name: "no-id"}))
	require.Empty(t, state.copyData().instances)
}

func TestReconcilePrunesAndUpserts(t *testing.T) {
	state := newTopState()
	state.applyChange(instanceEvent(t, dbCommon.CreateOperation, params.Instance{ID: "stale", Name: "ghost"}))

	state.beginSeed()
	state.reconcile([]params.Instance{{ID: "fresh", Name: "runner"}}, []params.Job{{ID: 1}})

	data := state.copyData()
	require.Len(t, data.instances, 1)
	require.Equal(t, "fresh", data.instances[0].ID, "entries absent from the seed are pruned")
	require.Len(t, data.jobs, 1)
}

func TestReconcileEventsWinDuringSeed(t *testing.T) {
	state := newTopState()
	state.beginSeed()

	// While the seed request is in flight, events change the world: one
	// instance is updated and another is deleted.
	state.applyChange(instanceEvent(t, dbCommon.CreateOperation, params.Instance{ID: "i1", Name: "from-event"}))
	state.applyChange(instanceEvent(t, dbCommon.CreateOperation, params.Instance{ID: "i2", Name: "short-lived"}))
	state.applyChange(instanceEvent(t, dbCommon.DeleteOperation, params.Instance{ID: "i2"}))

	// The (stale) seed response still lists both with old names.
	state.reconcile([]params.Instance{
		{ID: "i1", Name: "stale-name"},
		{ID: "i2", Name: "deleted-meanwhile"},
	}, nil)

	data := state.copyData()
	require.Len(t, data.instances, 1)
	require.Equal(t, "from-event", data.instances[0].Name, "the event is fresher than the seed")
}

func TestApplyChangeSnapshotPatch(t *testing.T) {
	state := newTopState()

	// Pool events before the first snapshot are dropped.
	poolPayload, err := json.Marshal(params.Pool{ID: "p1", ProviderName: "lxd"})
	require.NoError(t, err)
	state.applyChange(changePayload{EntityType: dbCommon.PoolEntityType, Operation: dbCommon.UpdateOperation, Payload: poolPayload})
	require.False(t, state.copyData().haveSnapshot)

	state.setSnapshot(&metrics.MetricsSnapshot{
		Entities: []metrics.MetricsEntity{
			{ID: "e1", Name: "org/repo", Type: "repository", PoolCount: 2, ScaleSetCount: 1, Healthy: true},
		},
		Pools: []metrics.MetricsPool{
			{ID: "p1", ProviderName: "lxd", ForgeInstanceName: "gitea", Enabled: true},
		},
	})

	// A pool event patches the snapshot but keeps the forge instance name,
	// which the event payload does not carry.
	state.applyChange(changePayload{EntityType: dbCommon.PoolEntityType, Operation: dbCommon.UpdateOperation, Payload: poolPayload})
	data := state.copyData()
	require.Len(t, data.pools, 1)
	require.Equal(t, "gitea", data.pools[0].ForgeInstanceName)

	// Entity events do not carry pool/scale set counts; they are preserved.
	repoPayload, err := json.Marshal(params.Repository{ID: "e1", Owner: "org", Name: "repo"})
	require.NoError(t, err)
	state.applyChange(changePayload{EntityType: dbCommon.RepositoryEntityType, Operation: dbCommon.UpdateOperation, Payload: repoPayload})
	data = state.copyData()
	require.Len(t, data.entities, 1)
	require.Equal(t, 2, data.entities[0].PoolCount)
	require.Equal(t, 1, data.entities[0].ScaleSetCount)
	require.False(t, data.entities[0].Healthy, "health comes from the event")
}

func TestApplyChangeController(t *testing.T) {
	state := newTopState()
	payload, err := json.Marshal(params.ControllerInfo{Hostname: "garm-host", Version: "v1.2.3"})
	require.NoError(t, err)
	state.applyChange(changePayload{EntityType: dbCommon.ControllerEntityType, Operation: dbCommon.UpdateOperation, Payload: payload})

	data := state.copyData()
	require.NotNil(t, data.controller)
	require.Equal(t, "v1.2.3", data.controller.Version)
}

func TestEntityEventToMetrics(t *testing.T) {
	repo, err := json.Marshal(params.Repository{
		ID: "r1", Owner: "org", Name: "repo",
		Endpoint:          params.ForgeEndpoint{Name: "github.com"},
		PoolManagerStatus: params.PoolManagerStatus{IsRunning: true},
	})
	require.NoError(t, err)
	entity := entityEventToMetrics(dbCommon.RepositoryEntityType, repo)
	require.Equal(t, "org/repo", entity.Name)
	require.Equal(t, string(params.ForgeEntityTypeRepository), entity.Type)
	require.True(t, entity.Healthy)

	// Forge instances arrive as "forge_instance" on the events channel but
	// use "instance" in the snapshot vocabulary; their display name is the
	// endpoint name.
	forge, err := json.Marshal(params.ForgeInstance{
		ID:       "f1",
		Endpoint: params.ForgeEndpoint{Name: "gitea"},
	})
	require.NoError(t, err)
	entity = entityEventToMetrics(dbCommon.ForgeInstanceEntityType, forge)
	require.Equal(t, string(params.ForgeEntityTypeInstance), entity.Type)
	require.Equal(t, "gitea", entity.Name)

	require.Empty(t, entityEventToMetrics(dbCommon.RepositoryEntityType, []byte("{bad")).ID)
	require.Empty(t, entityEventToMetrics(dbCommon.JobEntityType, repo).ID, "unsupported entity types yield nothing")
}

func TestCapacityRows(t *testing.T) {
	pools := []metrics.MetricsPool{
		{ID: "disabled-pool", Enabled: false, RepoName: "org/repo", RunnerCounts: map[string]int{"running": 5}},
		{ID: "forge-pool", Enabled: true, ForgeInstanceName: "gitea", RunnerCounts: map[string]int{"running": 1}},
	}
	scaleSets := []metrics.MetricsScaleSet{
		{ID: 3, Enabled: true, RunnerCounts: map[string]int{"running": 2}},
	}

	rows := capacityRows(pools, scaleSets)
	require.Len(t, rows, 3)

	// Enabled rows first, sorted by runner count; the disabled pool sinks to
	// the bottom despite having the most runners.
	require.Equal(t, "scaleset:3", rows[0].key)
	require.Equal(t, "scaleset-3", rows[0].name, "unnamed scale sets get a fallback name")
	require.Equal(t, "pool:forge-pool", rows[1].key)
	require.Equal(t, "gitea / forge-po", rows[1].name, "forge instance pools show their owner")
	require.Equal(t, "pool:disabled-pool", rows[2].key)
}

func TestTruncateText(t *testing.T) {
	require.Equal(t, "short", truncateText("short", 10))
	require.Equal(t, "exact", truncateText("exact", 5))
	require.Equal(t, "abcd…", truncateText("abcdef", 5))
	// Multi-byte runes are never cut in half.
	require.Equal(t, "héll…", truncateText("héllo wörld", 5))
	require.Equal(t, "🚀🚀…", truncateText("🚀🚀🚀🚀", 3))
}

func TestFormatDuration(t *testing.T) {
	require.Equal(t, "0s", formatDuration(-5*time.Second), "clock skew clamps to zero")
	require.Equal(t, "59s", formatDuration(59*time.Second))
	require.Equal(t, "1m", formatDuration(61*time.Second))
	require.Equal(t, "1h30m", formatDuration(90*time.Minute))
	require.Equal(t, "1d1h", formatDuration(25*time.Hour))
}

func TestMatchesFilter(t *testing.T) {
	require.True(t, matchesFilter("", "anything"))
	require.True(t, matchesFilter("RUN", "runner-1", "idle"))
	require.True(t, matchesFilter("idle", "runner-1", "IDLE"))
	require.False(t, matchesFilter("gone", "runner-1", "idle"))
}

func TestIsAuthError(t *testing.T) {
	require.True(t, isAuthError(errors.New(`failed to stream logs: "bad handshake"  (401 Unauthorized)`)))
	require.False(t, isAuthError(errors.New("connection refused")))
	require.False(t, isAuthError(nil))
}

// sampleRenderData builds a small consistent world for render tests.
func sampleRenderData() renderData {
	return renderData{
		haveSnapshot: true,
		entities: []metrics.MetricsEntity{
			{ID: "e1", Name: "org/repo", Type: "repository", Endpoint: "github.com", PoolCount: 1, Healthy: true},
			{ID: "f1", Name: "gitea", Type: "instance", Endpoint: "gitea", PoolCount: 1, Healthy: true},
		},
		pools: []metrics.MetricsPool{
			{
				ID: "pool-1", ProviderName: "lxd", OSType: "linux", MaxRunners: 10, Enabled: true,
				RepoName: "org/repo", RunnerCounts: map[string]int{"running": 2},
			},
			{
				ID: "pool-2", ProviderName: "lxd", OSType: "linux", MaxRunners: 5, Enabled: true,
				ForgeInstanceName: "gitea", RunnerCounts: map[string]int{"running": 1},
			},
		},
		instances: []params.Instance{
			{
				ID: "i1", Name: "runner-a", PoolID: "pool-1", ProviderName: "lxd", OSType: "linux",
				Status: "running", RunnerStatus: params.RunnerIdle, CreatedAt: time.Now().Add(-time.Hour),
			},
			{
				ID: "i2", Name: "runner-b", PoolID: "pool-2", ProviderName: "lxd", OSType: "linux",
				Status: "running", RunnerStatus: params.RunnerActive, CreatedAt: time.Now().Add(-time.Minute),
			},
		},
		jobs: []params.Job{
			{
				ID: 1, Name: "build", Status: "queued", RepositoryOwner: "org", RepositoryName: "repo",
				Labels: []string{"self-hosted"}, CreatedAt: time.Now().Add(-time.Minute), UpdatedAt: time.Now(),
			},
		},
		controller: &params.ControllerInfo{Hostname: "garm-host", Version: "v0.1.6"},
		conn:       connConnected,
		lastUpdate: time.Now(),
	}
}

func newTestTopUI() *topUI {
	return newTopUI(tview.NewApplication(), newTopState())
}

func cellTexts(table *tview.Table, row int) []string {
	var texts []string
	for col := 0; col < table.GetColumnCount(); col++ {
		texts = append(texts, table.GetCell(row, col).Text)
	}
	return texts
}

func TestRenderPopulatesPanels(t *testing.T) {
	ui := newTestTopUI()
	ui.render(sampleRenderData())

	// Entities panel: forge instances are labeled and counted.
	require.Equal(t, " Entities (2) ", ui.tables[panelEntities].GetTitle())
	entityRows := [][]string{cellTexts(ui.tables[panelEntities], 1), cellTexts(ui.tables[panelEntities], 2)}
	require.Contains(t, fmt.Sprint(entityRows), "forge")

	// Pools panel: the forge-instance pool shows its owner.
	require.Contains(t, fmt.Sprint(cellTexts(ui.tables[panelPools], 1), cellTexts(ui.tables[panelPools], 2)), "gitea / pool-2")

	// Instances panel resolves pool IDs to their owner names.
	instRow := fmt.Sprint(cellTexts(ui.tables[panelInstances], 1))
	require.Contains(t, instRow, "runner-b", "active runner sorts before idle by recency")
	require.Contains(t, instRow, "gitea / pool-2")

	require.Equal(t, " Jobs (1) ", ui.tables[panelJobs].GetTitle())

	// Summary counts forge instances and computes runner buckets from the
	// instance list.
	summary := ui.summary.GetText(true)
	require.Contains(t, summary, "Forge instances: 1")
	require.Contains(t, summary, "Active: 1")
	require.Contains(t, summary, "Idle: 1")
	require.Contains(t, summary, "1 queued")

	header := ui.header.GetText(true)
	require.Contains(t, header, "garm-host v0.1.6")
	require.Contains(t, header, "connected")
}

func TestRenderFilterNarrowsRows(t *testing.T) {
	ui := newTestTopUI()
	ui.filters[panelInstances] = "runner-a"
	ui.render(sampleRenderData())

	table := ui.tables[panelInstances]
	require.Equal(t, 2, table.GetRowCount(), "header plus one matching row")
	require.Equal(t, " Instances (1/2 — /runner-a) ", table.GetTitle())

	ui.filters[panelInstances] = "no-such-runner"
	ui.render(sampleRenderData())
	require.Contains(t, table.GetCell(1, 0).Text, "No matches")
}

func TestSelectionFollowsRowsAcrossRenders(t *testing.T) {
	ui := newTestTopUI()
	ui.applyFocusStyles(panelInstances)
	data := sampleRenderData()
	ui.render(data)

	// Select runner-a (row 2: runner-b sorts first via active status).
	require.Equal(t, "runner-a", ui.tables[panelInstances].GetCell(2, 0).Text)
	ui.tables[panelInstances].Select(2, 0)
	require.Equal(t, "instance:i1", ui.selKeys[panelInstances])

	// runner-a becomes active and jumps to the top on the next render; the
	// selection follows it.
	data.instances[0].RunnerStatus = params.RunnerActive
	data.instances[0].CreatedAt = time.Now()
	ui.render(data)
	row, _ := ui.tables[panelInstances].GetSelection()
	require.Equal(t, "runner-a", ui.tables[panelInstances].GetCell(row, 0).Text)
}

func TestRowRefsCarryItems(t *testing.T) {
	ui := newTestTopUI()
	ui.render(sampleRenderData())

	ref := tableRowRef(ui.tables[panelJobs], 1)
	require.NotNil(t, ref)
	job, ok := ref.item.(params.Job)
	require.True(t, ok)
	require.Equal(t, int64(1), job.ID)

	require.Nil(t, tableRowRef(ui.tables[panelJobs], 0), "header row has no reference")
	require.Nil(t, tableRowRef(ui.tables[panelJobs], 99), "out of range")
}

func TestDetailTexts(t *testing.T) {
	data := sampleRenderData()
	inst := data.instances[0]
	inst.StatusMessages = []params.StatusMessage{
		{CreatedAt: time.Now(), Message: "runner [installed] ok", EventLevel: params.EventInfo},
	}
	text := instanceDetailText(inst, data, "none")
	require.Contains(t, text, "runner-a")
	require.Contains(t, text, "org/repo / pool-1")
	require.Contains(t, text, "runner [installed[] ok", "status messages are escaped for tview")

	jobText := jobDetailText(data.jobs[0])
	require.Contains(t, jobText, "org/repo")
	require.Contains(t, jobText, "self-hosted")

	rows := capacityRows(data.pools, nil)
	capText := capacityDetailText(rows[0])
	require.Contains(t, capText, "Instances by status")

	entText := entityDetailText(data.entities[1], data)
	require.Contains(t, entText, "gitea / pool-2")
}

func TestDialogCloseKeepsPanelFocus(t *testing.T) {
	// Regression: hiding a focused page makes Pages delegate focus down the
	// main layout, which lands on the entities panel and used to clobber
	// the remembered panel index.
	ui := newTestTopUI()
	ui.render(sampleRenderData())
	ui.app.SetRoot(ui.pages, true)

	// Jobs panel: its details need no async API fetch.
	ui.focusPanel(panelJobs)
	require.Equal(t, panelJobs, ui.focusIdx)

	ui.openDetails(tableRowRef(ui.tables[panelJobs], 1))
	front, _ := ui.pages.GetFrontPage()
	require.Equal(t, "details", front)

	ui.closeDetails()
	require.Equal(t, panelJobs, ui.focusIdx, "closing details must not reset the focused panel")
	require.Equal(t, tview.Primitive(ui.tables[panelJobs]), ui.app.GetFocus())
}

func TestClicksOutsideDataRowsKeepSelection(t *testing.T) {
	// Regression: a click on the header or the blank area below the rows
	// selected a non-data row, which visibly clamped to the first row until
	// the next repaint restored it.
	ui := newTestTopUI()
	ui.render(sampleRenderData())
	ui.applyFocusStyles(panelInstances)

	table := ui.tables[panelInstances]
	table.SetRect(0, 0, 80, 20) // border at y=0, header at y=1, rows from y=2
	table.Select(2, 0)
	require.Equal(t, "instance:i1", ui.selKeys[panelInstances])

	click := func(y int) {
		handler := table.MouseHandler()
		handler(tview.MouseLeftClick, tcell.NewEventMouse(5, y, tcell.ButtonNone, 0), func(tview.Primitive) {})
	}

	click(1) // header row
	row, _ := table.GetSelection()
	require.Equal(t, 2, row, "header clicks must not move the selection")

	click(15) // blank area below the rows
	row, _ = table.GetSelection()
	require.Equal(t, 2, row, "blank-area clicks must not move the selection")

	click(2) // first data row
	row, _ = table.GetSelection()
	require.Equal(t, 1, row, "clicking a data row selects it")
	require.Equal(t, "instance:i2", ui.selKeys[panelInstances])
}
