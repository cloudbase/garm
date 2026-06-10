package editor

import (
	"fmt"
	"strconv"
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

// Editor is a terminal text editor with optional vim-style keybindings.
type Editor struct {
	app       *tview.Application
	pages     *tview.Pages
	layout    *tview.Grid
	frame     *editorFrame
	editor    *tview.TextArea
	gutter    *lineNumberGutter
	footer    *tview.TextView
	searchBar *tview.InputField
	quitModal *tview.Modal

	initialContent string
	result         string
	saved          bool

	syntax      string
	highlighter *syntaxHighlighter

	useVim     bool
	vimMode    VimMode
	pendingKey rune // first key of a two-key vim command (gg, dd)
	searchTerm string
}

// NewEditor creates a new editor instance
func NewEditor() *Editor {
	return &Editor{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
	}
}

// SetSyntax sets the language used for syntax highlighting (a chroma lexer
// name, e.g. "bash" or "powershell"). An empty or unknown language disables
// highlighting. It must be called before EditText.
func (e *Editor) SetSyntax(language string) {
	e.syntax = language
}

// SetVimMode enables or disables vim modal editing
func (e *Editor) SetVimMode(enabled bool) {
	e.useVim = enabled
	e.vimMode = VimNormal
	e.pendingKey = 0
}

// EditText launches the editor with initial content. It returns the edited
// text and whether the user saved the changes.
func (e *Editor) EditText(initialContent string) (string, bool, error) {
	e.setup(initialContent)
	if err := e.app.SetRoot(e.pages, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
		return "", false, err
	}
	return e.result, e.saved, nil
}

// setup builds the editor UI for the given content.
func (e *Editor) setup(initialContent string) {
	e.initialContent = initialContent
	e.result = initialContent
	e.saved = false

	e.editor = tview.NewTextArea().
		SetText(initialContent, false).
		SetWrap(false)
	e.editor.SetInputCapture(e.handleEditorKey)
	// A mouse click back into the editor closes the search bar if it is open.
	e.editor.SetFocusFunc(e.closeSearchBar)
	e.editor.SetChangedFunc(e.onTextChanged)

	e.highlighter = newSyntaxHighlighter(e.syntax)
	e.gutter = newLineNumberGutter(e.editor)
	e.frame = &editorFrame{
		Flex: tview.NewFlex().
			AddItem(e.gutter, 0, 0, false).
			AddItem(e.editor, 0, 1, true),
		editor:      e.editor,
		gutter:      e.gutter,
		highlighter: e.highlighter,
	}
	e.frame.SetBorder(true).SetTitleAlign(tview.AlignCenter)
	e.onTextChanged()

	e.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	e.layout = tview.NewGrid().
		SetRows(0, 1).
		AddItem(e.frame, 0, 0, 1, 1, 0, 0, true).
		AddItem(e.footer, 1, 0, 1, 1, 0, 0, false)

	e.pages.AddPage("main", e.layout, true, true)
	e.createHelpPages()
	e.createQuitModal()
	e.refreshChrome()

	// By default tview stops the application on Ctrl+C, which would silently
	// discard all changes. Route it through the quit confirmation instead.
	e.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			e.requestQuit()
			return nil
		}
		return event
	})
}

// Gutter colors: a subtly lighter background sets the line number strip apart
// from the text area, dimmed numbers keep the focus on the text, and the
// cursor line is shown in the primary text color. On terminals without 256
// colors (e.g. TERM=xterm-color) the background would map to black, so the
// strip falls back to vim-style colored numbers instead.
var (
	gutterBackground     = tcell.Color238 // #444444, clearly lighter than dark terminal themes
	gutterNumberColor    = tcell.ColorSilver
	gutterFallbackNumber = tcell.ColorOlive // ANSI dark yellow, vim's default LineNr
)

// lineNumberGutter renders line numbers next to the text area. It reads the
// text area's scroll offset and cursor at draw time, so it stays in sync with
// scrolling without any event plumbing. Screen rows equal text lines because
// wrapping is disabled.
type lineNumberGutter struct {
	*tview.Box
	editor *tview.TextArea
	lines  int
}

func newLineNumberGutter(editor *tview.TextArea) *lineNumberGutter {
	return &lineNumberGutter{Box: tview.NewBox(), editor: editor}
}

