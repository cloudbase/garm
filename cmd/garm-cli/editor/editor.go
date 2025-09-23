package editor

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VimMode represents the current vim editing mode
type VimMode int

const (
	VimNormal VimMode = iota
	VimInsert
	VimSearch
)

// Editor represents a text editor that can be launched with initial content
type Editor struct {
	app               *tview.Application
	pages             *tview.Pages
	editor            *tview.TextArea
	footer            *tview.TextView
	result            string
	saved             bool
	useVim            bool
	vimMode           VimMode
	waitingForSecondG bool
	searchTerm        string
	searchInput       *tview.InputField
}

// NewEditor creates a new editor instance
func NewEditor() *Editor {
	e := &Editor{
		app:     tview.NewApplication(),
		pages:   tview.NewPages(),
		useVim:  false,
		vimMode: VimNormal,
	}
	return e
}

// SetVimMode enables or disables vim modal editing
func (e *Editor) SetVimMode(enabled bool) {
	e.useVim = enabled
	if enabled {
		e.vimMode = VimNormal
	}
}

// toggleVimMode toggles vim mode on/off and updates the UI
func (e *Editor) toggleVimMode() {
	e.useVim = !e.useVim
	if e.useVim {
		e.vimMode = VimNormal
	}
	e.updateTitle()
	e.updateFooter()
}

// updateTitle updates the editor title to show current vim mode
func (e *Editor) updateTitle() {
	if e.useVim {
		var modeStr string
		switch e.vimMode {
		case VimNormal:
			modeStr = "NORMAL"
		case VimInsert:
			modeStr = "INSERT"
		case VimSearch:
			modeStr = "SEARCH"
		}
		e.editor.SetTitle(fmt.Sprintf("Text Editor (VIM - %s)", modeStr))
	} else {
		e.editor.SetTitle("Text Editor")
	}
}

// resetGCommand resets the gg command state
func (e *Editor) resetGCommand() {
	e.waitingForSecondG = false
}

// updateFooter updates the footer text based on current mode
func (e *Editor) updateFooter() {
	if e.footer == nil {
		return
	}

	footerText := "[yellow]Ctrl+S[white]: Save & Exit | [yellow]Ctrl+Q[white]: Quit | [yellow]Alt+H[white]: Help | [yellow]Alt+V[white]: Toggle VIM"
	if e.useVim {
		switch e.vimMode {
		case VimNormal:
			footerText += " | [green]VIM NORMAL[white]: i/a/o for insert, h/j/k/l/arrows nav, G/gg, / search, n/N find next/prev, x del"
		case VimInsert:
			footerText += " | [green]VIM INSERT[white]: Esc for normal mode"
		case VimSearch:
			footerText += " | [green]VIM SEARCH[white]: Enter search term, Enter to find, Esc to cancel"
		}
	}
	e.footer.SetText(footerText)
}

// handleVimInput manages vim modal input handling
func (e *Editor) handleVimInput(event *tcell.EventKey) *tcell.EventKey {
	switch e.vimMode {
	case VimNormal:
		return e.handleVimNormalMode(event)
	case VimInsert:
		return e.handleVimInsertMode(event)
	case VimSearch:
		return e.handleVimSearchMode(event)
	}
	return event
}

// handleVimNormalMode handles input in vim normal mode
func (e *Editor) handleVimNormalMode(event *tcell.EventKey) *tcell.EventKey {
	// Handle global commands first
	if result := e.handleGlobalCommands(event); result != event {
		return result
	}

	// Handle vim character-based commands
	if result := e.handleVimCharCommands(event); result != event {
		return result
	}

	// Handle key-based navigation
	return e.handleKeyNavigation(event)
}

// handleGlobalCommands handles global commands available in all modes
func (e *Editor) handleGlobalCommands(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyCtrlS:
		e.result = e.editor.GetText()
		e.saved = true
		e.app.Stop()
		return nil
	case event.Key() == tcell.KeyCtrlQ:
		e.app.Stop()
		return nil
	case event.Rune() == 'h' && event.Modifiers()&tcell.ModAlt != 0:
		e.pages.SwitchToPage("help")
		return nil
	case event.Key() == tcell.KeyCtrlUnderscore:
		e.pages.SwitchToPage("help")
		return nil
	}
	return event // Continue processing
}

