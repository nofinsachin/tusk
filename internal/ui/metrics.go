package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5"

	"github.com/nofinsachin/tusk/internal/db"
)

// metricsInterval is how often the view auto-refreshes.
const metricsInterval = 2 * time.Second

// metricsFetchTimeout bounds a single metrics collection.
const metricsFetchTimeout = 5 * time.Second

// --- messages ---

// metricsMsg carries a fetched snapshot (or an error) tagged with the
// generation that requested it, so stale results are ignored after the screen
// is closed and reopened.
type metricsMsg struct {
	gen int
	m   db.Metrics
	err error
}

// metricsTickMsg is the periodic refresh trigger.
type metricsTickMsg struct {
	gen int
}

// metrics is the server metrics screen.
type metrics struct {
	initialized bool

	conn     *pgx.Conn
	connName string

	cur  *db.Metrics // latest snapshot
	prev db.Metrics  // previous snapshot (for TPS / cache-hit deltas)

	loading bool
	err     string

	// gen increments each time the screen is (re)opened; ticks/fetches from an
	// older generation are discarded.
	gen int

	width  int
	height int
}

func newMetrics(conn *pgx.Conn, connName string, gen int) metrics {
	return metrics{
		initialized: true,
		conn:        conn,
		connName:    connName,
		loading:     true,
		gen:         gen,
	}
}

func (mt *metrics) setSize(w, h int) {
	mt.width = w
	mt.height = h
}

// --- commands ---

func fetchMetricsCmd(conn *pgx.Conn, gen int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), metricsFetchTimeout)
		defer cancel()
		m, err := db.FetchMetrics(ctx, conn)
		return metricsMsg{gen: gen, m: m, err: err}
	}
}

func metricsTickCmd(gen int) tea.Cmd {
	return tea.Tick(metricsInterval, func(time.Time) tea.Msg {
		return metricsTickMsg{gen: gen}
	})
}

// init kicks off the first fetch and starts the ticker.
func (mt metrics) init() tea.Cmd {
	return tea.Batch(fetchMetricsCmd(mt.conn, mt.gen), metricsTickCmd(mt.gen))
}

func (mt metrics) Update(msg tea.Msg) (metrics, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mt.setSize(msg.Width, msg.Height)
		return mt, nil

	case metricsTickMsg:
		if msg.gen != mt.gen {
			return mt, nil // stale ticker from a previous open
		}
		// Fire a fetch and schedule the next tick.
		return mt, tea.Batch(fetchMetricsCmd(mt.conn, mt.gen), metricsTickCmd(mt.gen))

	case metricsMsg:
		if msg.gen != mt.gen {
			return mt, nil // stale result
		}
		mt.loading = false
		if msg.err != nil {
			mt.err = msg.err.Error()
			return mt, nil
		}
		mt.err = ""
		if mt.cur != nil {
			mt.prev = *mt.cur
		}
		snap := msg.m
		mt.cur = &snap
		return mt, nil
	}
	return mt, nil
}

// --- styling ---

var (
	metSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	metLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(16)
	metValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("231"))
	metBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	metBarBgStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	metErrStyle2    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func (mt metrics) View() string {
	if mt.loading && mt.cur == nil && mt.err == "" {
		return "Loading metrics…"
	}
	if mt.err != "" && mt.cur == nil {
		return metErrStyle2.Render("✗ "+mt.err) + "\n\n" + mt.footer()
	}

	m := mt.cur
	var b strings.Builder

	// Storage section.
	b.WriteString(metSectionStyle.Render("Storage"))
	b.WriteString("\n")
	b.WriteString(kv("database", m.Database))
	b.WriteString(kv("size", db.FormatBytes(m.DBSize)))
	b.WriteString("\n")

	if len(m.TopTables) > 0 {
		b.WriteString(metLabelStyle.Render("top tables"))
		b.WriteString("\n")
		maxBytes := m.TopTables[0].Bytes
		for _, t := range m.TopTables {
			name := t.Schema + "." + t.Name
			bar := sizeBar(t.Bytes, maxBytes, 24)
			line := fmt.Sprintf("  %-28s %s %10s",
				truncateToWidth(name, 28), bar, db.FormatBytes(t.Bytes))
			b.WriteString(truncateToWidth(line, max1(mt.width)))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Activity section.
	b.WriteString(metSectionStyle.Render("Activity / Load"))
	b.WriteString("\n")
	b.WriteString(kv("connections", fmt.Sprintf("%d total · %d active · %d idle",
		m.ConnTotal, m.ConnActive, m.ConnIdle)))
	b.WriteString(kv("cache hit", fmt.Sprintf("%.1f%%", m.CacheHitRatio(mt.prev)*100)))
	tps := m.TPS(mt.prev)
	tpsStr := fmt.Sprintf("%.1f /s", tps)
	if mt.prev.TakenAt.IsZero() {
		tpsStr = "sampling…"
	}
	b.WriteString(kv("transactions", tpsStr))
	b.WriteString("\n")

	// A small note clarifying the CPU/activity framing.
	b.WriteString(metBarBgStyle.Render(truncateToWidth(
		"note: true host CPU% isn't available via SQL; activity/load shown instead.",
		max1(mt.width))))
	b.WriteString("\n\n")

	b.WriteString(mt.footer())
	return b.String()
}

func (mt metrics) footer() string {
	ctx := mt.connName
	if mt.cur != nil {
		ctx = fmt.Sprintf("%s  ·  updated %s", mt.connName, mt.cur.TakenAt.Format("15:04:05"))
	}
	hints := []keyHint{
		{"r", "refresh"},
		{"b", "back"},
		{"q", "quit"},
	}
	return footerBar(mt.width, ctx, hints)
}

// kv renders a label/value line.
func kv(label, value string) string {
	return metLabelStyle.Render(label) + metValueStyle.Render(value) + "\n"
}

// sizeBar renders a proportional bar of the given width for v relative to max.
func sizeBar(v, max int64, width int) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	filled := int(float64(v) / float64(max) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return metBarStyle.Render(strings.Repeat("█", filled)) +
		metBarBgStyle.Render(strings.Repeat("░", width-filled))
}
