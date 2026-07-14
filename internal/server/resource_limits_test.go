package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/engine"
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
