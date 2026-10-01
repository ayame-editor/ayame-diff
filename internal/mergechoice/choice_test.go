package mergechoice

import (
	"reflect"
	"testing"
)

func TestParseOrdersCanonically(t *testing.T) {
	t.Parallel()
	// However the client ordered the toggles, the engine concatenates left
	// before right; three-way additionally puts base first.
	if got := Parse("right,left", "left", "right"); !reflect.DeepEqual(got, []string{"left", "right"}) {
		t.Fatalf("two-way parse = %v", got)
	}
	if got := Parse("right,base,left", "base", "left", "right"); !reflect.DeepEqual(got, []string{"base", "left", "right"}) {
		t.Fatalf("three-way parse = %v", got)
	}
	if got := Parse("", "left", "right"); got != nil {
		t.Fatalf("empty parse = %v, want nil", got)
	}
}

func TestValidateRejectsMalformedChoices(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value string
		ok    bool
	}{
		{"left", true},
		{"left,right", true},
		{" right , left ", true},
		{"", false},
		{"   ", false},
		{"left,left", false},
		{"both", false},
		{"left,base", false},
	}
	for _, tc := range cases {
		err := Validate(tc.value, "left", "right")
		if (err == nil) != tc.ok {
			t.Errorf("Validate(%q) err=%v, want ok=%v", tc.value, err, tc.ok)
		}
	}
}

func TestHasDetectsASide(t *testing.T) {
	t.Parallel()
	if !Has("left,right", "right") || Has("left", "base") {
		t.Fatal("Has misreported membership")
	}
}
