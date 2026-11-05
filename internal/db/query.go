package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// NullText is how a SQL NULL is rendered in results.
const NullText = "NULL"

// QueryResult holds the outcome of running a statement.
//
// For row-returning statements (SELECT, etc.), Columns and Rows are populated.
// For non-row statements (INSERT/UPDATE/DELETE/DDL), Columns/Rows are empty and
// Command carries the command tag (e.g. "INSERT 0 3"); RowsAffected is set when
// the tag reports it.
type QueryResult struct {
	Columns      []string
	Rows         [][]string
	Command      string
	RowsAffected int64
	Duration     time.Duration
}

// IsRowSet reports whether the result carries a column/row set to display.
func (r QueryResult) IsRowSet() bool {
	return len(r.Columns) > 0
}

// FormatValue renders a single scanned value as display text. It is pure and
// kept separate so it can be unit-tested without a database.
func FormatValue(v any) string {
	if v == nil {
		return NullText
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RunQuery executes sql on conn and returns a QueryResult. The supplied context
// governs cancellation and timeout; callers typically pass a context with a
// deadline.
func RunQuery(ctx context.Context, conn *pgx.Conn, sql string) (QueryResult, error) {
	start := time.Now()

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}

	var data [][]string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return QueryResult{}, fmt.Errorf("read row: %w", err)
		}
		rec := make([]string, len(vals))
		for i, v := range vals {
			rec[i] = FormatValue(v)
		}
		data = append(data, rec)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	tag := rows.CommandTag()
	res := QueryResult{
		Columns:      cols,
		Rows:         data,
		Command:      tag.String(),
		RowsAffected: tag.RowsAffected(),
		Duration:     time.Since(start),
	}
	return res, nil
}
