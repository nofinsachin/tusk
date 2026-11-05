// Package ui contains the bubbletea model that drives the tusk TUI.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nofinsachin/tusk/internal/config"
)

// Model is the root application model.
type Model struct {
	manager manager

	// connected holds the connection chosen via the manager. Browsing/query
	// screens will hang off this in later milestones.
	connected *config.Connection

	loadErr error
	width   int
	height  int
	ready   bool
}

// New returns an initialized root model, loading saved connections from disk.
func New() Model {
	store, err := config.Load()
	if err != nil {
		// Start with an empty store but surface the error in the UI.
		store = &config.Store{}
	}
	return Model{
		manager: newManager(store),
		loadErr: err,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		var cmd tea.Cmd
		m.manager, cmd = m.manager.Update(msg)
		return m, cmd

	case connectedMsg:
		c := msg.conn
		m.connected = &c
		return m, nil

	case tea.KeyMsg:
		// Global quit only when not capturing text in the form or filtering.
		if m.manager.mode == modeList &&
			m.manager.list.FilterState().String() != "filtering" {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		} else if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.manager, cmd = m.manager.Update(msg)
	return m, cmd
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

	header := headerStyle.Render("🐘 tusk")
	if m.connected != nil {
		header = headerStyle.Render("🐘 tusk — " + m.connected.Name)
	}

	var banner string
	if m.loadErr != nil {
		banner = errStyle.Render("config load error: "+m.loadErr.Error()) + "\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, banner+m.manager.View())
}
