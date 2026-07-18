package linediff

import (
	"fmt"
	"testing"
)

// churnyLines builds two sides where most lines differ, so every position pays
// a full resync scan — the regime where repeated normalization compounds.
func churnyLines(n int) (StringLines, StringLines) {
	old := make([]string, n)
	neu := make([]string, n)
	for i := range n {
		old[i] = fmt.Sprintf("  Handler_%d(Request, Context)   // payload payload payload", i)
		neu[i] = fmt.Sprintf("  Callback_%d(Req, Context)   // content content content", i)
	}
	return StringLines(old), StringLines(neu)
}

// BenchmarkDiffIgnoreCase exercises the normalizing path. Ignore-case alone
// installs a normalizer, so every comparison lowercases both sides; the resync
// scan then repeats the same line's normalization once per candidate.
func BenchmarkDiffIgnoreCase(b *testing.B) {
	old, neu := churnyLines(400)
	opts := Options{Window: 128, MaxHunks: 1 << 30, IgnoreCase: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DiffWith(old, neu, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDiffPlain is the control: no normalizer, so it shows how much of the
// cost above is normalization rather than the walk itself.
func BenchmarkDiffPlain(b *testing.B) {
	old, neu := churnyLines(400)
	opts := Options{Window: 128, MaxHunks: 1 << 30}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DiffWith(old, neu, opts); err != nil {
			b.Fatal(err)
		}
	}
}
