package db

import (
	"testing"
	"time"
)

func TestFormatValue(t *testing.T) {
	ts := time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil is NULL", nil, NullText},
		{"string passthrough", "hello", "hello"},
		{"bytes to string", []byte("bytes"), "bytes"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"float", 3.14, "3.14"},
		{"time rfc3339", ts, "2026-11-05T14:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatValue(c.in); got != c.want {
				t.Errorf("FormatValue(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsRowSet(t *testing.T) {
	if (QueryResult{}).IsRowSet() {
		t.Error("empty result should not be a row set")
	}
	if !(QueryResult{Columns: []string{"id"}}).IsRowSet() {
		t.Error("result with columns should be a row set")
	}
}