// width returns the columns needed for the highest line number plus a blank
// separator column on each side.
func (g *lineNumberGutter) width() int {
	return len(strconv.Itoa(max(g.lines, 1))) + 2
}

func (g *lineNumberGutter) Draw(screen tcell.Screen) {
	g.DrawForSubclass(screen, g)
	x, y, width, height := g.GetInnerRect()
	rowOffset, _ := g.editor.GetOffset()
	cursorRow, _, _, _ := g.editor.GetCursor()
	base := tcell.StyleDefault.Foreground(gutterFallbackNumber)
	if screen.Colors() >= 256 {
		base = tcell.StyleDefault.Background(gutterBackground).Foreground(gutterNumberColor)
	}
	for i := range height {
		line := rowOffset + i + 1
		style := base
		if line-1 == cursorRow {
			style = style.Foreground(tview.Styles.PrimaryTextColor)
		}
		var number string
		if line <= g.lines {
			number = strconv.Itoa(line)
		}
		// The number right-aligned with one separator column, the rest of the
		// strip (including rows past the end of the text) just background.
		text := fmt.Sprintf("%*s ", width-1, number)
		for j, r := range text {
			screen.SetContent(x+j, y+i, r, nil, style)
		}
	}
}

// editorFrame is the bordered container holding the gutter and the text area.
// Flex defers drawing the focused item (the text area) until last, and
// TextArea.Draw may still adjust the scroll offset while drawing, so the
// syntax recoloring pass and the gutter run afterwards to pick up the settled
// offset.
type editorFrame struct {
	*tview.Flex
	editor      *tview.TextArea
	gutter      *lineNumberGutter
	highlighter *syntaxHighlighter // nil when highlighting is disabled
}

func (f *editorFrame) Draw(screen tcell.Screen) {
	f.Flex.Draw(screen)
	if f.highlighter != nil {
		f.highlighter.recolor(screen, f.editor)
	}
	f.gutter.Draw(screen)
}

// onTextChanged recounts the lines for the gutter (resizing it to fit the
// digits of the highest line number) and re-tokenizes the text for syntax
// highlighting.
func (e *Editor) onTextChanged() {
	text := e.editor.GetText()
	e.gutter.lines = strings.Count(text, "\n") + 1
	e.frame.ResizeItem(e.gutter, e.gutter.width(), 0)
	if e.highlighter != nil {
		e.highlighter.update(text)
	}
}

// handleEditorKey is the input capture of the text area. Global shortcuts are
// handled first; everything else goes through the vim handler when vim mode
// is enabled.
func (e *Editor) handleEditorKey(event *tcell.EventKey) *tcell.EventKey {
	alt := event.Modifiers()&tcell.ModAlt != 0
	switch {
	case event.Key() == tcell.KeyCtrlS:
		e.result = e.editor.GetText()
		e.saved = true
		e.app.Stop()
		return nil
	case event.Key() == tcell.KeyCtrlQ:
		e.requestQuit()
		return nil
	case event.Key() == tcell.KeyCtrlUnderscore, alt && event.Rune() == 'h':
		e.pages.SwitchToPage("help")
		return nil
	case alt && event.Rune() == 'v':
		e.toggleVimMode()
		return nil
	case alt && event.Rune() == 'c':
		// Forward as Ctrl+Q, the text area's native "copy selection" binding
		// (the Ctrl+Q key itself is taken by "quit" above).
		return tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
	}

	if !e.useVim {
		return event
	}
	if e.vimMode == VimInsert {
		if event.Key() == tcell.KeyEscape {
			e.setVimState(VimNormal)
			return nil
		}
		return event
	}
	return e.handleVimNormalKey(event)
}

// handleVimNormalKey handles input in vim normal mode.
func (e *Editor) handleVimNormalKey(event *tcell.EventKey) *tcell.EventKey {
	pending := e.pendingKey
	e.pendingKey = 0

	if event.Key() != tcell.KeyRune {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown,
			tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn,
			tcell.KeyDelete, tcell.KeyCtrlZ, tcell.KeyCtrlY:
			return event
		case tcell.KeyEnter:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
		}
		// Block everything else so the text cannot be modified in normal mode.
		return nil
	}
	return e.handleVimNormalRune(pending, event.Rune())
}

