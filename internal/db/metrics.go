package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TableSize describes one relation's on-disk size.
type TableSize struct {
	Schema string
	Name   string
	Bytes  int64
}

// Metrics is a point-in-time snapshot of server storage and activity.
//
// The transaction and block counters are cumulative values straight from
// pg_stat_database; rates (TPS, cache-hit ratio) are derived by comparing two
// snapshots — see TPS and CacheHitRatio.
type Metrics struct {
	Database  string
	DBSize    int64       // pg_database_size of the connected database
	TopTables []TableSize // largest relations in the connected database

	// connection counts from pg_stat_activity
	ConnTotal  int
	ConnActive int
	ConnIdle   int

	// cumulative counters from pg_stat_database (current database)
	XactCommit   int64
	XactRollback int64
	BlksHit      int64
	BlksRead     int64

	TakenAt time.Time
}

// topTablesLimit bounds how many large tables we list.
const topTablesLimit = 10

// FetchMetrics collects a storage + activity snapshot for the connected
// database. It issues a few lightweight catalog queries.
func FetchMetrics(ctx context.Context, conn *pgx.Conn) (Metrics, error) {
	var m Metrics
	m.TakenAt = time.Now()

	// Database name + size.
	err := conn.QueryRow(ctx,
		`SELECT current_database(), pg_database_size(current_database())`).
		Scan(&m.Database, &m.DBSize)
	if err != nil {
		return Metrics{}, fmt.Errorf("db size: %w", err)
	}

	// Connection counts (this database). state can be NULL for some backends.
	err = conn.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE state = 'active'),
			count(*) FILTER (WHERE state = 'idle')
		FROM pg_stat_activity
		WHERE datname = current_database()`).
		Scan(&m.ConnTotal, &m.ConnActive, &m.ConnIdle)
	if err != nil {
		return Metrics{}, fmt.Errorf("connections: %w", err)
	}

	// Cumulative xact + block counters for the current database.
	err = conn.QueryRow(ctx, `
		SELECT
			COALESCE(xact_commit, 0),
			COALESCE(xact_rollback, 0),
			COALESCE(blks_hit, 0),
			COALESCE(blks_read, 0)
		FROM pg_stat_database
		WHERE datname = current_database()`).
		Scan(&m.XactCommit, &m.XactRollback, &m.BlksHit, &m.BlksRead)
	if err != nil {
		return Metrics{}, fmt.Errorf("stat_database: %w", err)
	}

	// Top tables by total size (incl. indexes/toast) in the current database.
	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname,
		       pg_total_relation_size(c.oid) AS total_bytes
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p', 'm')
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		ORDER BY total_bytes DESC
		LIMIT $1`, topTablesLimit)
	if err != nil {
		return Metrics{}, fmt.Errorf("top tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t TableSize
		if err := rows.Scan(&t.Schema, &t.Name, &t.Bytes); err != nil {
			return Metrics{}, fmt.Errorf("scan table size: %w", err)
		}
		m.TopTables = append(m.TopTables, t)
	}
	if err := rows.Err(); err != nil {
		return Metrics{}, fmt.Errorf("top tables: %w", err)
	}

	return m, nil
}

// CacheHitRatio returns the fraction of buffer requests served from cache,
// derived from the delta between an earlier snapshot (prev) and m. A value in
// [0,1] is returned; if there were no reads in the interval it returns 1.0
// (everything served from cache), and if prev is zero-valued it falls back to
// the cumulative ratio. Pure and testable.
func (m Metrics) CacheHitRatio(prev Metrics) float64 {
	hit := m.BlksHit - prev.BlksHit
	read := m.BlksRead - prev.BlksRead
	// Guard against counter resets or an empty prev: fall back to cumulative.
	if hit < 0 || read < 0 || (prev.BlksHit == 0 && prev.BlksRead == 0) {
		hit = m.BlksHit
		read = m.BlksRead
	}
	total := hit + read
	if total <= 0 {
		return 1.0
	}
	return float64(hit) / float64(total)
}

// TPS returns transactions per second (commits + rollbacks) between prev and m.
// Returns 0 when the interval is non-positive or prev is zero-valued. Pure.
func (m Metrics) TPS(prev Metrics) float64 {
	if prev.TakenAt.IsZero() {
		return 0
	}
	secs := m.TakenAt.Sub(prev.TakenAt).Seconds()
	if secs <= 0 {
		return 0
	}
	dx := (m.XactCommit + m.XactRollback) - (prev.XactCommit + prev.XactRollback)
	if dx < 0 {
		return 0
	}
	return float64(dx) / secs
}

// FormatBytes renders a byte count as a human-readable size (KiB/MiB/GiB...).
// Pure and testable.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
