// Command tusk is a fast, keyboard-driven Postgres manager for the terminal.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nofinsachin/tusk/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tusk: %v\n", err)
		os.Exit(1)
	}
}