// handleVimNormalRune handles character commands in vim normal mode. The
// pending rune is the first key of a two-key command ("gg", "dd"), or 0.
func (e *Editor) handleVimNormalRune(pending, r rune) *tcell.EventKey {
	switch {
	case pending == 'g' && r == 'g':
		e.moveTo(0)
	case pending == 'd' && r == 'd':
		e.deleteCurrentLine()
	case r == 'g', r == 'd':
		e.pendingKey = r
	case r == 'i':
		e.setVimState(VimInsert)
	case r == 'a':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case r == 'A':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case r == 'I':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case r == 'o':
		e.setVimState(VimInsert)
		e.forwardKeys(tcell.KeyEnd, tcell.KeyEnter)
	case r == 'O':
		e.setVimState(VimInsert)
		e.forwardKeys(tcell.KeyHome, tcell.KeyEnter, tcell.KeyUp)
	case r == 'h':
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case r == 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case r == 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case r == 'l':
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case r == '0':
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case r == '$':
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case r == 'w':
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl)
	case r == 'b':
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl)
	case r == 'G':
		e.moveTo(e.editor.GetTextLength())
	case r == 'x':
		return tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone)
	case r == 'X':
		return tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone)
	case r == 'u':
		return tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone)
	case r == '/':
		e.openSearchBar()
	case r == 'n':
		e.findNext()
	case r == 'N':
		e.findPrevious()
	}
	// Block all other text input in normal mode.
	return nil
}

// forwardKeys feeds key events to the text area. The events pass through the
// input capture again, so callers must make sure the current mode lets them
// through.
func (e *Editor) forwardKeys(keys ...tcell.Key) {
	handler := e.editor.InputHandler()
	for _, key := range keys {
		handler(tcell.NewEventKey(key, 0, tcell.ModNone), nil)
	}
}

// setVimState switches the vim mode and updates title and footer.
func (e *Editor) setVimState(mode VimMode) {
	e.vimMode = mode
	e.pendingKey = 0
	e.refreshChrome()
}

// toggleVimMode toggles vim mode on/off and updates the UI.
func (e *Editor) toggleVimMode() {
	e.useVim = !e.useVim
	e.setVimState(VimNormal)
	// The navigation help includes a vim section only while vim mode is on.
	e.pages.RemovePage("help")
	e.createHelpPages()
}

// requestQuit exits the editor, asking for confirmation first if there are
// unsaved changes.
func (e *Editor) requestQuit() {
	if e.editor.GetText() == e.initialContent {
		e.app.Stop()
		return
	}
	e.quitModal.SetFocus(1) // default to "Cancel"
	e.pages.ShowPage("confirm-quit")
	e.app.SetFocus(e.quitModal)
}

// createQuitModal builds the unsaved-changes confirmation dialog.
func (e *Editor) createQuitModal() {
	cancel := func() {
		e.pages.SwitchToPage("main")
		e.app.SetFocus(e.editor)
	}
	e.quitModal = tview.NewModal().
		SetText("You have unsaved changes. Quit without saving?").
		AddButtons([]string{"Quit without saving", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, _ string) {
			if buttonIndex == 0 {
				e.app.Stop()
				return
			}
			cancel()
		})
	e.quitModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			cancel()
			return nil
		}
		return event
	})
	e.pages.AddPage("confirm-quit", e.quitModal, true, false)
}

// openSearchBar replaces the footer with a vim-style search input.
func (e *Editor) openSearchBar() {
	bar := tview.NewInputField().SetLabel("/")
	bar.SetDoneFunc(func(key tcell.Key) {
		e.closeSearchBar()
		if key == tcell.KeyEnter {
			if term := bar.GetText(); term != "" {
				e.searchTerm = term
				e.findNext()
			}
		}
	})
	e.searchBar = bar
	e.setVimState(VimSearch)
	e.layout.RemoveItem(e.footer)
	e.layout.AddItem(bar, 1, 0, 1, 1, 0, 0, true)
	e.app.SetFocus(bar)
}

// closeSearchBar restores the footer and returns focus to the editor. It is a
// no-op when the search bar is not open.
func (e *Editor) closeSearchBar() {
	if e.searchBar == nil {
		return
	}
	bar := e.searchBar
	e.searchBar = nil
	e.layout.RemoveItem(bar)
	e.layout.AddItem(e.footer, 1, 0, 1, 1, 0, 0, false)
	e.setVimState(VimNormal)
	e.app.SetFocus(e.editor)
}

