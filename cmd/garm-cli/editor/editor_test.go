package editor

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

// newTestEditor builds an editor around content and draws it once on a
// simulation screen. The text area computes its layout during drawing, which
// programmatic cursor positioning (Select) depends on.
func newTestEditor(t *testing.T, content string) *Editor {
	t.Helper()
	e := NewEditor()
	e.setup(content)
	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	t.Cleanup(sim.Fini)
	sim.SetSize(80, 24)
	e.editor.SetRect(0, 0, 80, 24)
	e.editor.Draw(sim)
	return e
}

func selection(e *Editor) (string, int, int) {
	return e.editor.GetSelection()
}

func TestFindNextSelectsMatchesAndWraps(t *testing.T) {
	// Multi-byte characters and a tab before the first match make sure the
	// search operates on byte offsets, not screen columns.
	content := "héllo\twörld\nfirst foo\nsecond foo\nlast"
	e := newTestEditor(t, content)
	e.searchTerm = "foo"

	first := strings.Index(content, "foo")
	second := strings.LastIndex(content, "foo")

	e.findNext()
	text, start, end := selection(e)
	require.Equal(t, "foo", text)
	require.Equal(t, first, start)
	require.Equal(t, first+3, end)

	e.findNext()
	_, start, _ = selection(e)
	require.Equal(t, second, start)

	// No further match: wrap around to the first one.
	e.findNext()
	_, start, _ = selection(e)
	require.Equal(t, first, start)
}

func TestFindPreviousWrapsAround(t *testing.T) {
	content := "first foo\nsecond foo\n"
	e := newTestEditor(t, content)
	e.searchTerm = "foo"

	// Cursor is at the beginning: wrap to the last occurrence.
	e.findPrevious()
	text, start, _ := selection(e)
	require.Equal(t, "foo", text)
	require.Equal(t, strings.LastIndex(content, "foo"), start)

	e.findPrevious()
	_, start, _ = selection(e)
	require.Equal(t, strings.Index(content, "foo"), start)
}

func TestSearchBeyondVisibleAreaScrollsCorrectly(t *testing.T) {
	// Regression test: TextArea.Select mislocates byte offsets beyond the
	// lines it has materialized so far (everything past the last known line
	// is attributed to that line, producing a huge cursor column). Searching
	// for a match far below the visible area then panned the view far to the
	// right, hiding the text.
	var sb strings.Builder
	for range 500 {
		sb.WriteString("\tsome padding line with a tab\n")
	}
	sb.WriteString("\tneedle here\n")
	content := sb.String()

	e := newTestEditor(t, content)
	e.searchTerm = "needle"
	e.findNext()

	row, col, _, _ := e.editor.GetCursor()
	require.Equal(t, 500, row)
	// One tab (rendered as TabSize columns) precedes the match.
	require.Equal(t, tview.TabSize, col)

	rowOffset, colOffset := e.editor.GetOffset()
	require.Equal(t, 0, colOffset, "view must not pan horizontally")
	_, _, _, height := e.editor.GetInnerRect()
	require.GreaterOrEqual(t, row, rowOffset)
	require.Less(t, row, rowOffset+height, "match row must be inside the viewport")

	// Jumping back to the start must restore the viewport.
	e.moveTo(0)
	rowOffset, colOffset = e.editor.GetOffset()
	require.Equal(t, 0, rowOffset)
	require.Equal(t, 0, colOffset)
}

func TestFindNextWithoutTermIsNoop(t *testing.T) {
	e := newTestEditor(t, "some text")
	e.findNext()
	e.findPrevious()
	_, start, end := selection(e)
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)
}

func TestMoveTo(t *testing.T) {
	content := "one\ntwo\nthree"
	e := newTestEditor(t, content)

	e.moveTo(e.editor.GetTextLength()) // vim "G"
	_, start, end := selection(e)
	require.Equal(t, len(content), start)
	require.Equal(t, len(content), end)

	e.moveTo(0) // vim "gg"
	_, start, _ = selection(e)
	require.Equal(t, 0, start)
}

func TestDeleteCurrentLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		cursor   int
		expected string
	}{
		{"middle line", "one\ntwo\nthree\n", 5, "one\nthree\n"},
		{"first line", "one\ntwo\n", 0, "two\n"},
		{"last line without newline", "one\ntwo", 5, "one\n"},
		{"single line", "only", 2, ""},
		{"empty text", "", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEditor(t, tc.content)
			e.moveTo(tc.cursor)
			e.deleteCurrentLine()
			require.Equal(t, tc.expected, e.editor.GetText())
		})
	}
}

func TestVimNormalModeKeyTranslation(t *testing.T) {
	e := newTestEditor(t, "some text")
	e.SetVimMode(true)

	// Movement and editing commands translate to text area key events.
	for r, key := range map[rune]tcell.Key{
		'h': tcell.KeyLeft,
		'j': tcell.KeyDown,
		'k': tcell.KeyUp,
		'l': tcell.KeyRight,
		'0': tcell.KeyHome,
		'$': tcell.KeyEnd,
		'x': tcell.KeyDelete,
		'u': tcell.KeyCtrlZ,
	} {
		event := e.handleVimNormalRune(0, r)
		require.NotNil(t, event, "rune %q", r)
		require.Equal(t, key, event.Key(), "rune %q", r)
	}

	// Regular text input is blocked in normal mode.
	require.Nil(t, e.handleVimNormalRune(0, 'z'))
	require.Nil(t, e.handleVimNormalRune(0, 'q'))
}

