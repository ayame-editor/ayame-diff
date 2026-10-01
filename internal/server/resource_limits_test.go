package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// TestParseArchiveLimitsClampsToServerMax covers #170: a client cannot raise the
// archive-expansion guard past the server's absolute maxima (which would re-open
// the zip-bomb DoS #70), while ordinary values pass through untouched.
func TestParseArchiveLimitsClampsToServerMax(t *testing.T) {
	t.Parallel()
	entry, total, err := parseArchiveLimits("1024TiB", "1024TiB")
	if err != nil {
		t.Fatalf("parseArchiveLimits: %v", err)
	}
	if entry != serverMaxArchiveEntryBytes || total != serverMaxArchiveBytes {
		t.Fatalf("limits not clamped: entry=%d total=%d, want %d/%d", entry, total, serverMaxArchiveEntryBytes, serverMaxArchiveBytes)
	}
	// A modest in-range request is preserved exactly.
	entry, total, err = parseArchiveLimits("32MiB", "128MiB")
	if err != nil || entry != 32<<20 || total != 128<<20 {
		t.Fatalf("in-range values altered: entry=%d total=%d err=%v", entry, total, err)
	}
}

// TestClampMemoryBudget covers #170: an over-large memory budget is lowered to
// the server cap (spilling more), while in-range and malformed values are left
// for engine.Validate.
func TestClampMemoryBudget(t *testing.T) {
	t.Parallel()
	if got := clampMemoryBudget("128GiB"); got != serverMaxMemoryText {
		t.Fatalf("clampMemoryBudget(128GiB) = %q, want %q", got, serverMaxMemoryText)
	}
	if got := clampMemoryBudget("512MiB"); got != "512MiB" {
		t.Fatalf("in-range budget altered: %q", got)
	}
	if got := clampMemoryBudget("not-a-size"); got != "not-a-size" {
		t.Fatalf("malformed budget altered: %q", got)
	}
	// The clamp is wired into csvConfig.
	cfg := csvConfig(csvRequest{Old: "a", New: "b", Memory: "999GiB"}, "out.jsonl")
	limit, _ := engine.ParseByteSize(serverMaxMemoryText)
	got, err := engine.ParseByteSize(cfg.MemoryText)
	if err != nil || got > limit {
		t.Fatalf("csvConfig memory not clamped: MemoryText=%q err=%v", cfg.MemoryText, err)
	}
}

// TestCSVMemoryStatusReportsBudgetAndSpill covers #138: the CSV diff response
// carries the resolved budget, the server cap, and — only when the comparison
// actually offloaded — the directory spill files went to, so the GUI can show the
// memory state instead of a bare status string.
func TestCSVMemoryStatusReportsBudgetAndSpill(t *testing.T) {
	t.Parallel()
	summary := engine.Summary{MemoryBudgetBytes: 512 << 20, Spilled: true}
	got := csvMemoryStatusFor(csvRequest{TempDir: "/var/tmp/ayame"}, summary)
	if got.BudgetBytes != 512<<20 || got.Budget != "512.0MiB" {
		t.Fatalf("budget = %d/%q, want %d/%q", got.BudgetBytes, got.Budget, 512<<20, "512.0MiB")
	}
	if got.Cap != serverMaxMemoryText || !got.Spilled || got.SpillDir != "/var/tmp/ayame" {
		t.Fatalf("status = %#v", got)
	}
	// With no explicit temp dir the spill directory falls back to the system temp
	// directory, never an empty string the UI would have to guess about.
	fallback := csvMemoryStatusFor(csvRequest{}, engine.Summary{MemoryBudgetBytes: 1 << 20})
	if fallback.Spilled || fallback.SpillDir != "" {
		t.Fatalf("resident run reported spill = %#v", fallback)
	}
	spilledDefault := csvMemoryStatusFor(csvRequest{}, summary)
	if spilledDefault.SpillDir == "" {
		t.Fatal("spill dir is empty with no --temp-dir")
	}
}

// TestLimitedGatesConcurrentComparisons covers #170: expensive handlers reject
// with 429 once maxConcurrentComparisons are in flight, and recover once a slot
// frees.
func TestLimitedGatesConcurrentComparisons(t *testing.T) {
	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	gated := s.limited(ok)

	// Saturate every comparison slot.
	for i := 0; i < cap(s.compareSem); i++ {
		s.compareSem <- struct{}{}
	}
	rec := httptest.NewRecorder()
	gated(rec, httptest.NewRequest(http.MethodPost, "/api/diff", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("saturated: code=%d, want 429", rec.Code)
	}

	// Free one slot; the next request runs.
	<-s.compareSem
	rec = httptest.NewRecorder()
	gated(rec, httptest.NewRequest(http.MethodPost, "/api/diff", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("after freeing a slot: code=%d, want 200", rec.Code)
	}
}