// findNext selects the next occurrence of the search term, wrapping around at
// the end of the text.
func (e *Editor) findNext() {
	if e.searchTerm == "" {
		return
	}
	text := e.editor.GetText()
	_, selStart, selEnd := e.editor.GetSelection()
	from := selEnd
	if from == selStart {
		from++ // No active selection: skip the character under the cursor.
	}
	if from < len(text) {
		if idx := strings.Index(text[from:], e.searchTerm); idx >= 0 {
			e.selectMatch(from + idx)
			return
		}
	}
	if idx := strings.Index(text, e.searchTerm); idx >= 0 {
		e.selectMatch(idx)
	}
}

// findPrevious selects the previous occurrence of the search term, wrapping
// around at the beginning of the text.
func (e *Editor) findPrevious() {
	if e.searchTerm == "" {
		return
	}
	text := e.editor.GetText()
	_, selStart, _ := e.editor.GetSelection()
	if idx := strings.LastIndex(text[:selStart], e.searchTerm); idx >= 0 {
		e.selectMatch(idx)
		return
	}
	if idx := strings.LastIndex(text, e.searchTerm); idx >= 0 {
		e.selectMatch(idx)
	}
}

// selectMatch highlights the match at the given byte offset and scrolls it
// into view.
func (e *Editor) selectMatch(pos int) {
	e.materializeThrough(pos)
	e.editor.Select(pos, pos+len(e.searchTerm))
	e.scrollCursorIntoView()
}

// moveTo places the cursor at the given byte offset.
func (e *Editor) moveTo(pos int) {
	e.materializeThrough(pos)
	e.editor.Select(pos, pos)
	e.scrollCursorIntoView()
}

// materializeThrough makes the text area build its internal line index at
// least up to the line containing the given byte offset. TextArea.Select
// mislocates offsets that lie beyond the lines materialized so far (it
// attributes the entire remaining text to the last known line, yielding a
// bogus cursor column far off screen), so we page the cursor down first —
// keyboard navigation extends the index correctly. Rows equal text lines
// here because wrapping is disabled.
func (e *Editor) materializeThrough(pos int) {
	targetRow := strings.Count(e.editor.GetText()[:pos], "\n")
	handler := e.editor.InputHandler()
	lastRow := -1
	for {
		row, _, _, _ := e.editor.GetCursor()
		if row >= targetRow || row == lastRow {
			return
		}
		lastRow = row
		handler(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone), nil)
	}
}

// scrollCursorIntoView adjusts the scroll offsets so the cursor is visible.
// The text area only scrolls automatically on keyboard navigation, not when
// the cursor is moved programmatically via Select.
func (e *Editor) scrollCursorIntoView() {
	row, column, _, _ := e.editor.GetCursor()
	rowOffset, columnOffset := e.editor.GetOffset()
	_, _, width, height := e.editor.GetInnerRect()
	switch {
	case row < rowOffset:
		rowOffset = row
	case row >= rowOffset+height:
		rowOffset = row - height + 1
	}
	switch {
	case column < columnOffset:
		columnOffset = column
	case column >= columnOffset+width:
		columnOffset = column - width + 1
	}
	e.editor.SetOffset(rowOffset, columnOffset)
}

// deleteCurrentLine implements vim's "dd".
func (e *Editor) deleteCurrentLine() {
	text := e.editor.GetText()
	_, start, _ := e.editor.GetSelection()
	if start > len(text) {
		start = len(text)
	}
	lineStart := strings.LastIndexByte(text[:start], '\n') + 1
	lineEnd := len(text)
	if idx := strings.IndexByte(text[start:], '\n'); idx >= 0 {
		lineEnd = start + idx + 1
	}
	e.editor.Replace(lineStart, lineEnd, "")
}

// refreshChrome updates the editor title and footer for the current mode.
func (e *Editor) refreshChrome() {
	title := "Text Editor"
	footer := "[yellow]Ctrl+S[white]: Save & Exit | [yellow]Ctrl+Q[white]: Quit | [yellow]Alt+H[white]: Help | [yellow]Alt+V[white]: Toggle VIM"
	if e.useVim {
		var mode string
		switch e.vimMode {
		case VimInsert:
			mode = "INSERT"
			footer += " | [green]INSERT[white]: Esc for normal mode"
		case VimSearch:
			mode = "SEARCH"
		default:
			mode = "NORMAL"
			footer += " | [green]NORMAL[white]: i insert, h/j/k/l move, gg/G, dd, x del, u undo, / n N search"
		}
		title = fmt.Sprintf("Text Editor (VIM - %s)", mode)
	}
	e.frame.SetTitle(title)
	e.footer.SetText(footer)
}