// handleVimCharCommands handles vim character-based commands
func (e *Editor) handleVimCharCommands(event *tcell.EventKey) *tcell.EventKey {
	// Handle mode switching commands
	if result := e.handleModeSwitching(event); result != event {
		return result
	}

	// Handle navigation commands
	if result := e.handleCharNavigation(event); result != event {
		return result
	}

	// Handle editing commands
	return e.handleEditingCommands(event)
}

// handleModeSwitching handles commands that switch vim modes
func (e *Editor) handleModeSwitching(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'i':
		e.resetGCommand()
		e.enterInsertMode()
		return nil
	case 'a':
		e.resetGCommand()
		e.enterInsertMode()
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case 'A':
		e.resetGCommand()
		e.enterInsertMode()
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case 'I':
		e.resetGCommand()
		e.enterInsertMode()
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case 'o':
		e.resetGCommand()
		e.enterInsertMode()
		e.insertNewLineBelow()
		return nil
	case 'O':
		e.resetGCommand()
		e.enterInsertMode()
		e.insertNewLineAbove()
		return nil
	}
	return event // Continue processing
}

// enterInsertMode switches to insert mode
func (e *Editor) enterInsertMode() {
	e.vimMode = VimInsert
	e.updateTitle()
	e.updateFooter()
}

// exitInsertMode switches to normal mode
func (e *Editor) exitInsertMode() {
	if e.vimMode != VimInsert {
		return
	}

	e.vimMode = VimNormal
	e.updateTitle()
	e.updateFooter()
}

// handleCharNavigation handles character-based navigation commands
func (e *Editor) handleCharNavigation(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'h':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case 'j':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case 'l':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case '0':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case '$':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case 'w':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl)
	case 'b':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl)
	case 'G':
		e.resetGCommand()
		e.goToEnd()
		return nil
	case 'g':
		return e.handleGCommand(event)
	}
	return event // Continue processing
}

// handleEditingCommands handles editing and search commands
func (e *Editor) handleEditingCommands(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'x':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone)
	case 'X':
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone)
	case 'd':
		e.resetGCommand()
		return e.handleDeleteCommand(event)
	case '/':
		e.resetGCommand()
		e.startSearch()
		return nil
	case 'n':
		e.resetGCommand()
		e.findNext()
		return nil
	case 'N':
		e.resetGCommand()
		e.findPrevious()
		return nil
	}

	return event // Continue processing
}

// handleKeyNavigation handles key-based navigation (arrow keys, etc.)
func (e *Editor) handleKeyNavigation(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		e.resetGCommand()
		return nil
	case tcell.KeyLeft:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case tcell.KeyDown:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case tcell.KeyUp:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case tcell.KeyRight:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case tcell.KeyHome:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case tcell.KeyEnd:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case tcell.KeyPgUp:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
	case tcell.KeyPgDn:
		e.resetGCommand()
		return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
	}

	// Block all other text input in normal mode
	return nil
}

// handleVimInsertMode handles input in vim insert mode
func (e *Editor) handleVimInsertMode(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		e.exitInsertMode()
		return nil
	case tcell.KeyCtrlS:
		e.result = e.editor.GetText()
		e.saved = true
		e.app.Stop()
		return nil
	case tcell.KeyCtrlQ:
		e.app.Stop()
		return nil
	}

	// Pass through all other keys in insert mode
	return event
}

// Helper functions for vim operations
func (e *Editor) insertNewLineBelow() {
	// Move to end of current line and insert newline
	e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), nil)
	e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), nil)
}

func (e *Editor) insertNewLineAbove() {
	// Move to beginning of line, insert newline, move up
	e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone), nil)
	e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), nil)
	e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
}

func (e *Editor) goToEnd() {
	// Go to end of document by setting text with cursor at end
	text := e.editor.GetText()
	e.editor.SetText(text, true) // true = cursor at end
}

func (e *Editor) goToBeginning() {
	// Go to beginning of document by setting text with cursor at beginning
	text := e.editor.GetText()
	e.editor.SetText(text, false) // false = cursor at beginning
}

