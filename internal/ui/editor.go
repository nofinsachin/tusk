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
	initialized bool

	conn     *pgx.Conn
	connName string
	area     textarea.Model
	focus    editorFocus

	running bool
	err     string
	res     *db.QueryResult

	// results virtualization: selRow is the highlighted row (cursor);
	// rowOff is the first visible row; colOff is the first visible column.
	selRow int
	rowOff int
	colOff int

	width  int
	height int
}

func newEditor(conn *pgx.Conn, connName string) editor {
	ta := textarea.New()
	ta.Placeholder = "SELECT * FROM ..."
	ta.Focus()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0 // unlimited

	return editor{
		initialized: true,
		conn:        conn,
		connName:    connName,
		area:        ta,
		focus:       focusEditor,
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
		e.selRow, e.rowOff, e.colOff = 0, 0, 0
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

// bodyRows returns how many result rows fit in the results viewport. Pure.
func (e editor) bodyRows() int {
	// Reserve space for: editor (~h/3) + status + pane label + header +
	// divider + footer. Use a conservative split.
	n := e.height/2 - 3
	if n < 1 {
		n = 1
	}
	return n
}

// syncViewport scrolls rowOff so the selected row stays visible.
func (e *editor) syncViewport() {
	vis := e.bodyRows()
	if e.selRow < e.rowOff {
		e.rowOff = e.selRow
	}
	if e.selRow >= e.rowOff+vis {
		e.rowOff = e.selRow - vis + 1
	}
	if e.rowOff < 0 {
		e.rowOff = 0
	}
}

func (e editor) updateResultsKey(msg tea.KeyMsg) (editor, tea.Cmd) {
	if e.res == nil || !e.res.IsRowSet() {
		return e, nil
	}
	nRows := len(e.res.Rows)
	nCols := len(e.res.Columns)
	switch msg.String() {
	case "down", "j":
		if e.selRow < nRows-1 {
			e.selRow++
		}
	case "up", "k":
		if e.selRow > 0 {
			e.selRow--
		}
	case "pgdown", "ctrl+d":
		e.selRow += e.bodyRows()
		if e.selRow > nRows-1 {
			e.selRow = nRows - 1
		}
	case "pgup", "ctrl+u":
		e.selRow -= e.bodyRows()
		if e.selRow < 0 {
			e.selRow = 0
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
		e.selRow = 0
	case "end", "G":
		e.selRow = nRows - 1
		if e.selRow < 0 {
			e.selRow = 0
		}
	}
	e.syncViewport()
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
	paneHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	gridHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	gridDivStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	gridGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	gridSelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("24"))
	gridNullStyle   = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")).Italic(true)
	gridSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// colSep is drawn between columns.
const colSep = " │ "

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
		b.WriteString(editorErrStyle.Render("✗ " + truncateToWidth(e.err, max1(e.width-2))))
	case e.res != nil:
		b.WriteString(editorOKStyle.Render("✓ " + resultSummary(*e.res)))
	default:
		b.WriteString(paneHintStyle.Render("ready"))
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
		b.WriteString(e.renderGrid())
		b.WriteString("\n")
	}

	// Footer bar (connection name + row position on the left, keys on right).
	b.WriteString(e.footer())
	return b.String()
}

// max1 returns at least 1 (guards width math against tiny terminals).
func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// footer builds the styled, width-safe bottom bar.
func (e editor) footer() string {
	ctx := e.connName
	if e.res != nil && e.res.IsRowSet() && len(e.res.Rows) > 0 {
		ctx = fmt.Sprintf("%s  ·  row %d/%d", e.connName, e.selRow+1, len(e.res.Rows))
	}
	hints := []keyHint{
		{"ctrl+r", "run"},
		{"tab", "pane"},
		{"↑↓", "rows"},
		{"←→", "cols"},
		{"esc", "back"},
	}
	return footerBar(e.width, ctx, hints)
}

// renderGrid draws the virtualized results grid with a row cursor.
func (e editor) renderGrid() string {
	r := e.res
	if len(r.Rows) == 0 {
		return paneHintStyle.Render("(0 rows)")
	}

	vis := e.bodyRows()
	rowEnd := e.rowOff + vis
	if rowEnd > len(r.Rows) {
		rowEnd = len(r.Rows)
	}

	// Compute widths from the visible row window (virtualized).
	widths := computeColWidths(r.Columns, r.Rows, e.rowOff, rowEnd)

	// Gutter width holds the largest row number shown.
	gutterW := displayWidth(fmt.Sprintf("%d", rowEnd))
	if gutterW < 2 {
		gutterW = 2
	}

	// Determine how many columns fit horizontally, starting at colOff.
	avail := e.width - gutterW - 1 // gutter + a space
	if avail < minColWidth {
		avail = minColWidth
	}
	colStart := e.colOff
	if colStart >= len(r.Columns) {
		colStart = len(r.Columns) - 1
	}
	colEnd := colStart
	used := 0
	for c := colStart; c < len(r.Columns); c++ {
		w := widths[c]
		add := w
		if c > colStart {
			add += displayWidth(colSep)
		}
		if used+add > avail && c > colStart {
			break
		}
		used += add
		colEnd = c + 1
	}
	if colEnd <= colStart {
		colEnd = colStart + 1
	}

	var sb strings.Builder

	// Header row.
	sb.WriteString(gridGutterStyle.Render(padRight("#", gutterW)))
	sb.WriteString(" ")
	for c := colStart; c < colEnd; c++ {
		if c > colStart {
			sb.WriteString(gridSepStyle.Render(colSep))
		}
		sb.WriteString(gridHeaderStyle.Render(truncCell(r.Columns[c], widths[c])))
	}
	sb.WriteString("\n")

	// Divider.
	sb.WriteString(gridDivStyle.Render(strings.Repeat("─", gutterW)))
	sb.WriteString(gridDivStyle.Render("─"))
	for c := colStart; c < colEnd; c++ {
		if c > colStart {
			sb.WriteString(gridDivStyle.Render("─┼─"))
		}
		sb.WriteString(gridDivStyle.Render(strings.Repeat("─", widths[c])))
	}
	sb.WriteString("\n")

	// Body rows.
	for ri := e.rowOff; ri < rowEnd; ri++ {
		row := r.Rows[ri]
		selected := ri == e.selRow

		var line strings.Builder
		line.WriteString(gridGutterStyle.Render(padRight(fmt.Sprintf("%d", ri+1), gutterW)))
		line.WriteString(" ")
		for c := colStart; c < colEnd; c++ {
			if c > colStart {
				line.WriteString(gridSepStyle.Render(colSep))
			}
			var cell string
			if c < len(row) {
				cell = row[c]
			}
			rendered := truncCell(cell, widths[c])
			if cell == db.NullText {
				rendered = gridNullStyle.Render(rendered)
			}
			line.WriteString(rendered)
		}

		if selected {
			// Re-render the whole line with the selection background. Strip
			// embedded styles by rebuilding from plain text for a clean bar.
			sb.WriteString(gridSelStyle.Render(e.plainRow(ri, widths, gutterW, colStart, colEnd)))
		} else {
			sb.WriteString(line.String())
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// plainRow renders a row as unstyled text (used for the selection bar so the
// highlight background applies uniformly).
func (e editor) plainRow(ri int, widths []int, gutterW, colStart, colEnd int) string {
	row := e.res.Rows[ri]
	var sb strings.Builder
	sb.WriteString(padRight(fmt.Sprintf("%d", ri+1), gutterW))
	sb.WriteString(" ")
	for c := colStart; c < colEnd; c++ {
		if c > colStart {
			sb.WriteString(colSep)
		}
		var cell string
		if c < len(row) {
			cell = row[c]
		}
		sb.WriteString(truncCell(cell, widths[c]))
	}
	return sb.String()
}
