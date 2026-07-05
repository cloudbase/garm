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
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	apiClientInstances "github.com/cloudbase/garm/client/instances"
	apiClientPools "github.com/cloudbase/garm/client/pools"
	apiClientScaleSets "github.com/cloudbase/garm/client/scalesets"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/workers/websocket/metrics"
)

// topPanel indexes the four dashboard tables.
type topPanel int

const (
	panelEntities topPanel = iota
	panelPools
	panelInstances
	panelJobs
	panelCount
)

var panelNames = [panelCount]string{"Entities", "Pools & Scale Sets", "Instances", "Jobs"}

// rowRef links a table row back to the object it renders. It is stored as
// the reference of the row's first cell. The key survives re-renders (rows
// move as sorting changes), so the selection can follow the object.
type rowRef struct {
	key  string
	item any
}

const (
	topFlashDuration = 5 * time.Second
	topLogHeight     = 12
	topLogMaxLines   = 2000

	redTag = "red" // tview color tag for errors
)

// enabledLabel renders the enabled flag the way all panels display it.
func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// topUI holds the dashboard widgets and interaction state.
type topUI struct {
	app   *tview.Application
	ctx   context.Context
	state *topState
	// notify asks the render loop for a repaint. It is set by the command
	// after the loop is created; nothing fires before Run starts.
	notify func()

	pages   *tview.Pages
	header  *tview.TextView
	summary *tview.TextView
	tables  [panelCount]*tview.Table
	body    *tview.Flex // panel grid (or zoomed panel) plus optional log view
	footer  *tview.TextView
	root    *tview.Flex

	detail    *tview.TextView
	detailKey string // rowRef key the detail view shows; guards async updates

	filterBar  *tview.InputField
	filterOpen bool
	filters    [panelCount]string

	logView    *tview.TextView
	logOpen    bool
	logStarted bool

	focusIdx topPanel
	zoomed   bool
	selKeys  [panelCount]string

	flashMu    sync.Mutex
	flashMsg   string
	flashErr   bool
	flashUntil time.Time

	lastData renderData
}

var (
	topBgColor     = tcell.Color235 // #262626 - dark gray
	topBorderColor = tcell.ColorLightGray
	topFocusColor  = tcell.ColorDodgerBlue
)

func newTopUI(app *tview.Application, state *topState) *topUI {
	// Explicit dark color scheme so the TUI looks consistent regardless of
	// light/dark terminal theme. tview.Styles is global, but top and the
	// template editor never run in the same invocation.
	tview.Styles.PrimitiveBackgroundColor = topBgColor
	tview.Styles.ContrastBackgroundColor = topBgColor
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.BorderColor = topBorderColor
	tview.Styles.TitleColor = tcell.ColorWhite

	ui := &topUI{
		app:     app,
		state:   state,
		pages:   tview.NewPages(),
		header:  tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		summary: tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		footer:  tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter),
		body:    tview.NewFlex().SetDirection(tview.FlexRow),
	}
	ui.header.SetBackgroundColor(topBgColor)
	ui.footer.SetBackgroundColor(topBgColor)
	ui.summary.SetBorder(true).
		SetTitle(" Summary ").
		SetTitleAlign(tview.AlignLeft).
		SetBackgroundColor(topBgColor)

	for i := range ui.tables {
		table := tview.NewTable().
			SetBorders(false).
			SetSelectable(false, false).
			SetFixed(1, 0)
		table.SetBorder(true).
			SetTitle(" " + panelNames[i] + " ").
			SetTitleAlign(tview.AlignLeft).
			SetBackgroundColor(topBgColor)
		panel := topPanel(i)
		table.SetFocusFunc(func() { ui.onPanelFocused(panel) })
		table.SetSelectionChangedFunc(func(row, _ int) {
			if ref := tableRowRef(ui.tables[panel], row); ref != nil {
				ui.selKeys[panel] = ref.key
			}
		})
		table.SetSelectedFunc(func(row, _ int) {
			ui.openDetails(tableRowRef(ui.tables[panel], row))
		})
		// A click selects whatever cell is under the cursor, including the
		// header row or the blank area below the rows; the selection then
		// visibly clamps to the first row until the next repaint restores
		// it. Only let clicks on actual data rows through — focusing the
		// panel (mouse-down) is not affected.
		table.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			if action == tview.MouseLeftClick {
				if row, _ := ui.tables[panel].CellAt(event.Position()); tableRowRef(ui.tables[panel], row) == nil {
					return action, nil
				}
			}
			return action, event
		})
		ui.tables[i] = table
	}

	ui.filterBar = tview.NewInputField()
	ui.filterBar.SetFieldBackgroundColor(topBgColor)

	ui.logView = tview.NewTextView().
		SetDynamicColors(true).
		SetMaxLines(topLogMaxLines).
		SetScrollable(true)
	ui.logView.ScrollToEnd()
	ui.logView.SetBorder(true).
		SetTitle(" GARM Log ").
		SetTitleAlign(tview.AlignLeft).
		SetBackgroundColor(topBgColor)

	ui.layoutBody()

	ui.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.header, 1, 0, false).
		AddItem(ui.summary, 5, 0, false).
		AddItem(ui.body, 0, 1, true).
		AddItem(ui.footer, 1, 0, false)

	ui.pages.AddPage("main", ui.root, true, true)
	ui.createDetailsPage()
	ui.createHelpPage()

	app.SetInputCapture(ui.handleGlobalKey)
	app.SetFocus(ui.tables[panelEntities])
	ui.applyFocusStyles(panelEntities)

	return ui
}

// layoutBody rebuilds the middle section: either the 2x2 panel grid or the
// zoomed panel, with the log view underneath when open. The column flexes
// are rebuilt from scratch each time, which re-parents the tables cleanly.
func (ui *topUI) layoutBody() {
	ui.body.Clear()
	if ui.zoomed {
		ui.body.AddItem(ui.tables[ui.focusIdx], 0, 1, true)
	} else {
		leftCol := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(ui.tables[panelEntities], 0, 1, true).
			AddItem(ui.tables[panelPools], 0, 1, false)
		rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(ui.tables[panelInstances], 0, 1, false).
			AddItem(ui.tables[panelJobs], 0, 1, false)
		columns := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(leftCol, 0, 1, true).
			AddItem(rightCol, 0, 1, false)
		ui.body.AddItem(columns, 0, 1, true)
	}
	if ui.logOpen {
		ui.body.AddItem(ui.logView, topLogHeight, 0, false)
	}
}

