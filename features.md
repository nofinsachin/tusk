# tusk — Feature Backlog

A running list of features. Keep adding as ideas come up. Status key:
`[ ]` todo · `[~]` in progress · `[x]` done.

## MVP

- [~] **Connection manager** — list, add, edit, delete saved connections; test & connect.
  - Store metadata in `~/.config/tusk/connections.yml`.
  - Never store passwords; resolve via `PGPASSWORD`, `~/.pgpass`, or prompt.
- [ ] **Schema browser** — tree of databases → schemas → tables/views → columns.
- [ ] **Query editor** — multi-line SQL input with run (Ctrl+Enter).
- [ ] **Results grid** — scrollable, virtualized table of query results.
- [ ] **Export results** — CSV / JSON.

## Nice to have (post-MVP)

- [ ] Inline row editing with generated `UPDATE`/`DELETE` (with confirmation).
- [ ] `EXPLAIN` / `EXPLAIN ANALYZE` viewer.
- [ ] Query history & saved queries.
- [ ] Autocomplete (tables, columns, keywords).
- [ ] Transaction controls (BEGIN/COMMIT/ROLLBACK) with safety guardrails.
- [ ] Multiple result tabs / split panes.
- [ ] Theming / color schemes.
- [ ] Rich type rendering (JSONB, arrays, ranges, timestamps, NULL).
- [ ] Copy cell / row / result to clipboard.
- [ ] Server & table stats (sizes, row counts, index usage).

## Ideas / maybe

- [ ] Schema diff between two connections.
- [ ] Favorite / pinned tables.
- [ ] Vim-style keybindings mode.