func TestVimTwoKeySequences(t *testing.T) {
	content := "one\ntwo\nthree"
	e := newTestEditor(t, content)
	e.SetVimMode(true)
	e.moveTo(5)

	// "gg" moves to the beginning of the document.
	event := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone)
	require.Nil(t, e.handleVimNormalKey(event))
	require.Equal(t, 'g', e.pendingKey)
	require.Nil(t, e.handleVimNormalKey(event))
	require.Equal(t, rune(0), e.pendingKey)
	_, start, _ := selection(e)
	require.Equal(t, 0, start)

	// "dd" deletes the current line.
	e.moveTo(5)
	dKey := tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)
	require.Nil(t, e.handleVimNormalKey(dKey))
	require.Nil(t, e.handleVimNormalKey(dKey))
	require.Equal(t, "one\nthree", e.editor.GetText())

	// A different key in between cancels the pending command.
	require.Nil(t, e.handleVimNormalKey(dKey))
	e.handleVimNormalKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	require.Equal(t, rune(0), e.pendingKey)
}

func TestModeSwitching(t *testing.T) {
	e := newTestEditor(t, "some text")
	e.SetVimMode(true)
	require.Equal(t, VimNormal, e.vimMode)

	require.Nil(t, e.handleVimNormalRune(0, 'i'))
	require.Equal(t, VimInsert, e.vimMode)

	// In insert mode, Escape returns to normal mode and typing passes through.
	typed := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone)
	require.Equal(t, typed, e.handleEditorKey(typed))
	require.Nil(t, e.handleEditorKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)))
	require.Equal(t, VimNormal, e.vimMode)

	// "a" enters insert mode and moves the cursor right.
	event := e.handleVimNormalRune(0, 'a')
	require.Equal(t, VimInsert, e.vimMode)
	require.Equal(t, tcell.KeyRight, event.Key())
}

func TestSaveShortcut(t *testing.T) {
	e := newTestEditor(t, "initial")
	e.editor.SetText("modified", true)

	require.Nil(t, e.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)))
	require.True(t, e.saved)
	require.Equal(t, "modified", e.result)
}

func TestQuitWithoutChangesDoesNotConfirm(t *testing.T) {
	e := newTestEditor(t, "initial")

	// Unchanged content: quit immediately, no confirmation page.
	require.Nil(t, e.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)))
	name, _ := e.pages.GetFrontPage()
	require.Equal(t, "main", name)
	require.False(t, e.saved)
}

func TestQuitWithChangesAsksForConfirmation(t *testing.T) {
	e := newTestEditor(t, "initial")
	e.editor.SetText("modified", true)

	require.Nil(t, e.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)))
	name, _ := e.pages.GetFrontPage()
	require.Equal(t, "confirm-quit", name)
	require.False(t, e.saved)
}

// screenRow returns the text of one row of a simulation screen.
func screenRow(t *testing.T, sim tcell.SimulationScreen, y int) string {
	t.Helper()
	cells, width, _ := sim.GetContents()
	var sb strings.Builder
	for x := range width {
		r := ' '
		if runes := cells[y*width+x].Runes; len(runes) > 0 {
			r = runes[0]
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func TestLineNumberGutterRendersAndFollowsScroll(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 120; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	content := sb.String()
	e := newTestEditor(t, content)

	// 120 newlines yield 121 addressable rows; 3 digits + 2 separator columns.
	require.Equal(t, 121, e.gutter.lines)
	require.Equal(t, 5, e.gutter.width())

	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	t.Cleanup(sim.Fini)
	sim.SetSize(e.gutter.width(), 24)
	e.gutter.SetRect(0, 0, e.gutter.width(), 24)

	e.gutter.Draw(sim)
	sim.Show()
	require.Equal(t, "1", strings.TrimSpace(screenRow(t, sim, 0)))
	require.Equal(t, "24", strings.TrimSpace(screenRow(t, sim, 23)))

	// The whole strip carries the gutter background shade.
	cells, width, _ := sim.GetContents()
	for _, idx := range []int{0, width - 1, 23*width + width - 1} {
		_, bg, _ := cells[idx].Style.Decompose()
		require.Equal(t, gutterBackground, bg, "cell %d", idx)
	}

	// Jump near the bottom: the gutter follows the text area's scroll offset.
	e.moveTo(strings.Index(content, "line 100"))
	rowOffset, _ := e.editor.GetOffset()
	require.Positive(t, rowOffset)
	e.gutter.Draw(sim)
	sim.Show()
	require.Equal(t, strconv.Itoa(rowOffset+1), strings.TrimSpace(screenRow(t, sim, 0)))
}

func TestGutterTracksLineCount(t *testing.T) {
	e := newTestEditor(t, "one\ntwo\nthree")
	require.Equal(t, 3, e.gutter.lines)
	require.Equal(t, 3, e.gutter.width())

	e.deleteCurrentLine()
	require.Equal(t, 2, e.gutter.lines)

	// Inserting text with newlines goes through the changed callback too.
	e.editor.Replace(0, 0, strings.Repeat("x\n", 100))
	require.Equal(t, 102, e.gutter.lines)
	require.Equal(t, 5, e.gutter.width())
}

func TestSearchBarOpensAndCloses(t *testing.T) {
	e := newTestEditor(t, "first foo\nsecond foo\n")
	e.SetVimMode(true)

	require.Nil(t, e.handleVimNormalRune(0, '/'))
	require.NotNil(t, e.searchBar)
	require.Equal(t, VimSearch, e.vimMode)

	// Type a term and submit it via the done func.
	e.searchBar.SetText("foo")
	handler := e.searchBar.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	require.Nil(t, e.searchBar)
	require.Equal(t, VimNormal, e.vimMode)
	require.Equal(t, "foo", e.searchTerm)
	text, start, _ := selection(e)
	require.Equal(t, "foo", text)
	require.Equal(t, 6, start)
}
