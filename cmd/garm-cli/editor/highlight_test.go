package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestSyntaxForOSType(t *testing.T) {
	require.Equal(t, "powershell", SyntaxForOSType("windows"))
	require.Equal(t, "powershell", SyntaxForOSType("Windows"))
	require.Equal(t, "bash", SyntaxForOSType("linux"))
	require.Equal(t, "bash", SyntaxForOSType(""))
	require.Equal(t, "bash", SyntaxForOSType("freebsd"))
}

func TestNewSyntaxHighlighter(t *testing.T) {
	require.NotNil(t, newSyntaxHighlighter("bash"))
	require.NotNil(t, newSyntaxHighlighter("powershell"))
	require.Nil(t, newSyntaxHighlighter(""))
	require.Nil(t, newSyntaxHighlighter("no-such-language"))
}

// spanAt returns the color of the span covering the given byte offset on the
// given line, or ColorDefault if none does.
func spanAt(h *syntaxHighlighter, line, pos int) tcell.Color {
	for _, s := range h.spans[line] {
		if s.start <= pos && pos < s.end {
			return s.color
		}
	}
	return tcell.ColorDefault
}

func TestHighlighterUpdateBash(t *testing.T) {
	h := newSyntaxHighlighter("bash")
	content := "#!/bin/bash\n# a comment\nif true; then\nMSG=\"hello\"\nfi\n"
	h.update(content)
	require.Len(t, h.spans, strings.Count(content, "\n")+1)

	require.Equal(t, tcell.ColorTeal, spanAt(h, 0, 0), "shebang is a comment")
	require.Equal(t, tcell.ColorTeal, spanAt(h, 1, 0), "comment")
	require.Equal(t, tcell.ColorOlive, spanAt(h, 2, 0), "if keyword")
	require.Equal(t, tcell.ColorGreen, spanAt(h, 3, len("MSG=")), "string")
	require.Equal(t, tcell.ColorOlive, spanAt(h, 4, 0), "fi keyword")
	// Whitespace stays uncolored.
	require.Equal(t, tcell.ColorDefault, spanAt(h, 2, len("if")))
}

func TestHighlighterMultilineToken(t *testing.T) {
	// A double-quoted string spanning two lines must produce a span on each.
	h := newSyntaxHighlighter("bash")
	h.update("X=\"first\nsecond\"\n")
	require.Equal(t, tcell.ColorGreen, spanAt(h, 0, len("X=\"fir")))
	require.Equal(t, tcell.ColorGreen, spanAt(h, 1, 0))
}

func TestHighlighterRecolorOnScreen(t *testing.T) {
	e := NewEditor()
	e.SetSyntax("bash")
	e.setup("# comment\nMSG=\"hi\"\nplain\n")

	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	t.Cleanup(sim.Fini)
	sim.SetSize(40, 10)
	e.frame.SetRect(0, 0, 40, 10)
	e.frame.Draw(sim)
	sim.Show()

	cells, width, _ := sim.GetContents()
	fgAt := func(x, y int) tcell.Color {
		fg, _, _ := cells[y*width+x].Style.Decompose()
		return fg
	}
	// Find where the text starts: after the border and the gutter.
	textX := 1 + e.gutter.width()

	require.Equal(t, tcell.ColorTeal, fgAt(textX, 1), "comment on line 1")
	require.Equal(t, tcell.ColorGreen, fgAt(textX+len("MSG="), 2), "string on line 2")
	require.Equal(t, tview.Styles.PrimaryTextColor, fgAt(textX, 3), "plain text keeps default color")
}

func TestEditorWithoutSyntaxHasNoHighlighter(t *testing.T) {
	e := newTestEditor(t, "# not highlighted")
	require.Nil(t, e.highlighter)
	require.Nil(t, e.frame.highlighter)
}