// tableRowRef returns the rowRef stored on a table row, or nil for the
// header row, empty-message rows and out-of-range rows.
func tableRowRef(table *tview.Table, row int) *rowRef {
	if row < 1 || row >= table.GetRowCount() {
		return nil
	}
	ref, _ := table.GetCell(row, 0).GetReference().(*rowRef)
	return ref
}

// onPanelFocused restyles the panels when focus lands on a table, whether
// via Tab, number keys or a mouse click.
func (ui *topUI) onPanelFocused(idx topPanel) {
	if ui.filterOpen && ui.app.GetFocus() != ui.filterBar {
		ui.closeFilterBar(false)
	}
	ui.applyFocusStyles(idx)
}

// applyFocusStyles marks the focused panel: highlighted border and the only
// visible selection bar. Unfocused panels keep their selection position but
// do not display it, so the screen reads as a single cursor.
func (ui *topUI) applyFocusStyles(idx topPanel) {
	ui.focusIdx = idx
	for i, table := range ui.tables {
		if topPanel(i) == idx {
			table.SetBorderColor(topFocusColor)
			table.SetSelectable(true, false)
		} else {
			table.SetBorderColor(topBorderColor)
			table.SetSelectable(false, false)
		}
	}
}

func (ui *topUI) focusPanel(idx topPanel) {
	if ui.zoomed && idx != ui.focusIdx {
		ui.focusIdx = idx
		ui.layoutBody()
	}
	ui.app.SetFocus(ui.tables[idx])
}

func (ui *topUI) requestRepaint() {
	if ui.notify != nil {
		ui.notify()
	}
}

// handleGlobalKey is the application-level key handler for the main page.
// Keys are passed through untouched while the filter bar is being typed in
// or while a dialog (details, help, confirm) is in front — those handle
// their own input.
func (ui *topUI) handleGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if ui.app.GetFocus() == ui.filterBar {
		return event
	}
	if front, _ := ui.pages.GetFrontPage(); front != "main" {
		return event
	}

	switch event.Key() {
	case tcell.KeyTab:
		ui.focusPanel((ui.focusIdx + 1) % panelCount)
		return nil
	case tcell.KeyBacktab:
		ui.focusPanel((ui.focusIdx - 1 + panelCount) % panelCount)
		return nil
	case tcell.KeyEscape:
		if ui.filters[ui.focusIdx] != "" {
			ui.filters[ui.focusIdx] = ""
			ui.requestRepaint()
		}
		return nil
	}

	switch event.Rune() {
	case 'q', 'Q':
		ui.app.Stop()
		return nil
	case '1', '2', '3', '4':
		ui.focusPanel(topPanel(event.Rune() - '1'))
		return nil
	case '/':
		ui.openFilterBar()
		return nil
	case 'z':
		ui.zoomed = !ui.zoomed
		ui.layoutBody()
		ui.app.SetFocus(ui.tables[ui.focusIdx])
		return nil
	case 'l':
		ui.toggleLog()
		return nil
	case '?':
		ui.pages.ShowPage("help")
		return nil
	case 'd':
		if ui.focusIdx == panelInstances {
			ui.confirmDeleteRunner()
		}
		return nil
	case 'e':
		if ui.focusIdx == panelPools {
			ui.confirmToggleCapacity()
		}
		return nil
	}
	return event
}

// --- filtering ---

func (ui *topUI) openFilterBar() {
	if ui.filterOpen {
		ui.app.SetFocus(ui.filterBar)
		return
	}
	ui.filterOpen = true
	panel := ui.focusIdx
	previous := ui.filters[panel]

	ui.filterBar.SetLabel(fmt.Sprintf(" filter %s: ", strings.ToLower(panelNames[panel])))
	ui.filterBar.SetText(previous)
	ui.filterBar.SetChangedFunc(func(text string) {
		ui.filters[panel] = text
		ui.requestRepaint()
	})
	ui.filterBar.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			ui.filters[panel] = previous // Esc/Tab cancel the edit
		}
		ui.closeFilterBar(true)
	})

	ui.root.RemoveItem(ui.footer)
	ui.root.AddItem(ui.filterBar, 1, 0, true)
	ui.app.SetFocus(ui.filterBar)
}

// closeFilterBar restores the footer. When refocus is set, focus returns to
// the current panel (a mouse click elsewhere already moved focus).
func (ui *topUI) closeFilterBar(refocus bool) {
	if !ui.filterOpen {
		return
	}
	ui.filterOpen = false
	ui.root.RemoveItem(ui.filterBar)
	ui.root.AddItem(ui.footer, 1, 0, false)
	if refocus {
		ui.app.SetFocus(ui.tables[ui.focusIdx])
	}
	ui.requestRepaint()
}

// matchesFilter reports whether any field contains the filter term,
// case-insensitively. An empty filter matches everything.
func matchesFilter(filter string, fields ...string) bool {
	if filter == "" {
		return true
	}
	filter = strings.ToLower(filter)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), filter) {
			return true
		}
	}
	return false
}

// panelTitle renders a panel title with row counts and the active filter.
func panelTitle(name string, shown, total int, filter string) string {
	switch {
	case filter != "":
		return fmt.Sprintf(" %s (%d/%d — /%s) ", name, shown, total, filter)
	case total > 0:
		return fmt.Sprintf(" %s (%d) ", name, total)
	default:
		return " " + name + " "
	}
}

// --- flash messages ---

// flash shows a transient message in the footer, e.g. the outcome of an
// action. Safe to call from any goroutine.
func (ui *topUI) flash(msg string, isErr bool) {
	ui.flashMu.Lock()
	ui.flashMsg = msg
	ui.flashErr = isErr
	ui.flashUntil = time.Now().Add(topFlashDuration)
	ui.flashMu.Unlock()
	ui.requestRepaint()
}

func (ui *topUI) currentFlash() (string, bool, bool) {
	ui.flashMu.Lock()
	defer ui.flashMu.Unlock()
	if time.Now().After(ui.flashUntil) {
		return "", false, false
	}
	return ui.flashMsg, ui.flashErr, true
}

// --- actions ---

