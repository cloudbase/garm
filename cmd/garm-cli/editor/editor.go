package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

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
	screen    tcell.Screen // set when running on a real terminal; enables OSC 52
	pages     *tview.Pages
	layout    *tview.Grid
	frame     *editorFrame
	editor    *tview.TextArea
	gutter    *lineNumberGutter
	footer    *editorFooter
	bottomBar *tview.InputField // search ("/") or command (":") bar, nil when closed
	quitModal *tview.Modal

	title          string
	initialContent string // last saved baseline; quitting is silent when the text matches
	result         string
	saved          bool
	modified       bool
	discarded      bool // the user quit while unsaved changes existed

	// saveHandler, when set, persists the text without leaving the editor
	// (vim ":w", and Ctrl+S saves through it before exiting). When nil, the
	// caller persists the result returned by EditText.
	saveHandler func(string) error

	syntax      string
	highlighter *syntaxHighlighter
	lines       []string // current text split into lines, for gutter and search highlighting

	useVim     bool
	vimMode    VimMode
	pendingKey rune // first key of a two-key vim command (gg, dd, yy)

	searchTerm string
	searchRe   *regexp.Regexp // compiled term; case-insensitive when the term is all lowercase
	hlSearch   bool           // highlight all matches; cleared with Escape

	clipText     string // mirrors the text area clipboard; feeds OSC 52 and vim paste
	clipLinewise bool   // yy/dd yanks paste as whole lines

	statusMsg string // transient footer message (search results, :w outcome)
	statusErr bool

	forwardingCopy bool // lets a synthesized Ctrl+Q reach the text area's copy binding
}

// editorFooter renders the key hints, status message and cursor ruler. The
// text is composed at draw time: the grid draws the focused frame last, so a
// footer updated from within the frame's draw pass would always lag one
// frame behind the cursor.
type editorFooter struct {
	*tview.TextView
	owner *Editor
}

func (f *editorFooter) Draw(screen tcell.Screen) {
	f.SetText(f.owner.footerText())
	f.TextView.Draw(screen)
}

// NewEditor creates a new editor instance
func NewEditor() *Editor {
	return &Editor{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
		title: "Text Editor",
	}
}

// SetSyntax sets the language used for syntax highlighting (a chroma lexer
// name, e.g. "bash" or "powershell"). An empty or unknown language disables
// highlighting. It must be called before EditText.
func (e *Editor) SetSyntax(language string) {
	e.syntax = language
}

// SetTitle sets the frame title, e.g. the name of the template being edited.
// It must be called before EditText.
func (e *Editor) SetTitle(title string) {
	if title != "" {
		e.title = title
	}
}

// SetSaveHandler installs a function that persists the text in place. It
// enables vim's ":w" and routes Ctrl+S ("save & exit") through it.
func (e *Editor) SetSaveHandler(handler func(text string) error) {
	e.saveHandler = handler
}

// SetVimMode enables or disables vim modal editing
func (e *Editor) SetVimMode(enabled bool) {
	e.useVim = enabled
	e.vimMode = VimNormal
	e.pendingKey = 0
}

// VimEnabled reports whether vim keybindings are currently enabled. Callers
// can persist this as the user's preference after the editor exits.
func (e *Editor) VimEnabled() bool {
	return e.useVim
}

// DiscardedChanges reports whether the editor was closed while unsaved
// changes existed, letting callers distinguish "changes discarded" from "no
// changes made".
func (e *Editor) DiscardedChanges() bool {
	return e.discarded
}

