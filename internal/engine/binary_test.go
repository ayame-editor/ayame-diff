package engine

import (
	"reflect"
	"testing"
)

func TestXXHash64Vectors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  uint64
	}{
		{"", 0xef46db3751d8e999},
		{"a", 0xd24ec4f1a98c6e5b},
		{"abc", 0x44bc2cf5ad770999},
	}
	for _, tc := range cases {
		if got := xxhash64([]byte(tc.input)); got != tc.want {
			t.Fatalf("xxhash64(%q) = 0x%x, want 0x%x", tc.input, got, tc.want)
		}
	}
}

func TestEncodeDecodeRow(t *testing.T) {
	t.Parallel()
	input := []string{"plain", "comma,value", "tab\tvalue", "line1\nline2", "日本語", ""}
	mapping := identityMap(len(input))
	key, row, err := encodeStringFields(input, mapping, []int{0, 4}, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) == 0 {
		t.Fatal("encoded key is empty")
	}
	got, err := decodeRow(row, len(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("decoded row = %#v, want %#v", got, input)
	}
}

func TestEncodeFullRowKeyStoresOneCopy(t *testing.T) {
	t.Parallel()
	input := []string{"1", "Alice", "2026-07-10"}
	mapping := identityMap(len(input))
	key, row, err := encodeStringFields(input, mapping, []int{0, 1, 2}, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != 0 {
		t.Fatalf("full-row key should not duplicate row bytes: len(row)=%d", len(row))
	}
	decoded, err := decodeRow(key, len(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, input) {
		t.Fatalf("decoded key = %#v, want %#v", decoded, input)
	}
}
