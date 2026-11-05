package db

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
)

// ExportFormat enumerates supported serialization formats. These values double
// as the accepted --format strings for the future headless mode.
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
)

// ParseExportFormat validates a format string.
func ParseExportFormat(s string) (ExportFormat, error) {
	switch ExportFormat(s) {
	case FormatCSV:
		return FormatCSV, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported format %q (want csv or json)", s)
	}
}

// Export writes r to w in the given format. It is the single entry point shared
// by the interactive export action and (later) the headless --format path.
func Export(w io.Writer, r QueryResult, format ExportFormat) error {
	switch format {
	case FormatCSV:
		return WriteCSV(w, r)
	case FormatJSON:
		return WriteJSON(w, r)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// WriteCSV writes the result as RFC 4180 CSV: a header row of column names
// followed by one row per record. SQL NULL is written as an empty field (the
// conventional CSV representation); encoding/csv handles quoting of values that
// contain commas, quotes, or newlines.
func WriteCSV(w io.Writer, r QueryResult) error {
	cw := csv.NewWriter(w)

	if len(r.Columns) > 0 {
		if err := cw.Write(r.Columns); err != nil {
			return fmt.Errorf("write csv header: %w", err)
		}
	}

	for i, row := range r.Rows {
		rec := make([]string, len(row))
		for j, cell := range row {
			if r.IsNull(i, j) {
				rec[j] = "" // NULL -> empty field
			} else {
				rec[j] = cell
			}
		}
		if err := cw.Write(rec); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

// WriteJSON writes the result as a JSON array of objects, one per row, keyed by
// column name. SQL NULL is written as JSON null; all other values are written
// as their string representation (values are already stringified in the result
// set). Duplicate or empty column names are made unique so object keys don't
// collide or vanish.
func WriteJSON(w io.Writer, r QueryResult) error {
	keys := uniqueColumnKeys(r.Columns)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	// Build an ordered representation. encoding/json sorts map keys, which
	// would lose column order, so use json.RawMessage assembled manually to
	// preserve order while still getting correct escaping per value.
	out := make([]json.RawMessage, 0, len(r.Rows))
	for i, row := range r.Rows {
		obj, err := rowToJSONObject(keys, row, r, i)
		if err != nil {
			return err
		}
		out = append(out, obj)
	}

	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// rowToJSONObject renders a single row as an ordered JSON object.
func rowToJSONObject(keys []string, row []string, r QueryResult, rowIdx int) (json.RawMessage, error) {
	var buf []byte
	buf = append(buf, '{')
	for j, key := range keys {
		if j > 0 {
			buf = append(buf, ',')
		}
		// Key.
		kb, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("marshal key: %w", err)
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')

		// Value: NULL -> null; otherwise the string value.
		if j < len(row) && !r.IsNull(rowIdx, j) {
			vb, err := json.Marshal(row[j])
			if err != nil {
				return nil, fmt.Errorf("marshal value: %w", err)
			}
			buf = append(buf, vb...)
		} else {
			buf = append(buf, []byte("null")...)
		}
	}
	buf = append(buf, '}')
	return json.RawMessage(buf), nil
}

// uniqueColumnKeys returns object keys for the given columns, disambiguating
// empty and duplicate names (e.g. "", "id", "id" -> "column_1", "id", "id_2").
func uniqueColumnKeys(columns []string) []string {
	seen := make(map[string]int, len(columns))
	keys := make([]string, len(columns))
	for i, c := range columns {
		name := c
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		if n, ok := seen[name]; ok {
			seen[name] = n + 1
			name = fmt.Sprintf("%s_%d", name, n+1)
		} else {
			seen[name] = 1
		}
		keys[i] = name
	}
	return keys
}
