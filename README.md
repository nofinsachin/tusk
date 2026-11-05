# tusk

A fast, clean, keyboard-driven Postgres manager for the terminal.

`tusk` is a TUI (terminal user interface) built because the existing options are
either slow and cluttered (pgAdmin) or not free (PEM). It focuses on doing the
90% of day-to-day work well: connect, browse schemas and tables, run queries,
and view results — all without leaving the keyboard.

## Goals

- **Fast**: instant startup, snappy navigation, no web stack.
- **Clean**: uncluttered, focused layout.
- **Keyboard-driven**: everything reachable without a mouse.

## Vision

Beyond a laptop TUI, tusk aims to be a **cloud-native Postgres client**: a
single static binary small enough to run as a container next to a Postgres
cluster, with a dual interactive/headless mode and a security-first posture. It
targets orchestrator-agnostic primitives (env vars, mounted secrets, DNS, TTY
detection, structured output) rather than any specific platform. See
[DESIGN.md](DESIGN.md) for the full vision and principles.

## Status

Early development. Current milestone: MVP.

### MVP scope

- [ ] Save & manage connections
- [ ] Browse schemas → tables → columns
- [ ] Query editor
- [ ] Results grid
- [ ] Export results (CSV/JSON)

## Stack

- Go
- [bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) — styling
- [pgx](https://github.com/jackc/pgx) — Postgres driver

## Build & run

```sh
go run ./cmd/tusk
```

## License

TBD
