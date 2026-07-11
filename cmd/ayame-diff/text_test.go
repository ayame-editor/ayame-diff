package main

import (
	"testing"

	"github.com/hjosugi/ayame-diff/internal/diffout"
)

func TestSubcommandDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"text", "a", "b"}, "text"},
		{[]string{"sorted", "a", "b"}, "sorted"},
		{[]string{"dir", "a", "b"}, "dir"},
		{[]string{"serve"}, "serve"},
		{[]string{"gui"}, "gui"},
		{[]string{"update", "--check"}, "update"},
		{[]string{"remove"}, "remove"},
		{[]string{"csv", "--left", "a"}, "csv"},
		{[]string{"--left", "a.csv"}, ""}, // bare flags => CSV back-compat
		{[]string{"--version"}, ""},       // handled before dispatch
		{[]string{"unknown", "x"}, ""},    // not a subcommand => CSV
	}
	for _, c := range cases {
		if got := subcommand(c.args); got != c.want {
			t.Fatalf("subcommand(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestDiffFlagsFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d    diffFlags
		want diffout.Format
	}{
		{diffFlags{}, diffout.Unified},
		{diffFlags{side: true}, diffout.SideBySide},
		{diffFlags{summary: true}, diffout.Summary},
		{diffFlags{json: true}, diffout.JSON},
		{diffFlags{json: true, side: true}, diffout.JSON}, // json wins
		{diffFlags{summary: true, side: true}, diffout.Summary},
	}
	for _, c := range cases {
		if got := c.d.format(); got != c.want {
			t.Fatalf("format(%+v) = %v, want %v", c.d, got, c.want)
		}
	}
}