// EditText launches the editor with initial content. It returns the edited
// text and whether the user saved the changes. With a save handler set,
// "saved" means the handler ran successfully at least once and the returned
// text is what it last received.
func (e *Editor) EditText(initialContent string) (string, bool, error) {
	e.setup(initialContent)
	// Create the screen ourselves so the clipboard hook can reach it: text
	// copied in the editor is offered to the system clipboard via OSC 52 on
	// terminals that support it. Any error is left for app.Run to surface.
	if screen, err := tcell.NewScreen(); err == nil {
		e.screen = screen
		e.app.SetScreen(screen)
	}
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
	e.modified = false

	e.editor = tview.NewTextArea().
		SetText(initialContent, false).
		SetWrap(false)
	e.editor.SetInputCapture(e.handleEditorKey)
	// A mouse click back into the editor closes the search/command bar if
	// one is open.
	e.editor.SetFocusFunc(e.closeBottomBar)
	e.editor.SetChangedFunc(e.onTextChanged)
	e.editor.SetClipboard(func(text string) {
		e.setClipText(text, false)
	}, func() string {
		return e.clipText
	})

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
	e.frame.afterDraw = e.afterFrameDraw
	e.frame.SetBorder(true).SetTitleAlign(tview.AlignCenter)
	e.onTextChanged()

	e.footer = &editorFooter{
		TextView: tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter),
		owner: e,
	}

	e.layout = tview.NewGrid().
		SetRows(0, 1).
		AddItem(e.frame, 0, 0, 1, 1, 0, 0, true).
		AddItem(e.footer, 1, 0, 1, 1, 0, 0, false)

	e.pages.AddPage("main", e.layout, true, true)
	e.createHelpPages()
	e.createQuitModal()
	e.refreshChrome()

	// By default tview stops the application on Ctrl+C, which would silently
	// discard all changes. With a selection active it copies instead — the
	// most ingrained "copy" chord there is — and otherwise it routes through
	// the quit confirmation.
	e.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if e.app.GetFocus() == e.editor {
				if text, _, _ := e.editor.GetSelection(); text != "" {
					e.copySelection()
					return nil
				}
			}
			e.requestQuit()
			return nil
		}
		return event
	})
}

// setClipText stores yanked/copied text and offers it to the system
// clipboard via OSC 52 when running on a real terminal.
func (e *Editor) setClipText(text string, linewise bool) {
	e.clipText = text
	e.clipLinewise = linewise
	if e.screen != nil {
		e.screen.SetClipboard([]byte(text))
	}
}

// copySelection feeds Ctrl+Q (the text area's native "copy selection"
// binding) to the text area. The event passes through handleEditorKey again,
// where the forwardingCopy flag keeps it from being treated as "quit".
func (e *Editor) copySelection() {
	e.forwardingCopy = true
	defer func() { e.forwardingCopy = false }()
	handler := e.editor.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone), nil)
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
	afterDraw   func(screen tcell.Screen)
}

func (f *editorFrame) Draw(screen tcell.Screen) {
	f.Flex.Draw(screen)
	if f.highlighter != nil {
		f.highlighter.recolor(screen, f.editor)
	}
	f.gutter.Draw(screen)
	if f.afterDraw != nil {
		f.afterDraw(screen)
	}
}

// afterFrameDraw runs once per frame after the text area has settled and
// paints the search-match highlights over it.
func (e *Editor) afterFrameDraw(screen tcell.Screen) {
	e.drawSearchHighlights(screen)
}

// drawSearchHighlights marks every visible search match. Cells that already
// carry a non-default background (the selection, and with it the current
// match) keep their style.
func (e *Editor) drawSearchHighlights(screen tcell.Screen) {
	if !e.hlSearch || e.searchRe == nil {
		return
	}
	x, y, width, height := e.editor.GetInnerRect()
	rowOffset, columnOffset := e.editor.GetOffset()
	defaultBg := tview.Styles.PrimitiveBackgroundColor
	for row := range height {
		lineIdx := rowOffset + row
		if lineIdx >= len(e.lines) {
			break
		}
		locs := e.searchRe.FindAllStringIndex(e.lines[lineIdx], -1)
		if len(locs) == 0 {
			continue
		}
		spans := make([]lineSpan, 0, len(locs))
		for _, loc := range locs {
			spans = append(spans, lineSpan{start: loc[0], end: loc[1]})
		}
		visitSpanCells(x, y+row, width, columnOffset, e.lines[lineIdx], spans,
			func(cx, cy int, _ lineSpan) {
				str, style, _ := screen.Get(cx, cy)
				if str == "" {
					return // continuation cell of a wide character
				}
				if _, bg, _ := style.Decompose(); bg != defaultBg && bg != tcell.ColorDefault {
					return
				}
				screen.Put(cx, cy, str, style.Background(tcell.ColorOlive).Foreground(tcell.ColorBlack))
			})
	}
}

