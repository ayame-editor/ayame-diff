package linediff

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
)

func mustFilters(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		out[i] = regexp.MustCompile(pattern)
	}
	return out
}

// TestNormalizationCacheDoesNotChangeResults is the #156 safety net. The resync
// scan memoizes normalized lines in a ring sized to the window; a stale or
// mismatched slot would silently change which lines compare equal, so the
// results must match what an uncached comparison produces.
//
// The ring is indexed modulo its size, so a window far smaller than the file
// forces wraparound and eviction — exactly where a missing index check would
// show up.
func TestNormalizationCacheDoesNotChangeResults(t *testing.T) {
	t.Parallel()
	old := make([]string, 300)
	neu := make([]string, 300)
	for i := range old {
		old[i] = fmt.Sprintf("  Value_%d\tRequest  ", i%37)
		neu[i] = fmt.Sprintf("value_%d  request", i%41)
	}

	for _, opts := range []Options{
		{IgnoreCase: true},
		{Whitespace: WSAll},
		{Whitespace: WSChange},
		{IgnoreCase: true, Whitespace: WSAll},
		{LineFilters: mustFilters(`\d+`)},
		{IgnoreCase: true, LineFilters: mustFilters(`Value_\d+`)},
	} {
		for _, window := range []uint64{1, 2, 7, 128, 4096, 100000} {
			opts := opts
			opts.Window, opts.MaxHunks = window, 1<<30
			name := fmt.Sprintf("window=%d/case=%v/ws=%v/filters=%d", window, opts.IgnoreCase, opts.Whitespace, len(opts.LineFilters))
			t.Run(name, func(t *testing.T) {
				got, err := DiffWith(StringLines(old), StringLines(neu), opts)
				if err != nil {
					t.Fatal(err)
				}
				want := uncachedDiff(t, old, neu, opts)
				if !reflect.DeepEqual(got.Hunks, want.Hunks) {
					t.Fatalf("cached diff differs from uncached: %d vs %d hunks", len(got.Hunks), len(want.Hunks))
				}
			})
		}
	}
}

// uncachedDiff reproduces the comparison with the ring disabled, so the test
// compares against normalization performed fresh on every access.
func uncachedDiff(t *testing.T, old, neu []string, opts Options) Result {
	t.Helper()
	// Pre-normalizing both sides and comparing without any ignore option is
	// equivalent to normalizing on every access, and shares no code with the
	// ring — so a bug in the ring cannot hide here.
	norm := normalizer(opts)
	if norm == nil {
		t.Fatal("test options must install a normalizer")
	}
	normOld := make([]string, len(old))
	normNew := make([]string, len(neu))
	for i, line := range old {
		normOld[i] = norm(line)
	}
	for i, line := range neu {
		normNew[i] = norm(line)
	}
	plain := Options{Window: opts.Window, MaxHunks: opts.MaxHunks}
	result, err := DiffWith(StringLines(normOld), StringLines(normNew), plain)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestNormalizationRingEvictsByIndex pins the ring's own contract: a slot is
// only trusted when its recorded index matches, so wraparound misses instead of
// returning another line's text.
func TestNormalizationRingEvictsByIndex(t *testing.T) {
	t.Parallel()
	ring := newNormRing(3) // size 4
	ring.store(1, "one")
	if got, ok := ring.lookup(1); !ok || got != "one" {
		t.Fatalf("lookup(1) = %q, %v", got, ok)
	}
	// Index 5 shares slot 1 with index 1; storing it must evict, not alias.
	ring.store(5, "five")
	if _, ok := ring.lookup(1); ok {
		t.Error("evicted index 1 still reported a hit")
	}
	if got, ok := ring.lookup(5); !ok || got != "five" {
		t.Fatalf("lookup(5) = %q, %v", got, ok)
	}
	// An index never stored must miss even though its slot is filled.
	if _, ok := ring.lookup(9); ok {
		t.Error("unstored index 9 reported a hit")
	}
}

// TestNormalizationRingIsBounded keeps an enormous client-supplied window from
// allocating an enormous cache.
func TestNormalizationRingIsBounded(t *testing.T) {
	t.Parallel()
	ring := newNormRing(^uint64(0) >> 1)
	if len(ring.texts) > maxNormCacheEntries {
		t.Fatalf("ring holds %d entries; the cap is %d", len(ring.texts), maxNormCacheEntries)
	}
}
