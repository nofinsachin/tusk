# tusk — Feature Backlog

A running list of features. Keep adding as ideas come up. Status key:
`[ ]` todo · `[~]` in progress · `[x]` done.

## MVP

- [~] **Connection manager** — list, add, edit, delete saved connections; test & connect.
  - Store metadata in `~/.config/tusk/connections.yml`.
  - Never store passwords; resolve via `PGPASSWORD`, `~/.pgpass`, or an
    in-TUI masked prompt (shown only when no password is otherwise available).
- [x] **Schema browser** — collapsible tree of schemas → tables/views → columns.
  - Lazy-loads children; columns show type and nullability.
  - Keyboard nav; `r` refresh, `b` back to connections.
  - [x] Fuzzy text search (`/`) over loaded schema & table/view names
    (no columns); auto-shows schemas with matching children; `esc` clears.
- [x] **Query editor** — multi-line SQL input (textarea) run with `Ctrl+R`
  (terminals can't reliably detect Ctrl+Enter). Opens from the browser with
  `e`; `tab` switches editor/results panes; shows a basic scrollable results
  table, command tag for non-SELECTs, timing, and errors. A richer grid is the
  next item.
  - [x] Richer results grid: auto column widths (capped at 40 with ellipsis),
    row-number gutter, header divider, column separators, a highlighted row
    cursor, distinct NULL styling, virtualized rows & columns.
  - [x] Styled, width-safe footer bar: connection name + row position on the
    left, emphasized key hints on the right, truncated to width (no overflow).
- [x] **Results grid** — scrollable, virtualized table of query results
  (implemented as part of the query editor; see above).
- [x] **Metrics** — auto-refreshing (2s) server metrics view, reachable with
  `m` from the browser.
  - Storage: database size + top tables by total size (with size bars).
  - Activity / Load: connection counts, cache hit ratio, and transactions/sec
    (TPS sampled between refreshes). Note: true host CPU% is not available via
    SQL, so activity/load proxies are shown in its place.
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