// onTextChanged recounts the lines (for the gutter and search highlighting),
// re-tokenizes the text for syntax highlighting and tracks whether the text
// differs from the last saved baseline.
func (e *Editor) onTextChanged() {
	text := e.editor.GetText()
	e.lines = strings.Split(text, "\n")
	e.gutter.lines = len(e.lines)
	e.frame.ResizeItem(e.gutter, e.gutter.width(), 0)
	if e.highlighter != nil {
		e.highlighter.update(text)
	}
	if modified := text != e.initialContent; modified != e.modified {
		e.modified = modified
		e.refreshChrome()
	}
}

// handleEditorKey is the input capture of the text area. Global shortcuts are
// handled first; everything else goes through the vim handler when vim mode
// is enabled.
func (e *Editor) handleEditorKey(event *tcell.EventKey) *tcell.EventKey {
	alt := event.Modifiers()&tcell.ModAlt != 0
	shift := event.Modifiers()&tcell.ModShift != 0
	switch {
	case event.Key() == tcell.KeyCtrlS:
		e.saveAndExit()
		return nil
	case event.Key() == tcell.KeyCtrlQ:
		if e.forwardingCopy {
			return event // synthesized copy event on its way to the text area
		}
		e.requestQuit()
		return nil
	case event.Key() == tcell.KeyCtrlF:
		e.openSearchBar()
		return nil
	case event.Key() == tcell.KeyF3 && shift, event.Key() == tcell.KeyF15:
		// Shift+F3 arrives as F15 on terminals using the legacy function
		// key encoding.
		e.findPrevious()
		return nil
	case event.Key() == tcell.KeyF3:
		e.findNext()
		return nil
	case event.Key() == tcell.KeyF1, event.Key() == tcell.KeyCtrlUnderscore, alt && event.Rune() == 'h':
		e.pages.SwitchToPage("help")
		return nil
	case event.Key() == tcell.KeyF2, alt && event.Rune() == 'v':
		e.toggleVimMode()
		return nil
	case alt && event.Rune() == 'c':
		// Forward as Ctrl+Q, the text area's native "copy selection" binding
		// (the Ctrl+Q key itself is taken by "quit" above).
		e.copySelection()
		return nil
	}

	if !e.useVim {
		if event.Key() == tcell.KeyEscape {
			e.dismissTransients()
			return nil
		}
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

// dismissTransients clears the footer status and the search highlighting
// (vim's :noh). The search term itself survives, so n/N keep working.
func (e *Editor) dismissTransients() {
	e.hlSearch = false
	e.statusMsg = ""
	e.refreshChrome()
}

// handleVimNormalKey handles input in vim normal mode. Note that terminal
// paste events do not pass through this capture: like vim 8+ with bracketed
// paste, pasting in normal mode inserts the text.
func (e *Editor) handleVimNormalKey(event *tcell.EventKey) *tcell.EventKey {
	pending := e.pendingKey
	e.pendingKey = 0

	if event.Key() != tcell.KeyRune {
		if pending != 0 {
			e.refreshChrome() // remove the pending-key indicator
		}
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown,
			tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn,
			tcell.KeyDelete, tcell.KeyCtrlZ, tcell.KeyCtrlY,
			// Pure-movement emacs chords stay available in normal mode; the
			// destructive ones (Ctrl+K/U/W/D) stay blocked.
			tcell.KeyCtrlA, tcell.KeyCtrlE, tcell.KeyCtrlB:
			return event
		case tcell.KeyCtrlR: // vim redo
			return tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone)
		case tcell.KeyEnter:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
		case tcell.KeyEscape:
			e.dismissTransients()
			return nil
		}
		// Block everything else so the text cannot be modified in normal mode.
		return nil
	}
	result := e.handleVimNormalRune(pending, event.Rune())
	if e.pendingKey != pending {
		e.refreshChrome() // pending-key indicator appeared or disappeared
	}
	return result
}

