package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// maxColWidth caps how wide any single column may be rendered. Values wider
// than this are truncated with an ellipsis.
const maxColWidth = 40

// minColWidth keeps even tiny columns readable (and able to show an ellipsis).
const minColWidth = 3

// displayWidth returns the terminal cell width of s (handling wide runes).
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncCell fits s into exactly w display cells: right-padded with spaces when
// short, or truncated with a trailing ellipsis when too wide. It first strips
// newlines/tabs so a cell is always a single line.
func truncCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = collapseWhitespace(s)
	return runewidth.FillRight(runewidth.Truncate(s, w, "…"), w)
}

// collapseWhitespace replaces newlines and tabs with single spaces so values
// never break the single-line grid layout.
func collapseWhitespace(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case '\n', '\r', '\t':
			out = append(out, ' ')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// computeColWidths returns the render width for each column, sized to the
// widest visible cell (including the header), clamped to [minColWidth,
// maxColWidth].
//
// rows is the full result set; only the rows in [rowStart, rowEnd) are measured
// so that width is based on what is actually on screen (virtualized). It is
// pure and does not depend on any live data source.
func computeColWidths(columns []string, rows [][]string, rowStart, rowEnd int) []int {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = displayWidth(c)
	}

	if rowStart < 0 {
		rowStart = 0
	}
	if rowEnd > len(rows) {
		rowEnd = len(rows)
	}
	for r := rowStart; r < rowEnd; r++ {
		for i, cell := range rows[r] {
			if i >= len(widths) {
				break
			}
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	for i := range widths {
		if widths[i] > maxColWidth {
			widths[i] = maxColWidth
		}
		if widths[i] < minColWidth {
			widths[i] = minColWidth
		}
	}
	return widths
}

// truncateToWidth fits a plain (unstyled) string to w display cells, adding a
// trailing ellipsis if it must be cut. Used for the footer bar so it never
// overflows or wraps the terminal width.
func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if displayWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}

// padRight pads s with spaces to exactly w display cells (no truncation).
func padRight(s string, w int) string {
	if displayWidth(s) >= w {
		return s
	}
	return runewidth.FillRight(s, w)
}

// styleOrEmpty applies a style only to non-empty strings (keeps zero-width
// cells from acquiring background color artifacts).
func styleOrEmpty(st lipgloss.Style, s string) string {
	if s == "" {
		return s
	}
	return st.Render(s)
}
