package ui

import (
	"strings"
	"testing"

	"github.com/nofinsachin/tusk/internal/db"
)

func TestFlattenRespectsExpansion(t *testing.T) {
	roots := schemaNodes([]db.Schema{{Name: "public"}, {Name: "app"}})

	// Collapsed: only the two schema rows are visible.
	rows := flatten(roots)
	if len(rows) != 2 {
		t.Fatalf("collapsed: got %d rows, want 2", len(rows))
	}
	if rows[0].depth != 0 || rows[1].depth != 0 {
		t.Errorf("schema rows should be depth 0: %+v", rows)
	}

	// Expand the first schema and give it children.
	roots[0].expanded = true
	roots[0].loaded = true
	roots[0].children = relationNodes([]db.Relation{
		{Schema: "public", Name: "users", Kind: db.RelTable},
		{Schema: "public", Name: "orders", Kind: db.RelTable},
	})

	rows = flatten(roots)
	if len(rows) != 4 { // public, users, orders, app
		t.Fatalf("expanded: got %d rows, want 4", len(rows))
	}
	if rows[1].depth != 1 || rows[1].n.label != "users" {
		t.Errorf("first child row wrong: %+v", rows[1])
	}
	if rows[3].n.label != "app" {
		t.Errorf("trailing schema should still be present: %+v", rows[3])
	}
}

func TestFlattenNested(t *testing.T) {
	roots := schemaNodes([]db.Schema{{Name: "public"}})
	roots[0].expanded = true
	roots[0].loaded = true
	rel := relationNodes([]db.Relation{{Schema: "public", Name: "users", Kind: db.RelTable}})
	roots[0].children = rel

	rel[0].expanded = true
	rel[0].loaded = true
	rel[0].children = columnNodes([]db.Column{
		{Name: "id", DataType: "integer", Nullable: false},
		{Name: "email", DataType: "text", Nullable: true},
	})

	rows := flatten(roots)
	// public(0) users(1) id(2) email(2)
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	if rows[2].depth != 2 {
		t.Errorf("column depth = %d, want 2", rows[2].depth)
	}
}

func TestColumnLabelShowsNullability(t *testing.T) {
	cols := columnNodes([]db.Column{
		{Name: "id", DataType: "integer", Nullable: false},
		{Name: "bio", DataType: "text", Nullable: true},
	})
	if got := cols[0].label; got == "" || !strings.Contains(got, "NOT NULL") {
		t.Errorf("non-nullable label = %q, want it to contain NOT NULL", got)
	}
	if got := cols[1].label; !strings.Contains(got, "NULL") || strings.Contains(got, "NOT NULL") {
		t.Errorf("nullable label = %q, want plain NULL", got)
	}
}

func TestIconAndChildren(t *testing.T) {
	schema := &node{kind: nodeSchema}
	if !schema.hasChildren() {
		t.Error("schema should have children")
	}
	if schema.icon() != "▸" {
		t.Errorf("collapsed icon = %q, want ▸", schema.icon())
	}
	schema.expanded = true
	if schema.icon() != "▾" {
		t.Errorf("expanded icon = %q, want ▾", schema.icon())
	}
	schema.expanded = false
	schema.loading = true
	if schema.icon() != "…" {
		t.Errorf("loading icon = %q, want …", schema.icon())
	}

	col := &node{kind: nodeColumn}
	if col.hasChildren() {
		t.Error("column should be a leaf")
	}
	if col.icon() != " " {
		t.Errorf("leaf icon = %q, want space", col.icon())
	}
}