func (e *Editor) handleGCommand(_ *tcell.EventKey) *tcell.EventKey {
	// For now, let's implement a simpler approach
	// In vim, single 'g' usually requires a second command
	// But let's make gg work by tracking the state in the editor instance
	if e.waitingForSecondG {
		// This is the second 'g' - execute gg (go to beginning)
		e.waitingForSecondG = false
		e.goToBeginning()
		return nil
	}
	// First 'g' press - wait for second one
	e.waitingForSecondG = true
	return nil
}

func (e *Editor) handleDeleteCommand(_ *tcell.EventKey) *tcell.EventKey {
	// For now, just implement x (delete character)
	// A full implementation would handle dd, dw, etc.
	return tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone)
}

// handleVimSearchMode handles input in vim search mode
func (e *Editor) handleVimSearchMode(event *tcell.EventKey) *tcell.EventKey {
	// Search mode is handled by the modal dialog, so this shouldn't be called
	// But keep it for consistency
	switch event.Key() {
	case tcell.KeyEscape:
		e.cancelSearch()
		return nil
	case tcell.KeyCtrlS:
		e.result = e.editor.GetText()
		e.saved = true
		e.app.Stop()
		return nil
	case tcell.KeyCtrlQ:
		e.app.Stop()
		return nil
	}

	return event
}

// startSearch enters search mode and shows search input
func (e *Editor) startSearch() {
	e.vimMode = VimSearch
	e.updateTitle()
	e.updateFooter()
	e.showSearchInput()
}

// cancelSearch exits search mode without searching
func (e *Editor) cancelSearch() {
	e.vimMode = VimNormal
	e.updateTitle()
	e.updateFooter()
	e.hideSearchInput()
}

// findNext finds the next occurrence of the search term
func (e *Editor) findNext() {
	if e.searchTerm == "" {
		return
	}

	text := e.editor.GetText()
	fromRow, fromCol, _, _ := e.editor.GetCursor()

	// Convert current position to linear position
	lines := strings.Split(text, "\n")
	currentPos := 0
	for i := 0; i < fromRow && i < len(lines); i++ {
		currentPos += len(lines[i]) + 1 // +1 for newline
	}
	currentPos += fromCol

	// Search from current position + 1
	searchFrom := currentPos + 1
	if searchFrom < len(text) {
		index := strings.Index(text[searchFrom:], e.searchTerm)
		if index != -1 {
			e.goToPosition(searchFrom + index)
			return
		}
	}

	// Wrap around search from beginning
	index := strings.Index(text, e.searchTerm)
	if index != -1 && index < currentPos {
		e.goToPosition(index)
	}
}

// findPrevious finds the previous occurrence of the search term
func (e *Editor) findPrevious() {
	if e.searchTerm == "" {
		return
	}

	text := e.editor.GetText()
	fromRow, fromCol, _, _ := e.editor.GetCursor()

	// Convert current position to linear position
	lines := strings.Split(text, "\n")
	currentPos := 0
	for i := 0; i < fromRow && i < len(lines); i++ {
		currentPos += len(lines[i]) + 1 // +1 for newline
	}
	currentPos += fromCol

	// Search backwards from current position
	if currentPos > 0 {
		index := strings.LastIndex(text[:currentPos], e.searchTerm)
		if index != -1 {
			e.goToPosition(index)
			return
		}
	}

	// Wrap around search from end
	index := strings.LastIndex(text, e.searchTerm)
	if index != -1 && index > currentPos {
		e.goToPosition(index)
	}
}

// goToPosition moves cursor to a specific character position in the text
func (e *Editor) goToPosition(pos int) {
	text := e.editor.GetText()
	if pos >= len(text) {
		return
	}

	// Simple approach: reset text with cursor at beginning, then move right
	e.editor.SetText(text, false)

	// Move cursor to the right position by simulating key presses
	for range pos {
		e.editor.InputHandler()(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), nil)
	}
}

