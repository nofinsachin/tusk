package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Schema is a namespace in the database.
type Schema struct {
	Name string
}

// RelKind distinguishes tables from views.
type RelKind string

const (
	RelTable RelKind = "table"
	RelView  RelKind = "view"
	RelMatView RelKind = "materialized view"
)

// Relation is a table, view, or materialized view within a schema.
type Relation struct {
	Schema string
	Name   string
	Kind   RelKind
}

// Column describes a column of a relation.
type Column struct {
	Name     string
	DataType string
	Nullable bool
}

// ListSchemas returns the non-system schemas visible to the connection,
// ordered by name. If includeSystem is true, pg_catalog/information_schema and
// pg_toast schemas are included as well.
func ListSchemas(ctx context.Context, conn *pgx.Conn, includeSystem bool) ([]Schema, error) {
	const q = `
		SELECT nspname
		FROM pg_catalog.pg_namespace
		WHERE ($1 OR (nspname NOT IN ('pg_catalog', 'information_schema')
		              AND nspname NOT LIKE 'pg_toast%'
		              AND nspname NOT LIKE 'pg_temp%'))
		ORDER BY nspname`
	rows, err := conn.Query(ctx, q, includeSystem)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var out []Schema
	for rows.Next() {
		var s Schema
		if err := rows.Scan(&s.Name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	return out, nil
}

// ListRelations returns tables, views, and materialized views in the given
// schema, ordered by name.
func ListRelations(ctx context.Context, conn *pgx.Conn, schema string) ([]Relation, error) {
	// relkind: r=table, p=partitioned table, v=view, m=materialized view,
	// f=foreign table.
	const q = `
		SELECT c.relname,
		       CASE c.relkind
		         WHEN 'r' THEN 'table'
		         WHEN 'p' THEN 'table'
		         WHEN 'f' THEN 'table'
		         WHEN 'v' THEN 'view'
		         WHEN 'm' THEN 'materialized view'
		         ELSE 'table'
		       END AS kind
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
		ORDER BY c.relname`
	rows, err := conn.Query(ctx, q, schema)
	if err != nil {
		return nil, fmt.Errorf("list relations in %s: %w", schema, err)
	}
	defer rows.Close()

	var out []Relation
	for rows.Next() {
		r := Relation{Schema: schema}
		var kind string
		if err := rows.Scan(&r.Name, &kind); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		r.Kind = RelKind(kind)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list relations in %s: %w", schema, err)
	}
	return out, nil
}

// ListColumns returns the columns of a relation in ordinal order.
func ListColumns(ctx context.Context, conn *pgx.Conn, schema, relation string) ([]Column, error) {
	const q = `
		SELECT a.attname,
		       pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
		       NOT a.attnotnull AS nullable
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum`
	rows, err := conn.Query(ctx, q, schema, relation)
	if err != nil {
		return nil, fmt.Errorf("list columns of %s.%s: %w", schema, relation, err)
	}
	defer rows.Close()

	var out []Column
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.Name, &col.DataType, &col.Nullable); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		out = append(out, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list columns of %s.%s: %w", schema, relation, err)
	}
	return out, nil
}
