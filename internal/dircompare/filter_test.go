package dircompare

import (
	"testing"
	"time"
)

func TestFilterExpression(t *testing.T) {
	t.Parallel()
	filter, err := ParseFilter(`size > 1MiB and (name =~ '\.log$' or ext == '.json') and not path =~ '^vendor/'`)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		size int64
		want bool
	}{
		{"logs/app.log", 2 << 20, true}, {"data/app.json", 2 << 20, true},
		{"vendor/app.log", 2 << 20, false}, {"logs/app.log", 10, false}, {"notes.txt", 2 << 20, false},
	} {
		if got := filter.Match(test.path, test.size, time.Time{}); got != test.want {
			t.Errorf("Match(%q, %d) = %v, want %v", test.path, test.size, got, test.want)
		}
	}
}

func TestFilterMTimeAndErrors(t *testing.T) {
	t.Parallel()
	filter, err := ParseFilter(`mtime >= 2026-01-01 and name != skip.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if !filter.Match("keep.txt", 1, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) || filter.Match("skip.txt", 1, time.Now()) {
		t.Fatal("mtime/name filter mismatch")
	}
	for _, expression := range []string{"size >", "unknown == x", "name > x", "name =~ '['", "(size > 1KB"} {
		if _, err := ParseFilter(expression); err == nil {
			t.Errorf("ParseFilter(%q) succeeded", expression)
		}
	}
}
