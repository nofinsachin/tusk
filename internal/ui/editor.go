package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5"

	"github.com/nofinsachin/tusk/internal/db"
)

// queryTimeout bounds how long a single query may run before being cancelled.
const queryTimeout = 60 * time.Second

// textareaBlinkCmd is the cursor-blink command for a freshly focused editor.
func textareaBlinkCmd() tea.Cmd { return textarea.Blink }

// --- messages ---

// queryResultMsg carries the outcome of RunQuery.
type queryResultMsg struct {
	res db.QueryResult
	err error
}

// editorFocus identifies which pane has keyboard focus.
type editorFocus int

const (
	focusEditor editorFocus = iota
	focusResults
)

// editor is the SQL query-editor screen.
type editor struct {
	conn  *pgx.Conn
	area  textarea.Model
	focus editorFocus

	running bool
	err     string
	res     *db.QueryResult

	// results scroll offsets
	rowOff int
	colOff int

	width  int
	height int
}

func newEditor(conn *pgx.Conn) editor {
	ta := textarea.New()
	ta.Placeholder = "SELECT * FROM ..."
	ta.Focus()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0 // unlimited

	return editor{
		conn:  conn,
		area:  ta,
		focus: focusEditor,
	}
}

func (e *editor) setSize(w, h int) {
	e.width = w
	e.height = h
	// Editor takes roughly the top third; leave room for results + help.
	editorHeight := h / 3
	if editorHeight < 3 {
		editorHeight = 3
	}
	e.area.SetWidth(w)
	e.area.SetHeight(editorHeight)
}

// runCmd executes the current buffer off the UI goroutine.
func runCmd(conn *pgx.Conn, sql string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		defer cancel()
		res, err := db.RunQuery(ctx, conn, sql)
		return queryResultMsg{res: res, err: err}
	}
}

func (e editor) Update(msg tea.Msg) (editor, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.setSize(msg.Width, msg.Height)
		return e, nil

	case queryResultMsg:
		e.running = false
		e.rowOff, e.colOff = 0, 0
		if msg.err != nil {
			e.err = msg.err.Error()
			e.res = nil
			return e, nil
		}
		e.err = ""
		r := msg.res
		e.res = &r
		// Move focus to results when a row set comes back for easy scrolling.
		if r.IsRowSet() && len(r.Rows) > 0 {
			e.focus = focusResults
			e.area.Blur()
		}
		return e, nil

	case tea.KeyMsg:
		return e.updateKey(msg)
	}

	// Route other messages to the textarea when it's focused.
	if e.focus == focusEditor {
		var cmd tea.Cmd
		e.area, cmd = e.area.Update(msg)
		return e, cmd
	}
	return e, nil
}

func (e editor) updateKey(msg tea.KeyMsg) (editor, tea.Cmd) {
	switch msg.String() {
	case "ctrl+r":
		sql := strings.TrimSpace(e.area.Value())
		if sql == "" || e.running {
			return e, nil
		}
		e.running = true
		e.err = ""
		return e, runCmd(e.conn, sql)

	case "tab":
		// Toggle focus between editor and results.
		if e.focus == focusEditor {
			e.focus = focusResults
			e.area.Blur()
		} else {
			e.focus = focusEditor
			e.area.Focus()
			return e, textarea.Blink
		}
		return e, nil
	}

	if e.focus == focusResults {
		return e.updateResultsKey(msg)
	}

	// Editor focused: pass everything else to the textarea.
	var cmd tea.Cmd
	e.area, cmd = e.area.Update(msg)
	return e, cmd
}

func (e editor) updateResultsKey(msg tea.KeyMsg) (editor, tea.Cmd) {
	if e.res == nil || !e.res.IsRowSet() {
		return e, nil
	}
	nRows := len(e.res.Rows)
	nCols := len(e.res.Columns)
	switch msg.String() {
	case "down", "j":
		if e.rowOff < nRows-1 {
			e.rowOff++
		}
	case "up", "k":
		if e.rowOff > 0 {
			e.rowOff--
		}
	case "right", "l":
		if e.colOff < nCols-1 {
			e.colOff++
		}
	case "left", "h":
		if e.colOff > 0 {
			e.colOff--
		}
	case "home", "g":
		e.rowOff = 0
	case "end", "G":
		e.rowOff = nRows - 1
		if e.rowOff < 0 {
			e.rowOff = 0
		}
	}
	return e, nil
}