// confirm shows a modal with the given buttons plus Cancel. onChoice runs
// for any button except Cancel.
func (ui *topUI) confirm(text string, buttons []string, onChoice func(label string)) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons(append(slices.Clone(buttons), "Cancel"))
	modal.SetBackgroundColor(topBgColor)
	dismiss := func() {
		// Removing a focused page delegates focus to the first panel and
		// clobbers focusIdx; remember the panel first (see closeDetails).
		panel := ui.focusIdx
		ui.pages.RemovePage("confirm")
		ui.app.SetFocus(ui.tables[panel])
	}
	modal.SetDoneFunc(func(_ int, label string) {
		dismiss()
		if label != "Cancel" && label != "" {
			onChoice(label)
		}
	})
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dismiss()
			return nil
		}
		return event
	})
	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *topUI) confirmDeleteRunner() {
	ref := tableRowRef(ui.tables[panelInstances], ui.selectedRow(panelInstances))
	if ref == nil {
		return
	}
	inst, ok := ref.item.(params.Instance)
	if !ok || inst.Name == "" {
		return
	}
	ui.confirm(
		fmt.Sprintf("Delete runner %q?\n\nForce delete removes it even if the provider errors out.", inst.Name),
		[]string{"Delete", "Force delete"},
		func(label string) {
			force := label == "Force delete"
			go func() {
				req := apiClientInstances.NewDeleteInstanceParams()
				req.InstanceName = inst.Name
				req.ForceRemove = &force
				bypass := false
				req.BypassGHUnauthorized = &bypass
				if err := apiCli.Instances.DeleteInstance(req, authToken); err != nil {
					ui.flash(fmt.Sprintf("failed to delete runner %s: %s", inst.Name, err), true)
					return
				}
				ui.flash(fmt.Sprintf("runner %s scheduled for deletion", inst.Name), false)
			}()
		},
	)
}

func (ui *topUI) confirmToggleCapacity() {
	ref := tableRowRef(ui.tables[panelPools], ui.selectedRow(panelPools))
	if ref == nil {
		return
	}
	row, ok := ref.item.(capacityRow)
	if !ok {
		return
	}
	verb := "Enable"
	if row.enabled {
		verb = "Disable"
	}
	enabled := !row.enabled
	ui.confirm(
		fmt.Sprintf("%s %s %q?", verb, row.kind, row.name),
		[]string{verb},
		func(string) {
			go func() {
				var err error
				if row.pool != nil {
					req := apiClientPools.NewUpdatePoolParams()
					req.PoolID = row.pool.ID
					req.Body = params.UpdatePoolParams{Enabled: &enabled}
					_, err = apiCli.Pools.UpdatePool(req, authToken)
				} else if row.scaleSet != nil {
					req := apiClientScaleSets.NewUpdateScaleSetParams()
					req.ScalesetID = strconv.FormatUint(uint64(row.scaleSet.ID), 10)
					req.Body = params.UpdateScaleSetParams{Enabled: &enabled}
					_, err = apiCli.Scalesets.UpdateScaleSet(req, authToken)
				}
				if err != nil {
					ui.flash(fmt.Sprintf("failed to %s %s %s: %s", strings.ToLower(verb), row.kind, row.name, err), true)
					return
				}
				ui.flash(fmt.Sprintf("%s %s %sd", row.kind, row.name, strings.ToLower(verb)), false)
			}()
		},
	)
}

func (ui *topUI) selectedRow(panel topPanel) int {
	row, _ := ui.tables[panel].GetSelection()
	return row
}

// --- details ---

func (ui *topUI) createDetailsPage() {
	ui.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	ui.detail.SetBorder(true)
	ui.detail.SetBackgroundColor(topBgColor)
	ui.detail.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyEnter,
			event.Rune() == 'q':
			ui.closeDetails()
			return nil
		}
		return event
	})

	grid := tview.NewGrid().
		SetColumns(0, 100, 0).
		SetRows(0, 34, 0).
		AddItem(ui.detail, 1, 1, 1, 1, 0, 0, true)
	ui.pages.AddPage("details", grid, true, false)
}

func (ui *topUI) closeDetails() {
	ui.detailKey = ""
	// Hiding a focused page makes Pages delegate focus down the main
	// layout's chain, which always ends at the first panel — its focus
	// callback overwrites focusIdx before we can use it. Remember the
	// panel first, then put focus back once the page is gone.
	panel := ui.focusIdx
	ui.pages.HidePage("details")
	ui.app.SetFocus(ui.tables[panel])
}

// openDetails shows the detail dialog for a table row. Instance details are
// refreshed asynchronously: list responses and events do not carry status
// messages, so a targeted GET fills them in.
func (ui *topUI) openDetails(ref *rowRef) {
	if ref == nil {
		return
	}
	ui.detailKey = ref.key

	switch item := ref.item.(type) {
	case params.Instance:
		ui.detail.SetTitle(fmt.Sprintf(" Runner: %s ", item.Name))
		ui.detail.SetText(instanceDetailText(item, ui.lastData, "[gray]fetching status messages...[-]"))
		ui.fetchInstanceDetails(ref.key, item.Name)
	case params.Job:
		ui.detail.SetTitle(fmt.Sprintf(" Job: %s ", truncateText(item.Name, 60)))
		ui.detail.SetText(jobDetailText(item))
	case capacityRow:
		title := strings.ToUpper(item.kind[:1]) + item.kind[1:]
		ui.detail.SetTitle(fmt.Sprintf(" %s: %s ", title, item.name))
		ui.detail.SetText(capacityDetailText(item))
	case metrics.MetricsEntity:
		ui.detail.SetTitle(fmt.Sprintf(" %s: %s ", entityTypeLabel(item.Type), item.Name))
		ui.detail.SetText(entityDetailText(item, ui.lastData))
	default:
		return
	}
	ui.detail.ScrollToBeginning()
	ui.pages.ShowPage("details")
	ui.app.SetFocus(ui.detail)
}

func (ui *topUI) fetchInstanceDetails(key, name string) {
	if name == "" {
		return
	}
	go func() {
		req := apiClientInstances.NewGetInstanceParams()
		req.InstanceName = name
		resp, err := apiCli.Instances.GetInstance(req, authToken)
		if err != nil {
			return // keep showing cached data
		}
		ui.app.QueueUpdateDraw(func() {
			if ui.detailKey != key {
				return // dialog closed or showing something else
			}
			ui.detail.SetText(instanceDetailText(resp.Payload, ui.lastData, "[gray]none[-]"))
		})
	}()
}

