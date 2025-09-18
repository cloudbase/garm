package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Editor represents a text editor that can be launched with initial content
type Editor struct {
	app    *tview.Application
	pages  *tview.Pages
	editor *tview.TextArea
	result string
	saved  bool
}

// NewEditor creates a new editor instance
func NewEditor() *Editor {
	return &Editor{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
	}
}

// EditText launches the editor with initial content and returns the edited text
func (e *Editor) EditText(initialContent string) (string, bool, error) {
	e.result = initialContent
	e.saved = false

	e.editor = tview.NewTextArea().
		SetText(initialContent, true).
		SetWrap(false)

	e.editor.SetBorder(true).
		SetTitle("Text Editor").
		SetTitleAlign(tview.AlignCenter)

	e.editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
		return event
	})

	// Create footer with shortcuts
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]Ctrl+S[white]: Save & Exit | [yellow]Ctrl+Q[white]: Quit | [yellow]Alt+H[white]: Help").
		SetTextAlign(tview.AlignCenter)

	// Create layout with editor and footer
	grid := tview.NewGrid().
		SetRows(0, 1).
		AddItem(e.editor, 0, 0, 1, 1, 0, 0, true).
		AddItem(footer, 1, 0, 1, 1, 0, 0, false)

	// Create help pages
	e.createHelpPages()

	// Add main editor page
	e.pages.AddPage("main", grid, true, true)

	e.app.SetRoot(e.pages, true)
	if err := e.app.Run(); err != nil {
		return "", false, err
	}

	return e.result, e.saved, nil
}

// createHelpPages creates the help system with multiple pages
func (e *Editor) createHelpPages() {
	// Create three separate help pages
	help1 := tview.NewTextView()
	help1.SetDynamicColors(true)
	help1.SetText(`[green]Navigation[white]

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
[yellow]Alt-F, Ctrl-Right[white]: Move forward by one word

[blue]Press Enter for more help, Escape to return to editor[white]`)
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
	help3.SetText(`[green]Undo/Redo[white]

[yellow]Ctrl-Z[white]: Undo last action
[yellow]Ctrl-Y[white]: Redo last undone action

[green]Editor Commands[white]

[yellow]Ctrl-S[white]: Save changes and exit editor
[yellow]Ctrl-Q[white]: Quit without saving changes
[yellow]Alt+H[white]: Show this help

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
