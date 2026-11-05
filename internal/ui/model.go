// Package ui contains the bubbletea model that drives the tusk TUI.
package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5"

	"github.com/nofinsachin/tusk/internal/config"
	"github.com/nofinsachin/tusk/internal/db"
)

// screen identifies which top-level view is active.
type screen int

const (
	screenManager screen = iota
	screenConnecting
	screenBrowser
	screenEditor
	screenMetrics
)

// --- lifecycle messages ---

// connOpenedMsg reports the result of opening a live connection.
type connOpenedMsg struct {
	conn *pgx.Conn
	name string
	err  error
}

// Model is the root application model.
type Model struct {
	screen  screen
	manager manager
	browser browser
	editor  editor
	metrics metrics

	// metricsGen increments each time the metrics screen is opened so that
	// ticks/fetches from a previous visit are ignored.
	metricsGen int

	// conn is the live connection while browsing; nil otherwise.
	conn     *pgx.Conn
	connName string
	connErr  string

	loadErr error
	width   int
	height  int
	ready   bool
}

// New returns an initialized root model, loading saved connections from disk.
func New() Model {
	store, err := config.Load()
	if err != nil {
		store = &config.Store{}
	}
	return Model{
		screen:  screenManager,
		manager: newManager(store),
		loadErr: err,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// openConnCmd opens a live connection off the UI goroutine.
func openConnCmd(c config.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		conn, err := db.Connect(context.Background(), c, password)
		return connOpenedMsg{conn: conn, name: c.Name, err: err}
	}
}

// closeConn closes any live connection, returning to a clean state.
func (m *Model) closeConn() {
	if m.conn != nil {
		_ = m.conn.Close(context.Background())
		m.conn = nil
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// The manager always exists. The browser and editor are created
		// lazily (on connect and on opening the editor) and are sized at
		// creation time from m.width/m.height, so only forward live resize
		// events to them once they have been initialized. Forwarding to an
		// uninitialized editor would call textarea.SetWidth on a zero-value
		// textarea.Model and panic.
		var cmd tea.Cmd
		m.manager, cmd = m.manager.Update(msg)
		if m.browser.initialized {
			b, _ := m.browser.Update(msg)
			m.browser = b
		}
		if m.editor.initialized {
			ed, _ := m.editor.Update(msg)
			m.editor = ed
		}
		if m.metrics.initialized {
			mt, _ := m.metrics.Update(msg)
			m.metrics = mt
		}
		return m, cmd

	case connectedMsg:
		// User chose a connection in the manager. Begin opening it.
		m.screen = screenConnecting
		m.connName = msg.conn.Name
		m.connErr = ""
		return m, openConnCmd(msg.conn, msg.password)

	case connOpenedMsg:
		if msg.err != nil {
			m.connErr = msg.err.Error()
			m.screen = screenManager
			return m, nil
		}
		m.conn = msg.conn
		m.connName = msg.name
		m.browser = newBrowser(msg.conn, msg.name)
		m.browser.setSize(m.width, m.height-1)
		m.screen = screenBrowser
		return m, m.browser.init()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Route other messages to the active screen.
	switch m.screen {
	case screenBrowser:
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	case screenEditor:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	case screenMetrics:
		var cmd tea.Cmd
		m.metrics, cmd = m.metrics.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.manager, cmd = m.manager.Update(msg)
		return m, cmd
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits, closing any live connection first.
	if msg.String() == "ctrl+c" {
		m.closeConn()
		return m, tea.Quit
	}

	switch m.screen {
	case screenBrowser:
		// While the filter input is focused, the browser owns all keys
		// (including 'b' and 'q' as literal text).
		if m.browser.filtering && m.browser.filterInput.Focused() {
			var cmd tea.Cmd
			m.browser, cmd = m.browser.Update(msg)
			return m, cmd
		}
		// 'b' returns to the connection manager, closing the connection.
		if msg.String() == "b" {
			m.closeConn()
			m.screen = screenManager
			return m, nil
		}
		if msg.String() == "q" {
			m.closeConn()
			return m, tea.Quit
		}
		// 'e' opens the query editor on the live connection.
		if msg.String() == "e" {
			m.editor = newEditor(m.conn, m.connName)
			m.editor.setSize(m.width, m.height-1)
			m.screen = screenEditor
			return m, textareaBlinkCmd()
		}
		// 'm' opens the auto-refreshing metrics view.
		if msg.String() == "m" {
			m.metricsGen++
			m.metrics = newMetrics(m.conn, m.connName, m.metricsGen)
			m.metrics.setSize(m.width, m.height-1)
			m.screen = screenMetrics
			return m, m.metrics.init()
		}
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd

	case screenMetrics:
		// esc/b return to the browser (which stops the ticker because ticks of
		// the current generation are no longer routed). q quits.
		switch msg.String() {
		case "esc", "b":
			m.screen = screenBrowser
			return m, nil
		case "q":
			m.closeConn()
			return m, tea.Quit
		case "r":
			// Manual refresh: issue an immediate fetch for the current gen.
			return m, fetchMetricsCmd(m.conn, m.metricsGen)
		}
		var cmd tea.Cmd
		m.metrics, cmd = m.metrics.Update(msg)
		return m, cmd

	case screenEditor:
		// esc returns to the browser; the editor owns every other key
		// (including 'q'/'b' which are valid SQL text).
		if msg.String() == "esc" {
			m.screen = screenBrowser
			return m, nil
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd

	case screenConnecting:
		// Allow aborting a hanging connect attempt.
		if msg.String() == "esc" {
			m.screen = screenManager
			return m, nil
		}
		return m, nil

	default: // screenManager
		if m.manager.mode == modeList &&
			m.manager.list.FilterState().String() != "filtering" {
			if msg.String() == "q" {
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		m.manager, cmd = m.manager.Update(msg)
		return m, cmd
	}
}

var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("15")).
	Background(lipgloss.Color("63")).
	Padding(0, 1)

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Loading tusk…"
	}

	switch m.screen {
	case screenConnecting:
		header := headerStyle.Render("🐘 tusk")
		return lipgloss.JoinVertical(lipgloss.Left,
			header, "\nConnecting to "+m.connName+"…  (esc to cancel)")

	case screenBrowser:
		header := headerStyle.Render("🐘 tusk — " + m.connName)
		return lipgloss.JoinVertical(lipgloss.Left, header, m.browser.View())

	case screenEditor:
		header := headerStyle.Render("🐘 tusk — " + m.connName + " · query")
		return lipgloss.JoinVertical(lipgloss.Left, header, m.editor.View())

	case screenMetrics:
		header := headerStyle.Render("🐘 tusk — " + m.connName + " · metrics")
		return lipgloss.JoinVertical(lipgloss.Left, header, m.metrics.View())

	default: // screenManager
		header := headerStyle.Render("🐘 tusk")
		var banner string
		if m.loadErr != nil {
			banner = errStyle.Render("config load error: "+m.loadErr.Error()) + "\n"
		}
		if m.connErr != "" {
			banner += errStyle.Render("connect error: "+m.connErr) + "\n"
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, banner+m.manager.View())
	}
}