// resultSummary returns a one-line description of a result. Pure and testable.
func resultSummary(r db.QueryResult) string {
	ms := r.Duration.Milliseconds()
	if r.IsRowSet() {
		return fmt.Sprintf("%d row(s) · %d col(s) · %dms", len(r.Rows), len(r.Columns), ms)
	}
	cmd := r.Command
	if cmd == "" {
		cmd = "OK"
	}
	return fmt.Sprintf("%s · %dms", cmd, ms)
}

// --- styling ---

var (
	editorErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	editorOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	resultHdrStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	paneHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (e editor) View() string {
	var b strings.Builder

	// Editor pane.
	editorHint := "editor"
	if e.focus == focusEditor {
		editorHint = "▸ editor"
	}
	b.WriteString(paneHintStyle.Render(editorHint))
	b.WriteString("\n")
	b.WriteString(e.area.View())
	b.WriteString("\n")

	// Status line.
	switch {
	case e.running:
		b.WriteString(paneHintStyle.Render("running…"))
	case e.err != "":
		b.WriteString(editorErrStyle.Render("✗ " + e.err))
	case e.res != nil:
		b.WriteString(editorOKStyle.Render("✓ " + resultSummary(*e.res)))
	default:
		b.WriteString(paneHintStyle.Render("ctrl+r run · tab switch pane · esc back"))
	}
	b.WriteString("\n")

	// Results pane.
	if e.res != nil && e.res.IsRowSet() {
		resultsHint := "results"
		if e.focus == focusResults {
			resultsHint = "▸ results"
		}
		b.WriteString(paneHintStyle.Render(resultsHint))
		b.WriteString("\n")
		b.WriteString(e.renderResults())
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("ctrl+r run · tab switch pane · ↑↓←→ scroll results · esc back"))
	return b.String()
}

// renderResults draws a simple windowed table of the current result set.
func (e editor) renderResults() string {
	r := e.res
	if len(r.Rows) == 0 {
		return paneHintStyle.Render("(0 rows)")
	}

	// How many rows/cols fit.
	// Reserve: editor (~h/3) + a few status/help lines.
	bodyRows := e.height/2 - 2
	if bodyRows < 1 {
		bodyRows = 1
	}
	colWidth := 20
	maxColsShown := e.width / (colWidth + 1)
	if maxColsShown < 1 {
		maxColsShown = 1
	}

	colEnd := e.colOff + maxColsShown
	if colEnd > len(r.Columns) {
		colEnd = len(r.Columns)
	}
	rowEnd := e.rowOff + bodyRows
	if rowEnd > len(r.Rows) {
		rowEnd = len(r.Rows)
	}

	var sb strings.Builder

	// Header.
	var hdr []string
	for c := e.colOff; c < colEnd; c++ {
		hdr = append(hdr, pad(r.Columns[c], colWidth))
	}
	sb.WriteString(resultHdrStyle.Render(strings.Join(hdr, " ")))
	sb.WriteString("\n")

	// Rows.
	for ri := e.rowOff; ri < rowEnd; ri++ {
		var cells []string
		row := r.Rows[ri]
		for c := e.colOff; c < colEnd && c < len(row); c++ {
			cells = append(cells, pad(row[c], colWidth))
		}
		sb.WriteString(strings.Join(cells, " "))
		sb.WriteString("\n")
	}

	// Scroll indicator.
	sb.WriteString(paneHintStyle.Render(fmt.Sprintf(
		"rows %d–%d/%d · cols %d–%d/%d",
		e.rowOff+1, rowEnd, len(r.Rows),
		e.colOff+1, colEnd, len(r.Columns))))
	return sb.String()
}

// pad truncates or right-pads s to exactly w runes (with an ellipsis when cut).
func pad(s string, w int) string {
	// Collapse newlines/tabs so cells stay single-line.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	r := []rune(s)
	if len(r) > w {
		if w <= 1 {
			return string(r[:w])
		}
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}