// navigationHelp returns the navigation help text, including the vim section
// when vim mode is enabled.
func (e *Editor) navigationHelp() string {
	text := `[green]Navigation[white]

[yellow]Arrow Keys[white]: Move cursor around
[yellow]Ctrl-A, Home[white]: Move to beginning of line
[yellow]Ctrl-E, End[white]: Move to end of line
[yellow]Ctrl-F, Page Down[white]: Move down by one page
[yellow]Ctrl-B, Page Up[white]: Move up by one page
[yellow]Alt-Up/Down/Left/Right[white]: Scroll the page
[yellow]Alt-B, Ctrl-Left[white]: Move back by one word
[yellow]Alt-F, Ctrl-Right[white]: Move forward by one word`

	if e.useVim {
		text += `

[green]VIM Normal Mode[white]
[yellow]h/j/k/l or Arrow Keys[white]: Move left/down/up/right
[yellow]0/$ or Home/End[white]: Beginning/end of line
[yellow]w/b[white]: Word forward/backward
[yellow]gg/G[white]: Beginning/end of document
[yellow]i/a[white]: Insert mode at/after cursor
[yellow]I/A[white]: Insert mode at beginning/end of line
[yellow]o/O[white]: New line below/above + insert mode
[yellow]x/X[white]: Delete character right/left
[yellow]dd[white]: Delete current line
[yellow]u[white]: Undo
[yellow]/[white]: Search, then [yellow]n/N[white]: next/previous match
[yellow]Esc[white]: Return to normal mode`
	}

	return text + `

[blue]Press Enter for more help, Escape to return to editor[white]`
}

const editingHelp = `[green]Editing[white]

Type to enter text.
[yellow]Ctrl-H, Backspace[white]: Delete left character
[yellow]Ctrl-D, Delete[white]: Delete right character
[yellow]Ctrl-K[white]: Delete to end of line
[yellow]Ctrl-W[white]: Delete rest of word
[yellow]Ctrl-U[white]: Delete current line
[yellow]Ctrl-Z[white]: Undo
[yellow]Ctrl-Y[white]: Redo

[green]Selection & Clipboard[white]

Hold [yellow]Shift[white] + movement keys to select
[yellow]Ctrl-L[white]: Select entire text
[yellow]Alt-C[white]: Copy selection
[yellow]Ctrl-X[white]: Cut selection
[yellow]Ctrl-V[white]: Paste

[blue]Press Enter for more help, Escape to return to editor[white]`

const commandsHelp = `[green]Editor Commands[white]

[yellow]Ctrl-S[white]: Save changes and exit editor
[yellow]Ctrl-Q, Ctrl-C[white]: Quit (asks for confirmation on unsaved changes)
[yellow]Alt+H[white]: Show this help
[yellow]Alt+V[white]: Toggle VIM mode on/off

[green]Mouse Support[white]

Click to position cursor
Drag to select text
Double-click to select word
Mouse wheel to scroll

[blue]Press Enter to cycle back, Escape to return to editor[white]`

// createHelpPages creates the help system with multiple pages.
func (e *Editor) createHelpPages() {
	helpContent := []struct {
		title string
		text  string
	}{
		{"Help - Navigation", e.navigationHelp()},
		{"Help - Editing", editingHelp},
		{"Help - Commands", commandsHelp},
	}

	helpPages := tview.NewPages()
	current := 0
	cycle := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.pages.SwitchToPage("main")
			return nil
		case tcell.KeyEnter:
			current = (current + 1) % len(helpContent)
			helpPages.SwitchToPage(helpContent[current].title)
			return nil
		}
		return event
	}

	for i, page := range helpContent {
		view := tview.NewTextView().SetDynamicColors(true).SetText(page.text)
		view.SetBorder(true)
		view.SetTitle(page.title)
		view.SetInputCapture(cycle)
		helpPages.AddPage(page.title, view, true, i == 0)
	}

	// Center the help dialog.
	helpGrid := tview.NewGrid().
		SetColumns(0, 80, 0).
		SetRows(0, 25, 0).
		AddItem(helpPages, 1, 1, 1, 1, 0, 0, true)

	e.pages.AddPage("help", helpGrid, true, false)
}