func detailField(label, value string) string {
	if value == "" {
		value = "-"
	}
	return fmt.Sprintf("[yellow]%-14s[-] %s\n", label+":", tview.Escape(value))
}

func instanceDetailText(inst params.Instance, data renderData, noMessages string) string {
	var b strings.Builder
	b.WriteString(detailField("Name", inst.Name))
	b.WriteString(detailField("ID", inst.ID))
	b.WriteString(detailField("Provider ID", inst.ProviderID))
	b.WriteString(detailField("Provider", inst.ProviderName))
	if inst.AgentID != 0 {
		b.WriteString(detailField("Agent ID", strconv.FormatInt(inst.AgentID, 10)))
	}
	b.WriteString(detailField("Status", string(inst.Status)))
	b.WriteString(detailField("Runner status", string(inst.RunnerStatus)))
	os := strings.TrimSpace(fmt.Sprintf("%s %s %s (%s)", inst.OSType, inst.OSName, inst.OSVersion, inst.OSArch))
	b.WriteString(detailField("OS", os))

	var addresses []string
	for _, addr := range inst.Addresses {
		addresses = append(addresses, addr.Address)
	}
	b.WriteString(detailField("Addresses", strings.Join(addresses, ", ")))

	switch {
	case inst.ScaleSetID > 0:
		b.WriteString(detailField("Scale set", scaleSetNameByID(data.scaleSets, inst.ScaleSetID)))
	case inst.PoolID != "":
		b.WriteString(detailField("Pool", poolNameByID(data.pools, inst.PoolID)))
	}
	b.WriteString(detailField("Created", inst.CreatedAt.Local().Format(time.DateTime)))
	b.WriteString(detailField("Age", formatDuration(time.Since(inst.CreatedAt))))

	if len(inst.ProviderFault) > 0 {
		fmt.Fprintf(&b, "\n[red]Provider fault:[-]\n%s\n", tview.Escape(string(inst.ProviderFault)))
	}

	b.WriteString("\n[yellow]Status messages:[-]\n")
	if len(inst.StatusMessages) == 0 {
		b.WriteString(noMessages + "\n")
	}
	const maxMessages = 20
	msgs := inst.StatusMessages
	if len(msgs) > maxMessages {
		msgs = msgs[len(msgs)-maxMessages:]
	}
	for _, msg := range msgs {
		level := string(msg.EventLevel)
		color := "gray"
		switch msg.EventLevel {
		case params.EventError:
			color = redTag
		case params.EventWarning:
			color = "yellow"
		}
		fmt.Fprintf(&b, "[gray]%s[-] [%s]%-7s[-] %s\n",
			msg.CreatedAt.Local().Format("01-02 15:04:05"), color, level, tview.Escape(msg.Message))
	}
	return b.String()
}

func jobDetailText(job params.Job) string {
	var b strings.Builder
	b.WriteString(detailField("Name", job.Name))
	b.WriteString(detailField("Job ID", strconv.FormatInt(job.ID, 10)))
	if job.WorkflowJobID != 0 {
		b.WriteString(detailField("Workflow job", strconv.FormatInt(job.WorkflowJobID, 10)))
	}
	if job.RunID != 0 {
		b.WriteString(detailField("Run ID", strconv.FormatInt(job.RunID, 10)))
	}
	status := job.Status
	if job.Conclusion != "" {
		status = fmt.Sprintf("%s (%s)", job.Status, job.Conclusion)
	}
	b.WriteString(detailField("Status", status))
	if job.RepositoryOwner != "" || job.RepositoryName != "" {
		b.WriteString(detailField("Repository", job.RepositoryOwner+"/"+job.RepositoryName))
	}
	b.WriteString(detailField("Runner", job.RunnerName))
	if job.RunnerGroupName != "" {
		b.WriteString(detailField("Runner group", job.RunnerGroupName))
	}
	b.WriteString(detailField("Labels", strings.Join(job.Labels, ", ")))
	b.WriteString(detailField("Created", job.CreatedAt.Local().Format(time.DateTime)))
	if !job.StartedAt.IsZero() {
		b.WriteString(detailField("Started", job.StartedAt.Local().Format(time.DateTime)))
	}
	if !job.CompletedAt.IsZero() {
		b.WriteString(detailField("Completed", job.CompletedAt.Local().Format(time.DateTime)))
	}
	b.WriteString(detailField("Run URL", job.WorkflowRunURL))
	return b.String()
}

func capacityDetailText(row capacityRow) string {
	var b strings.Builder
	b.WriteString(detailField("Name", row.name))
	b.WriteString(detailField("ID", row.id))
	b.WriteString(detailField("Type", row.kind))
	b.WriteString(detailField("Owner", row.owner))
	b.WriteString(detailField("Provider", row.provider))
	b.WriteString(detailField("OS", row.osType))
	b.WriteString(detailField("Runners", fmt.Sprintf("%d / %d max", row.current, row.max)))
	b.WriteString(detailField("Status", enabledLabel(row.enabled)))

	writeCounts := func(title string, counts map[string]int) {
		if len(counts) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n[yellow]%s:[-]\n", title)
		for _, k := range slices.Sorted(maps.Keys(counts)) {
			fmt.Fprintf(&b, "  %-20s %d\n", tview.Escape(k), counts[k])
		}
	}
	writeCounts("Instances by status", row.runnerCounts)
	writeCounts("Runners by status", row.runnerStatusCounts)
	return b.String()
}

func entityDetailText(entity metrics.MetricsEntity, data renderData) string {
	var b strings.Builder
	b.WriteString(detailField("Name", entity.Name))
	b.WriteString(detailField("ID", entity.ID))
	b.WriteString(detailField("Type", entityTypeLabel(entity.Type)))
	b.WriteString(detailField("Endpoint", entity.Endpoint))
	health := "running"
	if !entity.Healthy {
		health = "stopped"
	}
	b.WriteString(detailField("Pool manager", health))
	b.WriteString(detailField("Pools", strconv.Itoa(entity.PoolCount)))
	b.WriteString(detailField("Scale sets", strconv.Itoa(entity.ScaleSetCount)))

	rows := capacityRows(data.pools, data.scaleSets)
	var owned []capacityRow
	for _, row := range rows {
		if row.owner == entity.Name {
			owned = append(owned, row)
		}
	}
	if len(owned) > 0 {
		b.WriteString("\n[yellow]Capacity:[-]\n")
		for _, row := range owned {
			fmt.Fprintf(&b, "  %-10s %-30s %-10s %d/%d %s\n",
				row.kind, tview.Escape(truncateText(row.name, 30)), tview.Escape(row.provider), row.current, row.max, enabledLabel(row.enabled))
		}
	}
	return b.String()
}

