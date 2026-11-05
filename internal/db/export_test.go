package db

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sampleResult() QueryResult {
	return QueryResult{
		Columns: []string{"id", "name", "note"},
		Rows: [][]string{
			{"1", "alice", "hi"},
			{"2", "bob,jr", "line1\nline2"},
			{"3", "NULL", ""}, // the string "NULL" is NOT a SQL null here
		},
		Nulls: [][]bool{
			{false, false, false},
			{false, false, false},
			{false, false, true}, // only the last cell is a real SQL NULL
		},
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, sampleResult()); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()

	// Header present (encoding/csv uses \n line terminators by default).
	if !strings.HasPrefix(out, "id,name,note\n") {
		t.Errorf("missing/incorrect header:\n%q", out)
	}
	// Comma-containing value is quoted.
	if !strings.Contains(out, `"bob,jr"`) {
		t.Errorf("comma value should be quoted:\n%q", out)
	}
	// Newline-containing value is quoted.
	if !strings.Contains(out, "\"line1\nline2\"") {
		t.Errorf("newline value should be quoted:\n%q", out)
	}
	// The literal string "NULL" is preserved (it was not a SQL null).
	if !strings.Contains(out, "3,NULL,") {
		t.Errorf("literal NULL string should be preserved and last field empty:\n%q", out)
	}
}

func TestWriteJSONNullHandling(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, sampleResult()); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(rows) != 3 {
		t.Fatalf("got %d objects, want 3", len(rows))
	}

	// Row 3: note was a real SQL NULL -> JSON null; name is the string "NULL".
	if rows[2]["note"] != nil {
		t.Errorf("row3 note should be JSON null, got %#v", rows[2]["note"])
	}
	if rows[2]["name"] != "NULL" {
		t.Errorf(`row3 name should be the string "NULL", got %#v`, rows[2]["name"])
	}
	// Values are strings.
	if rows[0]["id"] != "1" {
		t.Errorf("row0 id should be string \"1\", got %#v", rows[0]["id"])
	}
}

func TestWriteJSONPreservesColumnOrderAndUniqueKeys(t *testing.T) {
	r := QueryResult{
		Columns: []string{"id", "", "id"}, // empty + duplicate names
		Rows:    [][]string{{"1", "x", "2"}},
		Nulls:   [][]bool{{false, false, false}},
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, r); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	out := buf.String()
	// Expect keys: id, column_2, id_2 (disambiguated).
	for _, want := range []string{`"id"`, `"column_2"`, `"id_2"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected key %s in output:\n%s", want, out)
		}
	}
}

func TestExportDispatch(t *testing.T) {
	if _, err := ParseExportFormat("csv"); err != nil {
		t.Errorf("csv should parse: %v", err)
	}
	if _, err := ParseExportFormat("json"); err != nil {
		t.Errorf("json should parse: %v", err)
	}
	if _, err := ParseExportFormat("xml"); err == nil {
		t.Error("xml should be rejected")
	}

	var buf bytes.Buffer
	if err := Export(&buf, sampleResult(), FormatCSV); err != nil {
		t.Errorf("Export csv: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("csv export produced no output")
	}
}

func TestWriteCSVEmptyResult(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, QueryResult{}); err != nil {
		t.Fatalf("WriteCSV empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty result should produce no CSV, got %q", buf.String())
	}
}
