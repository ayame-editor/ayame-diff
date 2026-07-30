package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEqual(t *testing.T) {
	t.Parallel()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(workingDirectory, "pathutil-test")
	path := filepath.Join(root, "file.txt")
	relative := filepath.Join("pathutil-test", "file.txt")

	for _, test := range []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "absolute_and_relative", a: path, b: relative, want: true},
		{name: "cleaned", a: path, b: filepath.Join(root, "sub", "..", "file.txt"), want: true},
		{name: "different", a: path, b: filepath.Join(root, "other.txt"), want: false},
		{name: "empty_left", a: "", b: path, want: false},
		{name: "empty_right", a: path, b: "", want: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := Equal(test.a, test.b); got != test.want {
				t.Fatalf("Equal(%q, %q) = %v, want %v", test.a, test.b, got, test.want)
			}
		})
	}
}