// --- log panel ---

func (ui *topUI) toggleLog() {
	ui.logOpen = !ui.logOpen
	if ui.logOpen && !ui.logStarted {
		ui.logStarted = true
		go ui.runLogStream()
	}
	ui.layoutBody()
	ui.requestRepaint()
}

// runLogStream tails the server log over WebSocket, reconnecting with the
// same cadence as the main streams. It runs once, started on the first
// toggle; closing the panel merely hides it.
func (ui *topUI) runLogStream() {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		reader, err := garmWs.NewReader(ui.ctx, mgr.BaseURL, "/api/v1/ws/logs", mgr.Token, ui.handleLogMessage)
		if err == nil {
			err = reader.Start()
		}
		if err == nil {
			backoff = time.Second
			select {
			case <-ui.ctx.Done():
				reader.Stop()
				return
			case <-reader.Done():
			}
		}
		fmt.Fprintf(ui.logView, "[red]-- log stream disconnected, retrying in %s --[-]\n", backoff.Round(time.Second))
		ui.requestRepaint()
		select {
		case <-ui.ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// handleLogMessage renders one server log line into the log view. Lines are
// slog JSON records; anything that does not parse is shown raw.
func (ui *topUI) handleLogMessage(_ int, msg []byte) error {
	var record map[string]any
	if err := json.Unmarshal(msg, &record); err != nil {
		fmt.Fprintln(ui.logView, tview.Escape(string(msg)))
		ui.requestRepaint()
		return nil //nolint:nilerr // non-JSON lines are shown raw, not an error
	}

	timestamp := ""
	if raw, ok := record["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			timestamp = t.Local().Format("15:04:05")
		}
	}
	level, _ := record["level"].(string)
	message, _ := record["msg"].(string)

	levelColor := "white"
	switch strings.ToUpper(level) {
	case "ERROR":
		levelColor = redTag
	case "WARN", "WARNING":
		levelColor = "yellow"
	case "DEBUG":
		levelColor = "gray"
	}

	var attrs []string
	for _, k := range slices.Sorted(maps.Keys(record)) {
		switch k {
		case "time", "level", "msg", "source":
			// source is the slog caller location — far too wide for a
			// dashboard tail; use `garm-cli debug-log` for full records.
			continue
		}
		attrs = append(attrs, fmt.Sprintf("%s=%v", k, record[k]))
	}
	attrText := ""
	if len(attrs) > 0 {
		attrText = " [gray]" + tview.Escape(strings.Join(attrs, " ")) + "[-]"
	}

	fmt.Fprintf(ui.logView, "[gray]%s[-] [%s]%-5s[-] %s%s\n",
		timestamp, levelColor, level, tview.Escape(message), attrText)
	ui.requestRepaint()
	return nil
}

// --- help ---

const topHelpText = `[green]Navigation[-]

[yellow]Tab / Shift+Tab[-]   Cycle through panels
[yellow]1 2 3 4[-]           Jump to a panel
[yellow]Up/Down, PgUp/PgDn[-] Move the selection
[yellow]Mouse[-]             Click to focus/select, wheel to scroll

[green]Panels[-]

[yellow]Enter[-]             Details for the selected row
[yellow]/[-]                 Filter the focused panel (Enter keeps, Esc cancels)
[yellow]Esc[-]               Clear the focused panel's filter
[yellow]z[-]                 Zoom the focused panel to full size

[green]Actions[-]

[yellow]d[-]                 Delete the selected runner (Instances panel)
[yellow]e[-]                 Enable/disable the selected pool or scale set

[green]Other[-]

[yellow]l[-]                 Tail the server log at the bottom
[yellow]?[-]                 This help
[yellow]q[-]                 Quit

The dashboard reconnects automatically if the server goes away; the header
shows the connection state and the time of the last received update.

[blue]Press Escape to close[-]`

func (ui *topUI) createHelpPage() {
	view := tview.NewTextView().SetDynamicColors(true).SetText(topHelpText)
	view.SetBorder(true)
	view.SetTitle(" Help ")
	view.SetBackgroundColor(topBgColor)
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape, event.Rune() == 'q', event.Rune() == '?':
			// Remember the panel before hiding the page (see closeDetails).
			panel := ui.focusIdx
			ui.pages.HidePage("help")
			ui.app.SetFocus(ui.tables[panel])
			return nil
		}
		return event
	})

	grid := tview.NewGrid().
		SetColumns(0, 70, 0).
		SetRows(0, 34, 0).
		AddItem(view, 1, 1, 1, 1, 0, 0, true)
	ui.pages.AddPage("help", grid, true, false)
}

// --- rendering ---

// render redraws the whole dashboard. It must run on the UI goroutine
// (either before the application starts or via app.QueueUpdateDraw).
func (ui *topUI) render(data renderData) {
	ui.lastData = data
	ui.renderHeader(data)
	ui.renderSummary(data)
	ui.renderEntities(data)
	ui.renderCapacity(data)
	ui.renderInstances(data)
	ui.renderJobs(data)
	ui.renderFooter()
}

func (ui *topUI) renderHeader(data renderData) {
	controller := ""
	if data.controller != nil {
		name := cmp.Or(data.controller.Hostname, "controller")
		version := data.controller.Version
		controller = fmt.Sprintf("%s %s  │  ", name, version)
	}

	var status string
	switch data.conn {
	case connConnected:
		status = "[green]● connected[-]"
	case connConnecting:
		status = "[yellow]● connecting[-]"
	case connReconnecting:
		status = "[red]● reconnecting[-]"
		if data.connDetail != "" {
			status += " [gray](" + tview.Escape(data.connDetail) + ")[-]"
		}
	}

	updated := "no data yet"
	if !data.lastUpdate.IsZero() {
		updated = fmt.Sprintf("updated %s ago", formatDuration(time.Since(data.lastUpdate)))
	}

	ui.header.SetText(fmt.Sprintf(
		" [::b]GARM Top[::-]  │  %s%s  │  %s  │  %s  │  %s",
		tview.Escape(controller), tview.Escape(mgr.BaseURL), status, updated,
		time.Now().Format("15:04:05"),
	))
}

