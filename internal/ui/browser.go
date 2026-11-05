package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5"

	"github.com/nofinsachin/tusk/internal/db"
)

// --- messages ---

// schemasLoadedMsg carries the result of the initial schema listing.
type schemasLoadedMsg struct {
	schemas []db.Schema
	err     error
}

// relationsLoadedMsg carries relations for a specific schema node.
type relationsLoadedMsg struct {
	target *node
	rels   []db.Relation
	err    error
}

// columnsLoadedMsg carries columns for a specific relation node.
type columnsLoadedMsg struct {
	target *node
	cols   []db.Column
	err    error
}

// browser is the schema-browser screen.
type browser struct {
	conn    *pgx.Conn
	connName string

	roots  []*node
	cursor int // index into the flattened visible rows

	loading bool
	err     string

	width  int
	height int
}

func newBrowser(conn *pgx.Conn, connName string) browser {
	return browser{
		conn:     conn,
		connName: connName,
		loading:  true,
	}
}

func (b *browser) setSize(w, h int) {
	b.width = w
	b.height = h
}

// --- commands ---

func loadSchemasCmd(conn *pgx.Conn) tea.Cmd {
	return func() tea.Msg {
		schemas, err := db.ListSchemas(context.Background(), conn, false)
		return schemasLoadedMsg{schemas: schemas, err: err}
	}
}

func loadRelationsCmd(conn *pgx.Conn, target *node) tea.Cmd {
	return func() tea.Msg {
		rels, err := db.ListRelations(context.Background(), conn, target.schema)
		return relationsLoadedMsg{target: target, rels: rels, err: err}
	}
}

func loadColumnsCmd(conn *pgx.Conn, target *node) tea.Cmd {
	return func() tea.Msg {
		cols, err := db.ListColumns(context.Background(), conn, target.schema, target.relation)
		return columnsLoadedMsg{target: target, cols: cols, err: err}
	}
}

// init kicks off the initial schema load.
func (b browser) init() tea.Cmd {
	return loadSchemasCmd(b.conn)
}

func (b browser) rows() []visibleRow {
	return flatten(b.roots)
}

// toggle expands or collapses the node at the cursor, triggering a lazy load
// of children when expanding an unloaded node.
func (b browser) toggle() (browser, tea.Cmd) {
	rows := b.rows()
	if b.cursor < 0 || b.cursor >= len(rows) {
		return b, nil
	}
	n := rows[b.cursor].n
	if !n.hasChildren() {
		return b, nil
	}

	if n.expanded {
		n.expanded = false
		return b, nil
	}

	// Expanding.
	n.expanded = true
	if n.loaded || n.loading {
		return b, nil
	}
	n.loading = true
	switch n.kind {
	case nodeSchema:
		return b, loadRelationsCmd(b.conn, n)
	case nodeRelation:
		return b, loadColumnsCmd(b.conn, n)
	}
	return b, nil
}

func (b browser) Update(msg tea.Msg) (browser, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.setSize(msg.Width, msg.Height)
		return b, nil

	case schemasLoadedMsg:
		b.loading = false
		if msg.err != nil {
			b.err = msg.err.Error()
			return b, nil
		}
		b.roots = schemaNodes(msg.schemas)
		b.cursor = 0
		return b, nil

	case relationsLoadedMsg:
		msg.target.loading = false
		msg.target.loaded = true
		if msg.err != nil {
			msg.target.err = msg.err.Error()
			msg.target.expanded = false
			return b, nil
		}
		msg.target.children = relationNodes(msg.rels)
		return b, nil

	case columnsLoadedMsg:
		msg.target.loading = false
		msg.target.loaded = true
		if msg.err != nil {
			msg.target.err = msg.err.Error()
			msg.target.expanded = false
			return b, nil
		}
		msg.target.children = columnNodes(msg.cols)
		return b, nil

	case tea.KeyMsg:
		return b.updateKey(msg)
	}
	return b, nil
}

func (b browser) updateKey(msg tea.KeyMsg) (browser, tea.Cmd) {
	rows := b.rows()
	switch msg.String() {
	case "up", "k":
		if b.cursor > 0 {
			b.cursor--
		}
	case "down", "j":
		if b.cursor < len(rows)-1 {
			b.cursor++
		}
	case "home", "g":
		b.cursor = 0
	case "end", "G":
		b.cursor = len(rows) - 1
		if b.cursor < 0 {
			b.cursor = 0
		}
	case "enter", " ", "right", "l":
		return b.toggle()
	case "left", "h":
		// Collapse if expanded; otherwise jump to parent-ish behavior is skipped
		// for simplicity (collapse only).
		if b.cursor >= 0 && b.cursor < len(rows) {
			n := rows[b.cursor].n
			if n.hasChildren() && n.expanded {
				n.expanded = false
			}
		}
	case "r":
		// Refresh: reload schemas from scratch.
		b.roots = nil
		b.cursor = 0
		b.loading = true
		b.err = ""
		return b, loadSchemasCmd(b.conn)
	}
	return b, nil
}

// --- styling ---

var (
	treeCursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63"))

	treeIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	treeTypeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	treeErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func (b browser) View() string {
	if b.loading && len(b.roots) == 0 {
		return "Loading schemas…"
	}
	if b.err != "" {
		return treeErrStyle.Render("✗ "+b.err) + "\n\n" +
			helpStyle.Render("r retry · b back · q quit")
	}

	rows := b.rows()
	if len(rows) == 0 {
		return helpStyle.Render("(no schemas)") + "\n\n" +
			helpStyle.Render("r refresh · b back · q quit")
	}

	// Vertical window around the cursor so large trees stay navigable.
	viewport := b.height - 2
	if viewport < 1 {
		viewport = 1
	}
	start := 0
	if b.cursor >= viewport {
		start = b.cursor - viewport + 1
	}
	end := start + viewport
	if end > len(rows) {
		end = len(rows)
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		row := rows[i]
		n := row.n
		indent := strings.Repeat("  ", row.depth)
		line := indent + treeIconStyle.Render(n.icon()) + " " + n.label
		if t := n.typeLabel(); t != "" {
			line += "  " + treeTypeStyle.Render("["+t+"]")
		}
		if n.err != "" {
			line += "  " + treeErrStyle.Render("("+n.err+")")
		}
		if i == b.cursor {
			line = treeCursorStyle.Render(line)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	help := helpStyle.Render("↑↓ move · enter/→ expand · ← collapse · r refresh · b back · q quit")
	return sb.String() + "\n" + help
}
