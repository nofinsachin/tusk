package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nofinsachin/tusk/internal/config"
)

// form field indices.
const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldDatabase
	fieldUser
	fieldSSLMode
	fieldCount
)

// connForm is the add/edit connection form.
type connForm struct {
	inputs   []textinput.Model
	focus    int
	editing  bool   // true = editing an existing connection
	origName string // original name when editing (for Update)
	err      string
}

func newConnForm() connForm {
	inputs := make([]textinput.Model, fieldCount)

	for i := range inputs {
		ti := textinput.New()
		ti.CharLimit = 256
		switch i {
		case fieldName:
			ti.Placeholder = "my-local-db"
			ti.Focus()
		case fieldHost:
			ti.Placeholder = "localhost"
		case fieldPort:
			ti.Placeholder = strconv.Itoa(config.DefaultPort)
		case fieldDatabase:
			ti.Placeholder = "postgres"
		case fieldUser:
			ti.Placeholder = "postgres"
		case fieldSSLMode:
			ti.Placeholder = config.DefaultSSLMode
		}
		inputs[i] = ti
	}

	return connForm{inputs: inputs, focus: fieldName}
}

// forConnection prepopulates the form for editing c.
func formForConnection(c config.Connection) connForm {
	f := newConnForm()
	f.editing = true
	f.origName = c.Name
	f.inputs[fieldName].SetValue(c.Name)
	f.inputs[fieldHost].SetValue(c.Host)
	if c.Port != 0 {
		f.inputs[fieldPort].SetValue(strconv.Itoa(c.Port))
	}
	f.inputs[fieldDatabase].SetValue(c.Database)
	f.inputs[fieldUser].SetValue(c.User)
	f.inputs[fieldSSLMode].SetValue(c.SSLMode)
	return f
}

// connection assembles a config.Connection from the current inputs.
// It validates the port and returns an error message if invalid.
func (f connForm) connection() (config.Connection, string) {
	portStr := strings.TrimSpace(f.inputs[fieldPort].Value())
	port := config.DefaultPort
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 || p > 65535 {
			return config.Connection{}, "port must be a number between 1 and 65535"
		}
		port = p
	}

	c := config.Connection{
		Name:     strings.TrimSpace(f.inputs[fieldName].Value()),
		Host:     strings.TrimSpace(f.inputs[fieldHost].Value()),
		Port:     port,
		Database: strings.TrimSpace(f.inputs[fieldDatabase].Value()),
		User:     strings.TrimSpace(f.inputs[fieldUser].Value()),
		SSLMode:  strings.TrimSpace(f.inputs[fieldSSLMode].Value()),
	}
	if c.Name == "" {
		return config.Connection{}, "name is required"
	}
	if c.Host == "" {
		c.Host = "localhost"
	}
	return c.WithDefaults(), ""
}

func (f *connForm) focusField(i int) {
	if i < 0 {
		i = 0
	}
	if i >= len(f.inputs) {
		i = len(f.inputs) - 1
	}
	for j := range f.inputs {
		if j == i {
			f.inputs[j].Focus()
		} else {
			f.inputs[j].Blur()
		}
	}
	f.focus = i
}

// update handles key input while the form is active. It returns the (possibly
// modified) form and any command to run. The caller decides submit/cancel via
// the returned action.
type formAction int

const (
	formNone formAction = iota
	formSubmit
	formCancel
)

func (f connForm) update(msg tea.Msg) (connForm, formAction, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return f, formCancel, nil
		case "enter":
			return f, formSubmit, nil
		case "tab", "down":
			f.focusField((f.focus + 1) % len(f.inputs))
			return f, formNone, nil
		case "shift+tab", "up":
			f.focusField((f.focus - 1 + len(f.inputs)) % len(f.inputs))
			return f, formNone, nil
		}
	}

	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return f, formNone, cmd
}

var (
	formLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Width(10)

	formErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63"))
)

func (f connForm) view() string {
	labels := []string{"Name", "Host", "Port", "Database", "User", "SSL mode"}

	var b strings.Builder
	title := "Add connection"
	if f.editing {
		title = "Edit connection"
	}
	b.WriteString(formTitleStyle.Render(title))
	b.WriteString("\n\n")

	for i := range f.inputs {
		b.WriteString(formLabelStyle.Render(labels[i]))
		b.WriteString(" ")
		b.WriteString(f.inputs[i].View())
		b.WriteString("\n")
	}

	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(formErrStyle.Render("✗ " + f.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab/↑↓ move · enter save · esc cancel"))
	return b.String()
}