// vimMotionKey translates single-key vim motions and character edits into
// text area key events. It returns nil for runes that are not one of them.
func vimMotionKey(r rune) *tcell.EventKey {
	switch r {
	case 'h':
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case 'l':
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case '0':
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case '$':
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case 'w':
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl)
	case 'b':
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl)
	case 'x':
		return tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone)
	case 'X':
		return tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone)
	case 'u':
		return tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone)
	}
	return nil
}

// handleVimNormalRune handles character commands in vim normal mode. The
// pending rune is the first key of a two-key command ("gg", "dd", "yy"), or 0.
func (e *Editor) handleVimNormalRune(pending, r rune) *tcell.EventKey {
	switch {
	case pending == 'g' && r == 'g':
		e.moveTo(0)
		return nil
	case pending == 'd' && r == 'd':
		e.deleteCurrentLine()
		return nil
	case pending == 'y' && r == 'y':
		e.yankCurrentLine()
		return nil
	}
	if event := vimMotionKey(r); event != nil {
		return event
	}
	switch r {
	case 'g', 'd', 'y':
		e.pendingKey = r
	case 'i':
		e.setVimState(VimInsert)
	case 'a':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case 'A':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case 'I':
		e.setVimState(VimInsert)
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case 'o':
		e.setVimState(VimInsert)
		e.forwardKeys(tcell.KeyEnd, tcell.KeyEnter)
	case 'O':
		e.setVimState(VimInsert)
		e.forwardKeys(tcell.KeyHome, tcell.KeyEnter, tcell.KeyUp)
	case 'G':
		e.moveTo(e.editor.GetTextLength())
	case 'p':
		e.pasteClipboard(false)
	case 'P':
		e.pasteClipboard(true)
	case '/':
		e.openSearchBar()
	case ':':
		e.openCommandBar()
	case 'n':
		e.findNext()
	case 'N':
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
	e.statusMsg = ""
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

// setStatus shows a transient message in the footer, replacing the key hints
// until the next mode change or Escape.
func (e *Editor) setStatus(msg string, isErr bool) {
	e.statusMsg = msg
	e.statusErr = isErr
	e.refreshChrome()
}

// saveAndExit persists the text and stops the editor. With a save handler
// set, a failed save keeps the editor open and reports the error instead.
func (e *Editor) saveAndExit() {
	text := e.editor.GetText()
	if text == e.initialContent {
		// Nothing new to persist; if an earlier ":w" saved content, the
		// saved flag already reflects it.
		e.result = text
		e.app.Stop()
		return
	}
	if e.saveHandler != nil {
		if err := e.saveHandler(text); err != nil {
			e.setStatus(fmt.Sprintf("save failed: %s", err), true)
			return
		}
	}
	e.result = text
	e.saved = true
	e.app.Stop()
}

// writeInPlace implements vim's ":w": persist through the save handler and
// keep editing. The saved baseline moves to the current text, so quitting
// afterwards does not ask for confirmation.
func (e *Editor) writeInPlace() {
	if e.saveHandler == nil {
		e.setStatus("no in-place save available; Ctrl+S saves and exits", true)
		return
	}
	text := e.editor.GetText()
	if err := e.saveHandler(text); err != nil {
		e.setStatus(fmt.Sprintf("write failed: %s", err), true)
		return
	}
	e.initialContent = text
	e.modified = false
	e.result = text
	e.saved = true
	e.setStatus(fmt.Sprintf("written (%d lines)", len(e.lines)), false)
}

// requestQuit exits the editor, asking for confirmation first if there are
// unsaved changes.
func (e *Editor) requestQuit() {
	if e.editor.GetText() == e.initialContent {
		e.app.Stop()
		return
	}
	e.quitModal.SetFocus(2) // default to "Cancel"
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
		SetText("You have unsaved changes.").
		AddButtons([]string{"Save & Quit", "Quit without saving", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, _ string) {
			switch buttonIndex {
			case 0:
				cancel()
				e.saveAndExit() // on save failure the editor stays open with the error shown
			case 1:
				e.discarded = true
				e.app.Stop()
			default:
				cancel()
			}
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

// showBottomBar swaps the footer for an input bar (search or command).
func (e *Editor) showBottomBar(bar *tview.InputField) {
	e.closeBottomBar()
	e.bottomBar = bar
	if e.useVim {
		e.setVimState(VimSearch)
	}
	e.layout.RemoveItem(e.footer)
	e.layout.AddItem(bar, 1, 0, 1, 1, 0, 0, true)
	e.app.SetFocus(bar)
}

// closeBottomBar restores the footer and returns focus to the editor. It is
// a no-op when no bar is open.
func (e *Editor) closeBottomBar() {
	if e.bottomBar == nil {
		return
	}
	bar := e.bottomBar
	e.bottomBar = nil
	e.layout.RemoveItem(bar)
	e.layout.AddItem(e.footer, 1, 0, 1, 1, 0, 0, false)
	if e.useVim {
		e.setVimState(VimNormal)
	}
	e.app.SetFocus(e.editor)
}

// openSearchBar replaces the footer with a vim-style search input. An empty
// submission repeats the previous search.
func (e *Editor) openSearchBar() {
	bar := tview.NewInputField().SetLabel("/")
	bar.SetDoneFunc(func(key tcell.Key) {
		term := bar.GetText()
		e.closeBottomBar()
		if key != tcell.KeyEnter {
			return
		}
		if term != "" {
			e.setSearchTerm(term)
		}
		e.findNext()
	})
	e.showBottomBar(bar)
}

// openCommandBar opens the vim ":" command line.
func (e *Editor) openCommandBar() {
	bar := tview.NewInputField().SetLabel(":")
	bar.SetDoneFunc(func(key tcell.Key) {
		cmd := strings.TrimSpace(bar.GetText())
		e.closeBottomBar()
		if key != tcell.KeyEnter || cmd == "" {
			return
		}
		e.runCommand(cmd)
	})
	e.showBottomBar(bar)
}

// runCommand executes a vim ":" command. Supported: w, q, q!, wq, x and line
// numbers.
func (e *Editor) runCommand(cmd string) {
	if n, err := strconv.Atoi(cmd); err == nil {
		e.goToLine(n)
		return
	}
	switch cmd {
	case "w":
		e.writeInPlace()
	case "q":
		e.requestQuit()
	case "q!":
		e.discarded = e.modified
		e.app.Stop()
	case "wq", "x":
		e.saveAndExit()
	default:
		e.setStatus(fmt.Sprintf("unknown command: %q", cmd), true)
	}
}

// goToLine moves the cursor to the start of the given 1-based line, clamped
// to the document.
func (e *Editor) goToLine(n int) {
	n = max(1, min(n, len(e.lines)))
	offset := 0
	for i := 0; i < n-1; i++ {
		offset += len(e.lines[i]) + 1
	}
	e.moveTo(offset)
	e.setStatus(fmt.Sprintf("line %d", n), false)
}

// setSearchTerm compiles the search pattern. The term is matched literally;
// all-lowercase terms match case-insensitively (vim's smartcase).
func (e *Editor) setSearchTerm(term string) {
	e.searchTerm = term
	e.searchRe = nil
	if term == "" {
		return
	}
	pattern := regexp.QuoteMeta(term)
	if term == strings.ToLower(term) {
		pattern = "(?i)" + pattern
	}
	e.searchRe = regexp.MustCompile(pattern) // QuoteMeta output always compiles
}

// searchMatches returns all match ranges in the current text.
func (e *Editor) searchMatches() [][]int {
	if e.searchRe == nil {
		return nil
	}
	return e.searchRe.FindAllStringIndex(e.editor.GetText(), -1)
}

// findNext selects the next occurrence of the search term, wrapping around
// at the end of the text.
func (e *Editor) findNext() {
	e.findMatch(false)
}

// findPrevious selects the previous occurrence of the search term, wrapping
// around at the beginning of the text.
func (e *Editor) findPrevious() {
	e.findMatch(true)
}

func (e *Editor) findMatch(backwards bool) {
	if e.searchRe == nil {
		e.setStatus("no search pattern (press / or Ctrl+F)", true)
		return
	}
	matches := e.searchMatches()
	if len(matches) == 0 {
		e.setStatus(fmt.Sprintf("pattern not found: %s", e.searchTerm), true)
		return
	}

	_, selStart, selEnd := e.editor.GetSelection()
	idx, wrapped := -1, false
	if backwards {
		for i := len(matches) - 1; i >= 0; i-- {
			if matches[i][0] < selStart {
				idx = i
				break
			}
		}
		if idx < 0 {
			idx, wrapped = len(matches)-1, true
		}
	} else {
		from := selEnd
		if from == selStart {
			from++ // No active selection: skip the character under the cursor.
		}
		for i, m := range matches {
			if m[0] >= from {
				idx = i
				break
			}
		}
		if idx < 0 {
			idx, wrapped = 0, true
		}
	}

	e.selectMatch(matches[idx][0], matches[idx][1])
	status := fmt.Sprintf("/%s — match %d of %d", e.searchTerm, idx+1, len(matches))
	if wrapped {
		status += " (wrapped)"
	}
	e.setStatus(status, false)
}

// selectMatch highlights the match spanning the given byte range and scrolls
// it into view.
func (e *Editor) selectMatch(start, end int) {
	e.hlSearch = true
	e.materializeThrough(start)
	e.editor.Select(start, end)
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

// currentLineRange returns the byte range of the line under the cursor,
// including the trailing newline when present.
func (e *Editor) currentLineRange() (int, int) {
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
	return lineStart, lineEnd
}

// deleteCurrentLine implements vim's "dd". Like vim, the line is yanked
// before it is removed, so "p" can put it back elsewhere.
func (e *Editor) deleteCurrentLine() {
	lineStart, lineEnd := e.currentLineRange()
	if line := e.yankRange(lineStart, lineEnd); line == "" {
		return
	}
	e.editor.Replace(lineStart, lineEnd, "")
}

// yankCurrentLine implements vim's "yy".
func (e *Editor) yankCurrentLine() {
	lineStart, lineEnd := e.currentLineRange()
	if e.yankRange(lineStart, lineEnd) != "" {
		e.setStatus("1 line yanked", false)
	}
}

// yankRange stores a linewise register from the given byte range, ensuring a
// trailing newline so pasting inserts a full line.
func (e *Editor) yankRange(start, end int) string {
	text := e.editor.GetText()
	if start >= end || start >= len(text) {
		return ""
	}
	line := text[start:min(end, len(text))]
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	e.setClipText(line, true)
	return line
}

// pasteClipboard implements vim's "p"/"P". Linewise yanks (yy, dd) paste as
// whole lines below/above the cursor; charwise clipboard text is inserted
// after/at the cursor.
func (e *Editor) pasteClipboard(before bool) {
	if e.clipText == "" {
		return
	}
	text := e.editor.GetText()
	_, start, _ := e.editor.GetSelection()
	if start > len(text) {
		start = len(text)
	}

	if !e.clipLinewise {
		pos := start
		if !before && pos < len(text) && text[pos] != '\n' {
			_, size := utf8.DecodeRuneInString(text[pos:])
			pos += size
		}
		e.editor.Replace(pos, pos, e.clipText)
		return
	}

	lineStart, lineEnd := e.currentLineRange()
	content := e.clipText
	insertPos := lineStart
	if !before {
		insertPos = lineEnd
		if lineEnd == len(text) && !strings.HasSuffix(text, "\n") {
			// Pasting below the last line: the newline goes in front.
			content = "\n" + strings.TrimSuffix(content, "\n")
		}
	}
	e.editor.Replace(insertPos, insertPos, content)
	e.moveTo(insertPos)
}

// footerText composes the footer: key hints (or a transient status message)
// plus the cursor ruler.
func (e *Editor) footerText() string {
	cursorRow, cursorCol, _, _ := e.editor.GetCursor()
	var body string
	switch {
	case e.statusMsg != "":
		color := "green"
		if e.statusErr {
			color = "red"
		}
		body = fmt.Sprintf("[%s]%s[white]", color, tview.Escape(e.statusMsg))
	case e.useVim && e.vimMode == VimInsert:
		body = "[green]-- INSERT --[white] Esc: normal mode | [yellow]Ctrl+S[white]: Save & Exit"
	case e.useVim && e.vimMode == VimNormal:
		body = "[yellow]i[white] insert  [yellow]hjkl[white] move  [yellow]/[white] [yellow]n[white] [yellow]N[white] search  [yellow]dd yy p[white] edit  [yellow]u[white] undo  [yellow]:[white] cmd  [yellow]Ctrl+S[white] save+exit  [yellow]F1[white] help"
	case e.useVim:
		body = "[yellow]Enter[white]: apply | [yellow]Esc[white]: cancel"
	default:
		body = "[yellow]Ctrl+S[white]: Save & Exit | [yellow]Ctrl+Q[white]: Quit | [yellow]Ctrl+F[white]: Search | [yellow]F1[white]: Help | [yellow]F2[white]: VIM mode"
	}

	ruler := fmt.Sprintf("Ln %d, Col %d", cursorRow+1, cursorCol+1)
	if e.useVim && e.pendingKey != 0 {
		ruler = string(e.pendingKey) + "  " + ruler
	}
	return body + "  [gray]│ " + ruler + "[white]"
}

// refreshChrome updates the editor title for the current mode. The footer
// recomposes itself on every draw.
func (e *Editor) refreshChrome() {
	title := e.title
	if e.modified {
		title += " [+]"
	}
	if e.useVim {
		var mode string
		switch e.vimMode {
		case VimInsert:
			mode = "INSERT"
		case VimSearch:
			mode = "SEARCH"
		default:
			mode = "NORMAL"
		}
		title = fmt.Sprintf("%s (VIM - %s)", title, mode)
	}
	e.frame.SetTitle(title)
}

// navigationHelp returns the navigation help text, including the vim section
// when vim mode is enabled.
func (e *Editor) navigationHelp() string {
	text := `[green]Navigation[white]

[yellow]Arrow Keys[white]: Move cursor around
[yellow]Ctrl-A, Home[white]: Move to beginning of line
[yellow]Ctrl-E, End[white]: Move to end of line
[yellow]Page Down / Page Up[white]: Move by one page
[yellow]Alt-Up/Down/Left/Right[white]: Scroll the page
[yellow]Alt-B, Ctrl-Left[white]: Move back by one word
[yellow]Alt-F, Ctrl-Right[white]: Move forward by one word

[green]Search[white]

[yellow]Ctrl-F[white]: Search (all-lowercase terms match any case)
[yellow]F3 / Shift-F3[white]: Next / previous match
[yellow]Esc[white]: Clear match highlighting`

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
[yellow]dd/yy[white]: Delete/yank line, [yellow]p/P[white]: paste after/before
[yellow]u / Ctrl-R[white]: Undo / redo
[yellow]/[white]: Search, then [yellow]n/N[white]: next/previous match
[yellow]:[white]: Command (w, q, q!, wq, or a line number)
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
[yellow]Ctrl-C, Alt-C[white]: Copy selection (also to the system clipboard
on terminals with OSC 52 support)
[yellow]Ctrl-X[white]: Cut selection
[yellow]Ctrl-V[white]: Paste

[blue]Press Enter for more help, Escape to return to editor[white]`

const commandsHelp = `[green]Editor Commands[white]

[yellow]Ctrl-S[white]: Save changes and exit editor
[yellow]Ctrl-Q, Ctrl-C[white]: Quit (asks for confirmation on unsaved changes)
[yellow]F1, Ctrl-_, Alt-H[white]: Show this help
[yellow]F2, Alt-V[white]: Toggle VIM mode on/off
[yellow]Ctrl-F[white]: Search

On macOS terminals without "Option as Meta", use the F-key and
Ctrl alternatives instead of the Alt shortcuts.

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
		SetRows(0, 27, 0).
		AddItem(helpPages, 1, 1, 1, 1, 0, 0, true)

	e.pages.AddPage("help", helpGrid, true, false)
}
