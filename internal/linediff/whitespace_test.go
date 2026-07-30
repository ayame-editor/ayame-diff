package linediff

import "testing"

func TestParseWhitespace(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		value string
		want  Whitespace
	}{
		{value: "", want: WSKeep},
		{value: "none", want: WSKeep},
		{value: "unknown", want: WSKeep},
		{value: "change", want: WSChange},
		{value: "all", want: WSAll},
	} {
		test := test
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			if got := ParseWhitespace(test.value); got != test.want {
				t.Fatalf("ParseWhitespace(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
