package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footer styling. The bar has a subtle background that spans the full width;
// key names are emphasized against the surrounding muted text.
var (
	footerBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250"))

	footerKeyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("228")).
			Bold(true)

	footerSepStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("240"))

	footerCtxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("153"))
)

// keyHint is a single "key action" pair shown in the footer.
type keyHint struct {
	key    string
	action string
}

// footerBar renders a single-line footer spanning width cells.
//
// Layout: on the left, a context segment (e.g. connection name and row
// position); on the right, emphasized key hints. The whole line is truncated
// to width so it can never wrap or overflow the terminal. It is built from a
// plain-text layout first (for correct width math) and then styled.
func footerBar(width int, context string, hints []keyHint) string {
	if width <= 0 {
		return ""
	}

	// Build the plain hint string to measure, and a parallel styled version.
	var plainHints strings.Builder
	var styledHints strings.Builder
	for i, h := range hints {
		if i > 0 {
			plainHints.WriteString("  ")
			styledHints.WriteString(footerSepStyle.Render("  "))
		}
		plainHints.WriteString(h.key)
		plainHints.WriteString(" ")
		plainHints.WriteString(h.action)

		styledHints.WriteString(footerKeyStyle.Render(h.key))
		styledHints.WriteString(footerBarStyle.Render(" "))
		styledHints.WriteString(footerBarStyle.Render(h.action))
	}

	plainCtx := context
	hintsWidth := displayWidth(plainHints.String())
	ctxWidth := displayWidth(plainCtx)

	// Decide what fits. Priority: context first, then hints, then a gap.
	// If everything fits, left-align context and right-align hints with a gap.
	const gapMin = 1
	if ctxWidth+gapMin+hintsWidth <= width {
		gap := width - ctxWidth - hintsWidth
		left := footerCtxStyle.Render(plainCtx)
		mid := footerBarStyle.Render(strings.Repeat(" ", gap))
		return left + mid + styledHints.String()
	}

	// Not enough room for both. Prefer the hints (navigation matters most);
	// show as much context as fits before them, truncated.
	if hintsWidth <= width {
		remaining := width - hintsWidth
		if remaining <= 0 {
			return styledHints.String()
		}
		truncCtx := truncateToWidth(plainCtx, remaining-gapMin)
		pad := remaining - displayWidth(truncCtx)
		if pad < 0 {
			pad = 0
		}
		left := footerCtxStyle.Render(truncCtx)
		mid := footerBarStyle.Render(strings.Repeat(" ", pad))
		return left + mid + styledHints.String()
	}

	// Even the hints alone don't fit: truncate the whole plain line.
	plain := plainCtx
	if plain != "" {
		plain += "  "
	}
	plain += plainHints.String()
	return footerBarStyle.Render(padRight(truncateToWidth(plain, width), width))
}
