package ui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nofinsachin/tusk/internal/config"
	"github.com/nofinsachin/tusk/internal/db"
)

// textinputBlink is the cursor-blink command for freshly focused forms.
func textinputBlink() tea.Cmd { return textinput.Blink }

// connItem adapts a config.Connection to the bubbles list.Item interface.
type connItem struct {
	conn config.Connection
}

func (i connItem) Title() string { return i.conn.Name }
func (i connItem) Description() string {
	return fmt.Sprintf("%s@%s:%d/%s (sslmode=%s)",
		i.conn.User, i.conn.Host, i.conn.Port, i.conn.Database, i.conn.SSLMode)
}
func (i connItem) FilterValue() string { return i.conn.Name }

// manager mode.
type managerMode int

const (
	modeList managerMode = iota
	modeForm
)

// --- messages ---

// testResultMsg reports the outcome of a connection test.
type testResultMsg struct {
	name string
	err  error
}

// connectedMsg signals a successful connect request for a connection.
type connectedMsg struct {
	conn config.Connection
}

// manager is the connection-manager screen.
type manager struct {
	store  *config.Store
	list   list.Model
	form   connForm
	mode   managerMode
	status string
	width  int
	height int
}

func newManager(store *config.Store) manager {
	items := make([]list.Item, 0, len(store.Connections))
	for _, c := range store.Connections {
		items = append(items, connItem{conn: c})
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Connections"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return manager{
		store: store,
		list:  l,
		mode:  modeList,
	}
}

func (m *manager) setSize(w, h int) {
	m.width = w
	m.height = h
	// Leave room for the status/help line.
	m.list.SetSize(w, h-2)
}

func (m *manager) reloadItems() {
	items := make([]list.Item, 0, len(m.store.Connections))
	for _, c := range m.store.Connections {
		items = append(items, connItem{conn: c})
	}
	m.list.SetItems(items)
}

func (m *manager) selectedConn() (config.Connection, bool) {
	it, ok := m.list.SelectedItem().(connItem)
	if !ok {
		return config.Connection{}, false
	}
	return it.conn, true
}

// testCmd runs a connection ping off the UI goroutine.
func testCmd(c config.Connection) tea.Cmd {
	return func() tea.Msg {
		err := db.Ping(context.Background(), c, "")
		return testResultMsg{name: c.Name, err: err}
	}
}

func (m manager) Update(msg tea.Msg) (manager, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case testResultMsg:
		if msg.err != nil {
			m.status = statusErr("test " + msg.name + ": " + msg.err.Error())
		} else {
			m.status = statusOK("test " + msg.name + ": connection OK")
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeForm {
			return m.updateForm(msg)
		}
		return m.updateList(msg)
	}

	// Non-key messages: route to the active sub-component.
	if m.mode == modeForm {
		var cmd tea.Cmd
		m.form, _, cmd = m.form.update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m manager) updateList(msg tea.KeyMsg) (manager, tea.Cmd) {
	// Don't steal keys while the user is filtering the list.
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "a":
		m.form = newConnForm()
		m.mode = modeForm
		m.status = ""
		return m, textinputBlink()
	case "e":
		if c, ok := m.selectedConn(); ok {
			m.form = formForConnection(c)
			m.mode = modeForm
			m.status = ""
			return m, textinputBlink()
		}
	case "d":
		if c, ok := m.selectedConn(); ok {
			m.store.Remove(c.Name)
			if err := m.store.Save(); err != nil {
				m.status = statusErr("delete: " + err.Error())
			} else {
				m.reloadItems()
				m.status = statusOK("deleted " + c.Name)
			}
		}
		return m, nil
	case "t":
		if c, ok := m.selectedConn(); ok {
			m.status = "testing " + c.Name + "…"
			return m, testCmd(c)
		}
	case "enter":
		if c, ok := m.selectedConn(); ok {
			return m, func() tea.Msg { return connectedMsg{conn: c} }
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m manager) updateForm(msg tea.KeyMsg) (manager, tea.Cmd) {
	var action formAction
	var cmd tea.Cmd
	m.form, action, cmd = m.form.update(msg)

	switch action {
	case formCancel:
		m.mode = modeList
		m.status = ""
		return m, nil
	case formSubmit:
		c, errMsg := m.form.connection()
		if errMsg != "" {
			m.form.err = errMsg
			return m, nil
		}
		var saveErr error
		if m.form.editing {
			saveErr = m.store.Update(m.form.origName, c)
		} else {
			saveErr = m.store.Add(c)
		}
		if saveErr != nil {
			m.form.err = saveErr.Error()
			return m, nil
		}
		if err := m.store.Save(); err != nil {
			m.form.err = "save failed: " + err.Error()
			return m, nil
		}
		m.reloadItems()
		m.mode = modeList
		m.status = statusOK("saved " + c.Name)
		return m, nil
	}
	return m, cmd
}

func (m manager) View() string {
	if m.mode == modeForm {
		body := m.form.view()
		return body
	}

	status := m.status
	if status == "" {
		status = helpStyle.Render("a add · e edit · d delete · t test · enter connect · / filter · q quit")
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), status)
}

// --- status styling helpers ---

var (
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

func statusOK(s string) string  { return okStyle.Render("✓ " + s) }
func statusErr(s string) string { return errStyle.Render("✗ " + s) }
