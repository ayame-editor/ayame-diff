<!-- i18n: language-switcher -->
[English](0005-no-embedded-sql-engine.md) | [日本語](0005-no-embedded-sql-engine.ja.md)

# ADR 0005: Pre-comparison shaping without an always-on SQL engine

- Status: Proposed (2026-09-27)
- Related Issue: ayame-editor/ayame-diff#126
- Related Work: #116 (order-independent equality), #119 (column mapping),
  #120 (statistics), #121 (alignment suggestions), #128 (asynchronous
  rendering), #19 / #60 (packaging)

## Context

A comparison is often only meaningful after the data has been shaped: drop
columns, reorder rows, filter by a predicate, or join two extracts. Asking for
SQL (`SELECT … ORDER BY … WHERE …`, optionally a join) before the comparison is
a natural way to express that, and it would share machinery with the
order-independent equality (#116), column mapping (#119), statistics (#120), and
alignment-suggestion (#121) work.

The project's priorities, in order, are:

1. **Runtime comfort.** Large inputs must not hang or OOM the machine.
2. **Redundancy, availability, never crashing.** The tool is used for data
   verification, where a crash loses work.
3. **Lightness.** The default binary must stay small.
4. **Builds finish in a realistic time.**

SQL is a "nice to have" and is only acceptable if it breaks none of the above.
There is also a standing constraint that the Go module has no dependencies
outside the standard library plus the single `x/text` exception recorded in
[ADR 0003](0003-encoding-dependency.md).

## Options

### A. Always embed DuckDB (for example `go-duckdb`, cgo)

Rejected. cgo makes builds much slower, complicates cross-compilation, and adds
a C surface that can crash. It violates priorities 2, 3, and 4 directly, which
is the concern raised in the issue.

### B. Optional build feature or a separate sidecar binary

A `ayame-diff-sql` binary (or a dynamically loaded sidecar) keeps the default
binary pure Go, light, and fast to build, and isolates the crash surface from
the main process. The cost is a doubled distribution and update path, which is
why it has to be designed together with packaging (#19 / #60) and the
inter-process handoff.

### C. Use an external `duckdb` / `sqlite3` on PATH when present

No embedding, so priorities 2–4 are unaffected, and only users who already have
the tool get the feature. The risk is shell execution, which needs the same
care as the `--pre` prediffer shell and the unauthenticated `serve` concern. A
progressive enhancement along these lines is acceptable.

### D. A dependency-free select / sort / filter DSL in the existing pipeline

Implement `select` / `order by` / `where` / `limit` equivalents ourselves. It is
not full SQL, but it covers the bulk of pre-comparison shaping with no new
dependency, shares code with #116 and #120, and can spill the way the engine
already does. The sister project ayame-editor's Rust core already carries
sort/group/distinct/top/aggregate/spill and is a design reference.

## Decision

- **Default: option D.** A dependency-free select/sort/filter expression that
  runs inside the existing pipeline and honours the storage-offload (spill)
  model, so large inputs stay responsive.
- **Full SQL is opt-in only, through B or C** (a separate extension binary, or
  an external CLI discovered on PATH). It is never required for the default
  install and never linked into the default binary.
- **Option A (always-on embedded DuckDB) is rejected.**
- Whichever path is taken, pre-comparison shaping must not block the UI, must
  be cancellable, and must degrade explicitly rather than truncate silently.

## Consequences

- The default binary keeps its current size, build time, and crash surface.
- The SQL-ish surface starts smaller than full SQL; queries that need joins or
  window functions are out of scope for D and belong to the opt-in path.
- #116, #119, #120, and #121 should build on D's shaping layer rather than
  introducing parallel filtering and ordering code.
- The opt-in path needs a distribution and update design before it ships.

## Rejected alternatives

- **Always-on embedded DuckDB (A):** violates the build-time, lightness, and
  crash-surface priorities.
- **Add SQL before designing the shaping layer:** would duplicate work that
  #116 and #120 need and lock the data model to a query engine.
- **No shaping at all:** leaves the "filter and reorder before comparing" need
  unmet, which is the point of the issue.