// showSearchInput displays the search input field
func (e *Editor) showSearchInput() {
	// Create search input field with proper handling
	e.searchInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(30).
		SetText("")

	// Set input capture for the search input
	e.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			// Get search term and perform search
			e.searchTerm = e.searchInput.GetText()
			if e.searchTerm != "" {
				e.findNext()
			}
			e.vimMode = VimNormal
			e.updateTitle()
			e.updateFooter()
			e.pages.RemovePage("search")
			e.pages.SwitchToPage("main")
			e.app.SetFocus(e.editor)
			return nil
		} else if event.Key() == tcell.KeyEscape {
			// Cancel search
			e.vimMode = VimNormal
			e.updateTitle()
			e.updateFooter()
			e.pages.RemovePage("search")
			e.pages.SwitchToPage("main")
			e.app.SetFocus(e.editor)
			return nil
		}
		return event
	})

	// Create a container with border and title
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(e.searchInput, 1, 1, true).
		AddItem(nil, 0, 1, false)

	container := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(flex, 40, 1, true).
		AddItem(nil, 0, 1, false)

	// Create bordered frame
	frame := tview.NewFrame(container).
		SetBorders(1, 1, 1, 1, 2, 2).
		AddText("Search", true, tview.AlignCenter, tcell.ColorWhite)

	// Add to pages and switch
	e.pages.AddPage("search", frame, true, true)
	e.app.SetFocus(e.searchInput)
}

// hideSearchInput hides the search input field
func (e *Editor) hideSearchInput() {
	e.pages.RemovePage("search")
	e.pages.SwitchToPage("main")
	e.app.SetFocus(e.editor)
}

// EditText launches the editor with initial content and returns the edited text
func (e *Editor) EditText(initialContent string) (string, bool, error) {
	e.result = initialContent
	e.saved = false

	e.editor = tview.NewTextArea().
		SetText(initialContent, false).
		SetWrap(false)

	e.editor.SetBorder(true).
		SetTitleAlign(tview.AlignCenter)

	e.updateTitle()

	e.editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle global commands first (available in all modes)
		switch {
		case event.Key() == tcell.KeyCtrlS:
			e.result = e.editor.GetText()
			e.saved = true
			e.app.Stop()
			return nil
		case event.Key() == tcell.KeyCtrlQ:
			e.app.Stop()
			return nil
		case event.Rune() == 'h' && event.Modifiers()&tcell.ModAlt != 0:
			e.pages.SwitchToPage("help")
			return nil
		case event.Key() == tcell.KeyCtrlUnderscore:
			e.pages.SwitchToPage("help")
			return nil
		case event.Rune() == 'v' && event.Modifiers()&tcell.ModAlt != 0:
			// Toggle vim mode
			e.toggleVimMode()
			return nil
		}

		// Handle vim keybindings if enabled
		if e.useVim {
			return e.handleVimInput(event)
		}

		return event
	})

	// Create footer with shortcuts
	e.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	e.updateFooter()

	// Create layout with editor and footer
	grid := tview.NewGrid().
		SetRows(0, 1).
		AddItem(e.editor, 0, 0, 1, 1, 0, 0, true).
		AddItem(e.footer, 1, 0, 1, 1, 0, 0, false)

	// Create help pages
	e.createHelpPages()

	// Add main editor page
	e.pages.AddPage("main", grid, true, true)

	e.app.SetRoot(e.pages, true)

	// Run the editor
	err := e.app.Run()
	if err != nil {
		return "", false, err
	}

	return e.result, e.saved, nil
}

