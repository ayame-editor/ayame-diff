// Package e2e holds the end-to-end large-input benchmarks for the CLI engine
// and the checks that keep them honest (#279).
//
// The package has no production code. Its cases live in test files so the
// normal build stays empty: comparing a ten-million-row CSV, a ~1 GiB text
// file, a 100k-file tree, and a single very long line end to end. Every case
// must either finish or fail with a named limit — never hang, OOM, or silently
// truncate.
//
// Sizes default to a small shape CI can run in a few minutes and grow when
// AYAME_BENCH_LARGE=1 is set, matching the sizes named in the issue. Each size
// is also individually overridable (AYAME_BENCH_CSV_ROWS, AYAME_BENCH_TEXT_BYTES,
// AYAME_BENCH_FOLDER_FILES, AYAME_BENCH_LINE_BYTES).
package e2e