func (ui *topUI) renderSummary(data renderData) {
	var entityCounts, capacityCounts string
	if data.haveSnapshot {
		repos, orgs, ents, forges := 0, 0, 0, 0
		for _, e := range data.entities {
			// The snapshot uses the params.ForgeEntityType vocabulary. Note
			// that a forge instance is "instance" there, not to be confused
			// with dbCommon's InstanceEntityType (a runner) used on the
			// events channel.
			switch params.ForgeEntityType(e.Type) {
			case params.ForgeEntityTypeRepository:
				repos++
			case params.ForgeEntityTypeOrganization:
				orgs++
			case params.ForgeEntityTypeEnterprise:
				ents++
			case params.ForgeEntityTypeInstance:
				forges++
			}
		}
		entityCounts = fmt.Sprintf(
			"[blue]Repos:[-] %d   [green]Orgs:[-] %d   [purple]Enterprises:[-] %d   [teal]Forge instances:[-] %d",
			repos, orgs, ents, forges,
		)
		capacityCounts = fmt.Sprintf("   Pools: %d   Scale sets: %d", len(data.pools), len(data.scaleSets))
	} else {
		entityCounts = "[gray]Waiting for the first metrics snapshot..."
	}

	// Runner buckets come from the live instance list — the same source as
	// the Instances panel — rather than the snapshot, which can lag by up
	// to 5 seconds.
	buckets := map[string]int{}
	for _, inst := range data.instances {
		cat, ok := runnerStatusCategory[inst.RunnerStatus]
		if !ok {
			cat = "other"
		}
		buckets[cat]++
	}
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
			runnerLine += fmt.Sprintf("[%s]%s:[-] %d   ", bucket.color, bucket.label, count)
		}
	}
	if runnerLine == " " {
		runnerLine = " [gray]No runners"
	} else {
		runnerLine += fmt.Sprintf("[white]Total:[-] %d", len(data.instances))
	}

	queued, inProgress, completed := 0, 0, 0
	for _, j := range data.jobs {
		switch params.JobStatus(j.Status) {
		case params.JobStatusQueued:
			queued++
		case params.JobStatusInProgress:
			inProgress++
		case params.JobStatusCompleted:
			completed++
		}
	}
	jobLine := fmt.Sprintf(
		" Jobs: [yellow]%d queued[-], [green]%d running[-], [gray]%d completed[-]",
		queued, inProgress, completed,
	)

	ui.summary.SetText(" " + entityCounts + capacityCounts + "\n" + runnerLine + "\n" + jobLine)
}

