# tusk — Design & Vision

This document is the north star for tusk's direction. Features should be
evaluated against the principles here.

## Vision

tusk is a **cloud-native Postgres client**: a single static binary, small
enough to ship as a container and run *next to* a Postgres cluster — as a
debug/sidecar workload — instead of port-forwarding and running `psql` in
"obscure ways" from a laptop.

It is first a fast, keyboard-driven TUI, and second a scriptable, headless
tool. The same binary serves both.

## Non-negotiables

1. **Security & compliance first.** This is the primary lens for every
   decision. A powerful DB client that runs inside a cluster is a high-value
   target and must behave accordingly.
2. **Never store passwords.** Credentials are resolved at connect time from the
   environment, mounted secret files, or an interactive prompt — never written
   to tusk's own config.
3. **Dual mode.** Interactive TUI when attached to a TTY; headless and
   scriptable (flags, SQL in, structured output, sane exit codes) when not.

## Orchestrator-agnostic, not Kubernetes-specific

We deliberately do **not** couple to Kubernetes (no CRDs, no pod-watching, no
operator integration). Kubernetes is today's orchestrator; Nomad, ECS, Cloud
Run, or a future system may not be. Instead we target the primitives that
outlive any particular orchestrator — essentially the 12-factor contract:

- Configuration and secrets from **environment variables** and **mounted
  files**.
- Reaching the database over the **network by DNS name** (e.g. an in-cluster
  service), honoring standard `PG*` / `DATABASE_URL` conventions.
- Running **interactively (TTY) or headless (no TTY)**.
- Emitting **structured output** and meaningful **exit codes**.

If tusk honors these, it runs unchanged as a Kubernetes pod, a Nomad task, an
ECS task, `docker run`, or a bare binary.

## Security posture (direction)

These are goals that shape features; not all are implemented yet.

- **Least privilege**: respect the connecting role's privileges; never assume
  superuser. Make it obvious what role/permissions a session has.
- **Read-only mode**: an opt-in mode (and/or read-only transactions) so a
  session cannot mutate data — important for compliance-sensitive access.
- **Credential hygiene**: prefer projected secret volumes, cloud secret
  managers, or injected env; never persist secrets; avoid echoing secret values
  in the UI or logs.
- **Auditability**: a per-session, structured activity log suitable for
  shipping to a central system (who/when/what SQL), so in-cluster access is
  accountable.
- **Supply chain**: distroless/scratch image, non-root user, pinned
  dependencies, reproducible builds, SBOM.

## Connection input priority (direction)

When resolving how to connect, prefer explicit/ephemeral sources over stored
ones:

1. Explicit flags (`--dsn`, headless invocations).
2. Environment: `DATABASE_URL`, then standard `PG*` vars.
3. Mounted config / saved connections YAML (laptop convenience).

Passwords within any of these are resolved via `PGPASSWORD` / `~/.pgpass` /
projected secret / prompt — never stored by tusk.

## What we deliberately defer

- Orchestrator-specific logic of any kind.
- A web UI / server (the TUI over `exec`/`attach` is the interactive path).
- Multi-database federation, schema diff, and other "big tool" features until
  the core + cloud-native packaging are solid.

## Current status vs. vision

Implemented: connection manager, schema browser (+ fuzzy search), query editor
with a rich results grid, auto-refreshing metrics. These are the interactive
core.

Next toward the vision:

1. **Export (CSV/JSON)** — also the first slice of headless/structured output.
2. **`DATABASE_URL` / `PG*` env as a first-class connection source.**
3. **Headless mode** — `tusk --dsn ... -c "SQL" --format csv|json|table`,
   auto-selected when there is no TTY.
4. **Container image** — distroless, non-root, static binary.
