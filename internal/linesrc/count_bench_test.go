package linesrc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/encoding"
)

func benchFile(b *testing.B, lines int) string {
	b.Helper()
	path := filepath.Join(b.TempDir(), "lines.txt")
	var builder strings.Builder
	for i := range lines {
		fmt.Fprintf(&builder, "func handler_%d(request, context) { return payload(%d) }\n", i, i)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		b.Fatal(err)
	}
	return path
}

// BenchmarkCountLines measures the pre-count pass alone. It reports allocations
// because the point of the change is that counting no longer builds a string
// per line only to discard it (#156).
func BenchmarkCountLines(b *testing.B) {
	path := benchFile(b, 200000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := countLines(path, false, encoding.UTF8, DefaultMaxLineBytes)
		if err != nil {
			b.Fatal(err)
		}
		if n != 200000 {
			b.Fatalf("count=%d", n)
		}
	}
}
