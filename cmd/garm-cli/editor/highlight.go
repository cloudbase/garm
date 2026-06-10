// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package editor

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// SyntaxForOSType maps a runner OS type to the syntax language used for
// highlighting its install templates. Bash is the fallback for anything that
// is not Windows.
func SyntaxForOSType(osType string) string {
	if strings.EqualFold(osType, "windows") {
		return "powershell"
	}
	return "bash"
}

// lineSpan is a colored byte range within a single line.
type lineSpan struct {
	start, end int
	color      tcell.Color
}

// syntaxHighlighter tokenizes the full text on every change and keeps a list
// of colored byte spans per line. Highlighting is applied as a post-draw pass
// that recolors the foreground of visible cells, since tview's TextArea has
// no per-token styling of its own.
type syntaxHighlighter struct {
	lexer chroma.Lexer
	lines []string
	spans [][]lineSpan
}

// newSyntaxHighlighter returns a highlighter for the given chroma language
// name, or nil if the language is unknown or empty.
func newSyntaxHighlighter(language string) *syntaxHighlighter {
	lexer := lexers.Get(language)
	if lexer == nil {
		return nil
	}
	return &syntaxHighlighter{lexer: chroma.Coalesce(lexer)}
}

// tokenColor maps chroma token types to terminal colors. The palette sticks
// to the basic ANSI colors so it renders on 8-color terminals too. Error
// tokens (e.g. half-typed constructs the lexer cannot place) keep the default
// color to avoid flagging code while it is being written.
func tokenColor(t chroma.TokenType) tcell.Color {
	switch {
	case t.InCategory(chroma.Comment):
		return tcell.ColorTeal
	case t.InCategory(chroma.Keyword):
		return tcell.ColorOlive
	case t.InSubCategory(chroma.LiteralString):
		return tcell.ColorGreen
	case t.InSubCategory(chroma.LiteralNumber):
		return tcell.ColorPurple
	case t == chroma.NameVariable, t == chroma.NameVariableGlobal,
		t == chroma.NameVariableInstance, t == chroma.NameVariableClass,
		t == chroma.NameBuiltin, t == chroma.NameBuiltinPseudo,
		t == chroma.NameAttribute:
		return tcell.ColorMaroon
	}
	return tcell.ColorDefault
}

// update re-tokenizes the text and rebuilds the per-line span lists.
func (h *syntaxHighlighter) update(text string) {
	h.lines = strings.Split(text, "\n")
	h.spans = make([][]lineSpan, len(h.lines))
	iterator, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return // leave the text unhighlighted
	}

	line, col := 0, 0 // current line index and byte offset within it
	for _, token := range iterator.Tokens() {
		color := tokenColor(token.Type)
		value := token.Value
		for value != "" && line < len(h.lines) {
			segment := value
			newline := false
			if idx := strings.IndexByte(value, '\n'); idx >= 0 {
				segment, value = value[:idx], value[idx+1:]
				newline = true
			} else {
				value = ""
			}
			if color != tcell.ColorDefault && segment != "" {
				h.spans[line] = append(h.spans[line], lineSpan{col, col + len(segment), color})
			}
			col += len(segment)
			if newline {
				line++
				col = 0
			}
		}
	}
}

// recolor applies the syntax colors to the visible cells of the text area.
// It walks each visible line with the same width rules the text area uses
// (tabs render as TabSize cells, everything else uses grapheme cluster
// widths). Only cells whose foreground still is the default text color are
// touched, so the selection keeps its inverted styling.
func (h *syntaxHighlighter) recolor(screen tcell.Screen, area *tview.TextArea) {
	x, y, width, height := area.GetInnerRect()
	rowOffset, columnOffset := area.GetOffset()
	for row := range height {
		lineIdx := rowOffset + row
		if lineIdx >= len(h.lines) {
			break
		}
		if len(h.spans[lineIdx]) > 0 {
			h.recolorLine(screen, x, y+row, width, columnOffset, h.lines[lineIdx], h.spans[lineIdx])
		}
	}
}

func (h *syntaxHighlighter) recolorLine(screen tcell.Screen, x, y, width, columnOffset int, line string, spans []lineSpan) {
	defaultFg := tview.Styles.PrimaryTextColor
	pos, col, state, spanIdx := 0, 0, -1, 0
	for line != "" && col-columnOffset < width {
		var cluster string
		var boundaries int
		cluster, line, boundaries, state = uniseg.StepString(line, state)
		clusterWidth := boundaries >> uniseg.ShiftWidth
		if cluster == "\t" {
			clusterWidth = tview.TabSize
		}
		for spanIdx < len(spans) && spans[spanIdx].end <= pos {
			spanIdx++
		}
		if spanIdx < len(spans) && spans[spanIdx].start <= pos {
			for i := range clusterWidth {
				cx := col + i - columnOffset
				if cx < 0 || cx >= width {
					continue
				}
				str, style, _ := screen.Get(x+cx, y)
				if str == "" {
					continue // continuation cell of a wide character
				}
				if fg, _, _ := style.Decompose(); fg == defaultFg {
					screen.Put(x+cx, y, str, style.Foreground(spans[spanIdx].color))
				}
			}
		}
		pos += len(cluster)
		col += clusterWidth
	}
}