// createHelpPages creates the help system with multiple pages
func (e *Editor) createHelpPages() {
	// Create three separate help pages
	help1 := tview.NewTextView()
	help1.SetDynamicColors(true)
	navText := `[green]Navigation[white]

[yellow]Arrow Keys[white]: Move cursor around
[yellow]Ctrl-A, Home[white]: Move to beginning of line
[yellow]Ctrl-E, End[white]: Move to end of line
[yellow]Ctrl-F, Page Down[white]: Move down by one page
[yellow]Ctrl-B, Page Up[white]: Move up by one page
[yellow]Alt-Up[white]: Scroll page up
[yellow]Alt-Down[white]: Scroll page down
[yellow]Alt-Left[white]: Scroll page left
[yellow]Alt-Right[white]: Scroll page right
[yellow]Alt-B, Ctrl-Left[white]: Move back by one word
[yellow]Alt-F, Ctrl-Right[white]: Move forward by one word`

	if e.useVim {
		navText += `

[green]VIM Navigation (Normal Mode)[white]
[yellow]h/j/k/l or Arrow Keys[white]: Move left/down/up/right
[yellow]0/$ or Home/End[white]: Beginning/end of line
[yellow]w/b[white]: Word forward/backward
[yellow]gg/G[white]: Beginning/end of document
[yellow]Page Up/Down[white]: Page navigation

[green]VIM Mode Switching[white]
[yellow]i[white]: Insert mode at cursor
[yellow]a[white]: Insert mode after cursor
[yellow]A[white]: Insert mode at end of line
[yellow]I[white]: Insert mode at beginning of line
[yellow]o[white]: New line below + insert mode
[yellow]O[white]: New line above + insert mode

[green]VIM Editing[white]
[yellow]x/X[white]: Delete character right/left
[yellow]Esc[white]: Return to normal mode
[yellow]Alt+V[white]: Toggle VIM mode on/off

[green]VIM Search[white]
[yellow]/[white]: Search for text (opens dialog)
[yellow]n[white]: Find next occurrence
[yellow]N[white]: Find previous occurrence`
	}

	navText += `

[blue]Press Enter for more help, Escape to return to editor[white]`

	help1.SetText(navText)
	help1.SetBorder(true)
	help1.SetTitle("Help - Navigation")

	help2 := tview.NewTextView()
	help2.SetDynamicColors(true)
	help2.SetText(`[green]Editing[white]

Type to enter text.
[yellow]Ctrl-H, Backspace[white]: Delete left character
[yellow]Ctrl-D, Delete[white]: Delete right character
[yellow]Ctrl-K[white]: Delete to end of line
[yellow]Ctrl-W[white]: Delete rest of word
[yellow]Ctrl-U[white]: Delete current line

[green]Selection & Clipboard[white]

Hold [yellow]Shift[white] + movement keys to select
Double-click to select a word
[yellow]Ctrl-L[white]: Select entire text
[yellow]Ctrl-Q[white]: Copy selection
[yellow]Ctrl-X[white]: Cut selection
[yellow]Ctrl-V[white]: Paste

[blue]Press Enter for more help, Escape to return to editor[white]`)
	help2.SetBorder(true)
	help2.SetTitle("Help - Editing")

	help3 := tview.NewTextView()
	help3.SetDynamicColors(true)
	help3.SetText(`[green]Editor Commands[white]

[yellow]Ctrl-S[white]: Save changes and exit editor
[yellow]Ctrl-Q[white]: Quit without saving changes
[yellow]Alt+H[white]: Show this help
[yellow]Alt+V[white]: Toggle VIM mode on/off

[green]Mouse Support[white]

Click to position cursor
Drag to select text
Double-click to select word
Mouse wheel to scroll

[blue]Press Enter to cycle back, Escape to return to editor[white]`)
	help3.SetBorder(true)
	help3.SetTitle("Help - Commands")

	helpPages := tview.NewPages()
	helpPages.AddPage("help1", help1, true, true)
	helpPages.AddPage("help2", help2, true, false)
	helpPages.AddPage("help3", help3, true, false)

	currentHelpPage := 0
	helpPageNames := []string{"help1", "help2", "help3"}

	// Set input capture for all help pages
	for _, helpView := range []*tview.TextView{help1, help2, help3} {
		helpView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				e.pages.SwitchToPage("main")
				return nil
			} else if event.Key() == tcell.KeyEnter {
				currentHelpPage = (currentHelpPage + 1) % len(helpPageNames)
				helpPages.SwitchToPage(helpPageNames[currentHelpPage])
				return nil
			}
			return event
		})
	}

	// Center the help dialog
	helpGrid := tview.NewGrid().
		SetColumns(0, 80, 0).
		SetRows(0, 25, 0).
		AddItem(helpPages, 1, 1, 1, 1, 0, 0, true)

	e.pages.AddPage("help", helpGrid, true, false)
}
