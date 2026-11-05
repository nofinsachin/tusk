package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestWindowSizeBeforeEditorInit is a regression test for a nil-pointer panic:
// the root model used to forward WindowSizeMsg to a zero-value editor, whose
// uninitialized textarea.Model panicked inside SetWidth. A resize that arrives
// before the editor (or browser) is created must be handled safely.
func TestWindowSizeBeforeEditorInit(t *testing.T) {
	m := New() // only the manager exists; browser/editor are zero values.

	if m.editor.initialized {
		t.Fatal("precondition: editor should not be initialized on a fresh model")
	}
	if m.browser.initialized {
		t.Fatal("precondition: browser should not be initialized on a fresh model")
	}

	// This previously panicked.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 236, Height: 60})
}

// TestWindowSizeResizesInitializedEditor verifies that once the editor exists,
// a resize is forwarded and applied without panicking.
func TestWindowSizeResizesInitializedEditor(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40

	// Create the editor as the connect flow would (no live conn needed here;
	// setSize/SetWidth only touch the textarea).
	m.editor = newEditor(nil, "testconn")
	m.editor.setSize(m.width, m.height-1)
	m.screen = screenEditor

	// A subsequent resize must be applied safely.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	got := updated.(Model)
	if got.width != 120 || got.height != 50 {
		t.Errorf("model size = %dx%d, want 120x50", got.width, got.height)
	}
}