func (ui *topUI) renderFooter() {
	if msg, isErr, ok := ui.currentFlash(); ok {
		color := "green"
		if isErr {
			color = redTag
		}
		ui.footer.SetText(fmt.Sprintf("[%s]%s[-]", color, tview.Escape(msg)))
		return
	}
	hints := "[yellow]Tab/1-4[-] panels │ [yellow]↵[-] details │ [yellow]/[-] filter │ [yellow]z[-] zoom │ [yellow]l[-] log │ [yellow]d[-] delete runner │ [yellow]e[-] enable/disable │ [yellow]?[-] help │ [yellow]q[-] quit"
	if ui.focusIdx != panelInstances {
		hints = strings.Replace(hints, " │ [yellow]d[-] delete runner", "", 1)
	}
	if ui.focusIdx != panelPools {
		hints = strings.Replace(hints, " │ [yellow]e[-] enable/disable", "", 1)
	}
	ui.footer.SetText(hints)
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

func emptyOrFiltered(base, filter string) string {
	if filter != "" {
		return fmt.Sprintf("No matches for /%s", filter)
	}
	return base
}

// restoreSelection re-selects the row whose key was selected before the
// re-render. Rows move as sorting reacts to status changes; without this the
// selection would silently migrate to whatever row landed on the old index.
func (ui *topUI) restoreSelection(panel topPanel) {
	table := ui.tables[panel]
	rowCount := table.GetRowCount()
	if rowCount <= 1 {
		return
	}
	if key := ui.selKeys[panel]; key != "" {
		for row := 1; row < rowCount; row++ {
			if ref := tableRowRef(table, row); ref != nil && ref.key == key {
				table.Select(row, 0)
				return
			}
		}
	}
	// The selected object is gone (or nothing was selected): clamp into
	// range and adopt whatever is there now.
	row, _ := table.GetSelection()
	row = max(1, min(row, rowCount-1))
	table.Select(row, 0)
}

// entityTypeLabel maps a snapshot entity type to a short display label.
func entityTypeLabel(entityType string) string {
	switch params.ForgeEntityType(entityType) {
	case params.ForgeEntityTypeRepository:
		return "repo"
	case params.ForgeEntityTypeOrganization:
		return "org"
	case params.ForgeEntityTypeEnterprise:
		return "ent"
	case params.ForgeEntityTypeInstance:
		return "forge"
	}
	return entityType
}

var entityTypeColors = map[string]tcell.Color{
	"repo":  tcell.ColorDodgerBlue,
	"org":   tcell.ColorGreen,
	"ent":   tcell.ColorMediumPurple,
	"forge": tcell.ColorTeal,
}

// renderEntities renders the entities panel. It sorts the slice in place.
func (ui *topUI) renderEntities(data renderData) {
	table := ui.tables[panelEntities]
	filter := ui.filters[panelEntities]
	table.Clear()
	setTableHeader(table, []string{"NAME", "TYPE", "ENDPOINT", "POOLS", "SCALESETS", "HEALTH"}, 3)

	entities := data.entities
	slices.SortFunc(entities, func(a, b metrics.MetricsEntity) int {
		return cmp.Or(
			cmp.Compare(b.PoolCount+b.ScaleSetCount, a.PoolCount+a.ScaleSetCount),
			cmp.Compare(a.Name, b.Name),
		)
	})

	row := 1
	for _, e := range entities {
		typeLabel := entityTypeLabel(e.Type)
		if !matchesFilter(filter, e.Name, typeLabel, e.Endpoint) {
			continue
		}
		typeColor := entityTypeColors[typeLabel]
		if typeColor == 0 {
			typeColor = tcell.ColorWhite
		}
		healthColor, healthStr := tcell.ColorGreen, "✓"
		if !e.Healthy {
			healthColor, healthStr = tcell.ColorRed, "✗"
		}

		nameCell := tview.NewTableCell(e.Name).SetExpansion(1).
			SetReference(&rowRef{key: "entity:" + e.ID, item: e})
		table.SetCell(row, 0, nameCell)
		table.SetCell(row, 1, tview.NewTableCell(typeLabel).SetTextColor(typeColor).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(e.Endpoint).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(strconv.Itoa(e.PoolCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(strconv.Itoa(e.ScaleSetCount)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(healthStr).SetTextColor(healthColor).SetAlign(tview.AlignRight).SetExpansion(1))
		row++
	}
	table.SetTitle(panelTitle(panelNames[panelEntities], row-1, len(entities), filter))
	if row == 1 {
		setEmptyMessage(table, emptyOrFiltered("No entities configured", filter))
		return
	}
	ui.restoreSelection(panelEntities)
}

// capacityRow is the common display shape of pools and scale sets, so both
// sort and render through a single code path.
type capacityRow struct {
	key      string
	kind     string // "pool" or "scale set"
	id       string
	name     string
	owner    string
	provider string
	osType   string
	current  int
	max      int
	enabled  bool

	runnerCounts       map[string]int
	runnerStatusCounts map[string]int

	pool     *metrics.MetricsPool
	scaleSet *metrics.MetricsScaleSet
}

func poolOwnerName(p metrics.MetricsPool) string {
	return cmp.Or(p.RepoName, p.OrgName, p.EnterpriseName, p.ForgeInstanceName)
}

func capacityRows(pools []metrics.MetricsPool, scaleSets []metrics.MetricsScaleSet) []capacityRow {
	rows := make([]capacityRow, 0, len(pools)+len(scaleSets))
	for i := range pools {
		p := &pools[i]
		owner := poolOwnerName(*p)
		name := shortID(p.ID, 8)
		if owner != "" {
			name = owner + " / " + name
		}
		rows = append(rows, capacityRow{
			key:                "pool:" + p.ID,
			kind:               "pool",
			id:                 p.ID,
			name:               name,
			owner:              owner,
			provider:           p.ProviderName,
			osType:             p.OSType,
			current:            sumCounts(p.RunnerCounts),
			max:                int(p.MaxRunners), // #nosec G115 - max runners fit in int
			enabled:            p.Enabled,
			runnerCounts:       p.RunnerCounts,
			runnerStatusCounts: p.RunnerStatusCounts,
			pool:               p,
		})
	}
	for i := range scaleSets {
		ss := &scaleSets[i]
		name := ss.Name
		if name == "" {
			name = fmt.Sprintf("scaleset-%d", ss.ID)
		}
		rows = append(rows, capacityRow{
			key:                fmt.Sprintf("scaleset:%d", ss.ID),
			kind:               "scale set",
			id:                 strconv.FormatUint(uint64(ss.ID), 10),
			name:               name,
			owner:              cmp.Or(ss.RepoName, ss.OrgName, ss.EnterpriseName),
			provider:           ss.ProviderName,
			osType:             ss.OSType,
			current:            sumCounts(ss.RunnerCounts),
			max:                int(ss.MaxRunners), // #nosec G115 - max runners fit in int
			enabled:            ss.Enabled,
			runnerCounts:       ss.RunnerCounts,
			runnerStatusCounts: ss.RunnerStatusCounts,
			scaleSet:           ss,
		})
	}
	// Enabled first, then by runner count, with a stable key tiebreaker so
	// rows do not jump around between refreshes.
	slices.SortFunc(rows, func(a, b capacityRow) int {
		if a.enabled != b.enabled {
			if a.enabled {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(b.current, a.current),
			cmp.Compare(a.key, b.key),
		)
	})
	return rows
}

// renderCapacity renders the pools and scale sets panel.
func (ui *topUI) renderCapacity(data renderData) {
	table := ui.tables[panelPools]
	filter := ui.filters[panelPools]
	table.Clear()
	setTableHeader(table, []string{"NAME", "PROVIDER", "OS", "RUNNERS", "CAP", "STATUS"}, 3)

	rows := capacityRows(data.pools, data.scaleSets)
	row := 1
	for _, r := range rows {
		status := enabledLabel(r.enabled)
		if !matchesFilter(filter, r.name, r.kind, r.provider, r.osType, status) {
			continue
		}

		utilization := 0
		if r.max > 0 {
			utilization = r.current * 100 / r.max
		}
		capColor := tcell.ColorGreen
		switch {
		case utilization >= 90:
			capColor = tcell.ColorRed
		case utilization >= 70:
			capColor = tcell.ColorYellow
		}
		statusColor, nameColor := tcell.ColorGreen, tcell.ColorWhite
		if !r.enabled {
			statusColor, nameColor = tcell.ColorGray, tcell.ColorGray
		}

		nameCell := tview.NewTableCell(r.name).SetTextColor(nameColor).SetExpansion(1).
			SetReference(&rowRef{key: r.key, item: r})
		table.SetCell(row, 0, nameCell)
		table.SetCell(row, 1, tview.NewTableCell(r.provider).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(r.osType).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d/%d", r.current, r.max)).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%d%%", utilization)).SetTextColor(capColor).SetAlign(tview.AlignRight).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(status).SetTextColor(statusColor).SetAlign(tview.AlignRight).SetExpansion(1))
		row++
	}
	table.SetTitle(panelTitle(panelNames[panelPools], row-1, len(rows), filter))
	if row == 1 {
		setEmptyMessage(table, emptyOrFiltered("No pools or scale sets configured", filter))
		return
	}
	ui.restoreSelection(panelPools)
}

func poolNameByID(pools []metrics.MetricsPool, id string) string {
	for _, p := range pools {
		if p.ID == id {
			if owner := poolOwnerName(p); owner != "" {
				return owner + " / " + shortID(id, 8)
			}
			break
		}
	}
	return shortID(id, 8)
}

func scaleSetNameByID(scaleSets []metrics.MetricsScaleSet, id uint) string {
	for _, ss := range scaleSets {
		if ss.ID == id && ss.Name != "" {
			return ss.Name
		}
	}
	return fmt.Sprintf("scaleset-%d", id)
}

// renderInstances renders the instances panel. It sorts the slice in place.
func (ui *topUI) renderInstances(data renderData) {
	table := ui.tables[panelInstances]
	filter := ui.filters[panelInstances]
	table.Clear()
	setTableHeader(table, []string{"NAME", "STATUS", "RUNNER", "PROVIDER", "OS", "POOL/SS", "AGE"}, -1)

	// Sort: running first, then by creation time desc, then by name so rows
	// do not jump around between refreshes.
	instances := data.instances
	slices.SortFunc(instances, func(a, b params.Instance) int {
		return cmp.Or(
			cmp.Compare(instanceStatusPriorities[a.Status], instanceStatusPriorities[b.Status]),
			b.CreatedAt.Compare(a.CreatedAt),
			cmp.Compare(a.Name, b.Name),
		)
	})

	row := 1
	for _, inst := range instances {
		runnerStr := string(inst.RunnerStatus)
		runnerColor := runnerStatusColors[inst.RunnerStatus]
		if runnerStr == "" {
			runnerStr = "-"
			runnerColor = tcell.ColorGray
		}

		poolRef := "-"
		switch {
		case inst.ScaleSetID > 0:
			poolRef = scaleSetNameByID(data.scaleSets, inst.ScaleSetID)
		case inst.PoolID != "":
			poolRef = poolNameByID(data.pools, inst.PoolID)
		}

		name := inst.Name
		if name == "" {
			name = shortID(inst.ID, 12)
		}
		if !matchesFilter(filter, name, string(inst.Status), runnerStr, inst.ProviderName, string(inst.OSType), poolRef) {
			continue
		}

		nameCell := tview.NewTableCell(name).SetExpansion(1).
			SetReference(&rowRef{key: "instance:" + inst.ID, item: inst})
		table.SetCell(row, 0, nameCell)
		table.SetCell(row, 1, tview.NewTableCell(string(inst.Status)).SetTextColor(instanceStatusColors[inst.Status]).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(runnerStr).SetTextColor(runnerColor).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(inst.ProviderName).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(string(inst.OSType)).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(truncateText(poolRef, 30)).SetExpansion(1))
		table.SetCell(row, 6, tview.NewTableCell(formatDuration(time.Since(inst.CreatedAt))).SetExpansion(1))
		row++
	}
	table.SetTitle(panelTitle(panelNames[panelInstances], row-1, len(instances), filter))
	if row == 1 {
		setEmptyMessage(table, emptyOrFiltered("No instances", filter))
		return
	}
	ui.restoreSelection(panelInstances)
}

// renderJobs renders the jobs panel. It sorts the slice in place.
func (ui *topUI) renderJobs(data renderData) {
	table := ui.tables[panelJobs]
	filter := ui.filters[panelJobs]
	table.Clear()
	setTableHeader(table, []string{"NAME", "STATUS", "REPO", "RUNNER", "LABELS", "AGE"}, -1)

	// Sort: in_progress first, then queued, then completed; within group by
	// update time desc, with the ID as a stable tiebreaker.
	jobs := data.jobs
	slices.SortFunc(jobs, func(a, b params.Job) int {
		return cmp.Or(
			cmp.Compare(jobStatusPriorities[a.Status], jobStatusPriorities[b.Status]),
			b.UpdatedAt.Compare(a.UpdatedAt),
			cmp.Compare(a.ID, b.ID),
		)
	})

	row := 1
	for _, job := range jobs {
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
		labels := strings.Join(job.Labels, ",")
		if !matchesFilter(filter, job.Name, statusStr, repoStr, runnerStr, labels) {
			continue
		}

		nameCell := tview.NewTableCell(truncateText(job.Name, 40)).SetExpansion(1).
			SetReference(&rowRef{key: "job:" + strconv.FormatInt(job.ID, 10), item: job})
		table.SetCell(row, 0, nameCell)
		table.SetCell(row, 1, tview.NewTableCell(statusStr).SetTextColor(statusColor).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(repoStr).SetExpansion(1))
		table.SetCell(row, 3, tview.NewTableCell(runnerStr).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(truncateText(labels, 30)).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(formatDuration(time.Since(job.CreatedAt))).SetExpansion(1))
		row++
	}
	table.SetTitle(panelTitle(panelNames[panelJobs], row-1, len(jobs), filter))
	if row == 1 {
		setEmptyMessage(table, emptyOrFiltered("No jobs", filter))
		return
	}
	ui.restoreSelection(panelJobs)
}

// --- Helpers ---

func shortID(id string, maxLen int) string {
	if len(id) > maxLen {
		return id[:maxLen]
	}
	return id
}

// truncateText shortens a string to at most maxRunes runes, appending an
// ellipsis. Unlike a byte slice, it cannot cut a multi-byte character in
// half (job names may contain any unicode).
func truncateText(s string, maxRunes int) string {
	if maxRunes < 1 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
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

// entityEventToMetrics converts an entity change event into the snapshot's
// entity shape. Events use the dbCommon.DatabaseEntityType vocabulary while
// the snapshot uses params.ForgeEntityType; notably a forge instance is
// "forge_instance" on the wire but "instance" in the snapshot.
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
			Type:     string(params.ForgeEntityTypeRepository),
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
			Type:     string(params.ForgeEntityTypeOrganization),
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
			Type:     string(params.ForgeEntityTypeEnterprise),
			Endpoint: e.Endpoint.Name,
			Healthy:  e.PoolManagerStatus.IsRunning,
		}
	case dbCommon.ForgeInstanceEntityType:
		var f params.ForgeInstance
		if err := json.Unmarshal(payload, &f); err != nil || f.ID == "" {
			return metrics.MetricsEntity{}
		}
		return metrics.MetricsEntity{
			ID:       f.ID,
			Name:     f.Endpoint.Name, // forge instances are named after their endpoint
			Type:     string(params.ForgeEntityTypeInstance),
			Endpoint: f.Endpoint.Name,
			Healthy:  f.PoolManagerStatus.IsRunning,
		}
	}
	return metrics.MetricsEntity{}
}
