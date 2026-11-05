package ui

import (
	"strings"
	"testing"
)

func TestComputeColWidthsWidestVisible(t *testing.T) {
	cols := []string{"id", "name"}
	rows := [][]string{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "a-very-long-name-here"},
	}

	// Measure only the first two rows: longest 'name' is "alice" (5).
	w := computeColWidths(cols, rows, 0, 2)
	if w[0] != minColWidth { // "id"=2, "1"/"2"=1 -> clamped up to minColWidth
		t.Errorf("col0 width = %d, want min %d", w[0], minColWidth)
	}
	if w[1] != 5 {
		t.Errorf("col1 width = %d, want 5 (alice)", w[1])
	}

	// Measure all rows: the long name should drive col1 width.
	w = computeColWidths(cols, rows, 0, len(rows))
	if w[1] != len("a-very-long-name-here") {
		t.Errorf("col1 width = %d, want %d", w[1], len("a-very-long-name-here"))
	}
}

func TestComputeColWidthsCapped(t *testing.T) {
	cols := []string{"blob"}
	long := strings.Repeat("x", 100)
	rows := [][]string{{long}}
	w := computeColWidths(cols, rows, 0, 1)
	if w[0] != maxColWidth {
		t.Errorf("width = %d, want capped at %d", w[0], maxColWidth)
	}
}

func TestComputeColWidthsHeaderCounts(t *testing.T) {
	cols := []string{"a_long_header_name"}
	rows := [][]string{{"x"}}
	w := computeColWidths(cols, rows, 0, 1)
	if w[0] != len("a_long_header_name") {
		t.Errorf("width = %d, want header width %d", w[0], len("a_long_header_name"))
	}
}

func TestFooterBarNeverExceedsWidth(t *testing.T) {
	hints := []keyHint{{"ctrl+r", "run"}, {"tab", "pane"}, {"esc", "back"}}
	for _, width := range []int{10, 20, 40, 80, 120} {
		out := footerBar(width, "mydb  ·  row 3/120", hints)
		// Strip ANSI styling to measure the visible width.
		plain := stripANSI(out)
		if displayWidth(plain) > width {
			t.Errorf("width=%d: footer visible width %d exceeds limit:\n%q",
				width, displayWidth(plain), plain)
		}
	}
}

func TestFooterBarIncludesContextWhenRoomy(t *testing.T) {
	hints := []keyHint{{"ctrl+r", "run"}}
	out := stripANSI(footerBar(120, "mydb · row 1/10", hints))
	if !strings.Contains(out, "mydb") {
		t.Errorf("roomy footer should contain context: %q", out)
	}
	if !strings.Contains(out, "run") {
		t.Errorf("roomy footer should contain key action: %q", out)
	}
}

// stripANSI removes ANSI escape sequences so tests can measure visible width.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inEsc {
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		if c == 0x1b { // ESC
			inEsc = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
