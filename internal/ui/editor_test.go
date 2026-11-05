package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/nofinsachin/tusk/internal/db"
)

func TestResultSummaryRowSet(t *testing.T) {
	r := db.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]string{{"1", "a"}, {"2", "b"}, {"3", "c"}},
		Duration: 12 * time.Millisecond,
	}
	got := resultSummary(r)
	if !strings.Contains(got, "3 row(s)") || !strings.Contains(got, "2 col(s)") {
		t.Errorf("summary = %q, want row/col counts", got)
	}
	if !strings.Contains(got, "12ms") {
		t.Errorf("summary = %q, want duration", got)
	}
}

func TestResultSummaryCommand(t *testing.T) {
	r := db.QueryResult{
		Command:      "INSERT 0 3",
		RowsAffected: 3,
		Duration:     5 * time.Millisecond,
	}
	got := resultSummary(r)
	if !strings.Contains(got, "INSERT 0 3") {
		t.Errorf("summary = %q, want command tag", got)
	}
	if strings.Contains(got, "row(s)") {
		t.Errorf("non-rowset summary should not mention row(s): %q", got)
	}
}

func TestPad(t *testing.T) {
	// Short value is right-padded to width.
	if got := pad("hi", 5); got != "hi   " {
		t.Errorf("pad short = %q, want %q", got, "hi   ")
	}
	// Exact width unchanged.
	if got := pad("hello", 5); got != "hello" {
		t.Errorf("pad exact = %q, want %q", got, "hello")
	}
	// Overlong is truncated with an ellipsis, preserving total width in runes.
	got := pad("abcdefgh", 5)
	if r := []rune(got); len(r) != 5 {
		t.Errorf("pad long width = %d, want 5 (%q)", len(r), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("pad long should end with ellipsis: %q", got)
	}
	// Newlines/tabs collapsed to spaces.
	if got := pad("a\nb", 3); strings.ContainsAny(got, "\n\t") {
		t.Errorf("pad should strip newlines/tabs: %q", got)
	}
}
