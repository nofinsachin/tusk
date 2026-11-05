package ui

import (
	"fmt"

	"github.com/sahilm/fuzzy"

	"github.com/nofinsachin/tusk/internal/db"
)

// nodeKind identifies what a tree node represents.
type nodeKind int

const (
	nodeSchema nodeKind = iota
	nodeRelation
	nodeColumn
)

// node is a single entry in the schema tree. The tree is intentionally simple:
// schemas contain relations, relations contain columns. Children are loaded
// lazily the first time a node is expanded.
type node struct {
	kind     nodeKind
	label    string // display label (short name)
	schema   string // owning schema (set for relations and columns)
	relation string // owning relation (set for columns)
	relKind  db.RelKind

	expanded bool
	loaded   bool // children have been fetched
	loading  bool // a fetch is in flight
	err      string

	children []*node
}

// hasChildren reports whether this node can be expanded. Columns are leaves.
func (n *node) hasChildren() bool {
	return n.kind != nodeColumn
}

// visibleRow is a flattened, renderable line produced from the tree.
type visibleRow struct {
	n     *node
	depth int
}

// flatten walks the tree depth-first and returns the currently visible rows,
// honoring each node's expanded state. It is pure: it does not mutate the tree.
func flatten(roots []*node) []visibleRow {
	var out []visibleRow
	var walk func(nodes []*node, depth int)
	walk = func(nodes []*node, depth int) {
		for _, n := range nodes {
			out = append(out, visibleRow{n: n, depth: depth})
			if n.expanded {
				walk(n.children, depth+1)
			}
		}
	}
	walk(roots, 0)
	return out
}

// fuzzyMatch reports whether pattern fuzzily matches s (case-insensitive).
// An empty pattern matches everything.
func fuzzyMatch(pattern, s string) bool {
	if pattern == "" {
		return true
	}
	return len(fuzzy.Find(pattern, []string{s})) > 0
}

// filterRows returns the visible rows when a fuzzy filter is active.
//
// Scope (by design): only schemas and relations are considered; columns are
// never matched and are omitted from filtered output. A schema is shown if its
// own name matches, or if any of its (already-loaded) relation children match.
// When a schema is shown only because of matching children, those matching
// children are shown beneath it (the schema is treated as expanded for display
// purposes, without mutating the node's real expanded state).
//
// It is pure: it does not mutate the tree. An empty pattern yields the normal
// unfiltered, expansion-respecting flatten.
func filterRows(roots []*node, pattern string) []visibleRow {
	if pattern == "" {
		return flatten(roots)
	}

	var out []visibleRow
	for _, schema := range roots {
		if schema.kind != nodeSchema {
			continue
		}
		schemaMatches := fuzzyMatch(pattern, schema.label)

		// Collect matching relation children.
		var matchingRels []*node
		for _, child := range schema.children {
			if child.kind != nodeRelation {
				continue
			}
			if fuzzyMatch(pattern, child.label) {
				matchingRels = append(matchingRels, child)
			}
		}

		if !schemaMatches && len(matchingRels) == 0 {
			continue
		}

		out = append(out, visibleRow{n: schema, depth: 0})

		// Show children: if the schema itself matched, show all its relation
		// children; otherwise show only the ones that matched.
		if schemaMatches {
			for _, child := range schema.children {
				if child.kind == nodeRelation {
					out = append(out, visibleRow{n: child, depth: 1})
				}
			}
		} else {
			for _, child := range matchingRels {
				out = append(out, visibleRow{n: child, depth: 1})
			}
		}
	}
	return out
}

// icon returns the expand/collapse/leaf indicator for a node.
func (n *node) icon() string {
	if !n.hasChildren() {
		return " "
	}
	if n.loading {
		return "…"
	}
	if n.expanded {
		return "▾"
	}
	return "▸"
}

// typeLabel returns a short tag for a node's kind, used in rendering.
func (n *node) typeLabel() string {
	switch n.kind {
	case nodeSchema:
		return "schema"
	case nodeRelation:
		return string(n.relKind)
	default:
		return ""
	}
}

// schemaNodes builds root schema nodes from a schema list.
func schemaNodes(schemas []db.Schema) []*node {
	out := make([]*node, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, &node{
			kind:   nodeSchema,
			label:  s.Name,
			schema: s.Name,
		})
	}
	return out
}

// relationNodes builds relation child nodes for a schema.
func relationNodes(rels []db.Relation) []*node {
	out := make([]*node, 0, len(rels))
	for _, r := range rels {
		out = append(out, &node{
			kind:     nodeRelation,
			label:    r.Name,
			schema:   r.Schema,
			relation: r.Name,
			relKind:  r.Kind,
		})
	}
	return out
}

// columnNodes builds column leaf nodes for a relation.
func columnNodes(cols []db.Column) []*node {
	out := make([]*node, 0, len(cols))
	for _, c := range cols {
		nullable := "NOT NULL"
		if c.Nullable {
			nullable = "NULL"
		}
		out = append(out, &node{
			kind:  nodeColumn,
			label: fmt.Sprintf("%s  %s  %s", c.Name, c.DataType, nullable),
		})
	}
	return out
}
