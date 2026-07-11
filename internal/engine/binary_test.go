package engine

import (
	"bytes"
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

func TestStringAndByteFieldEncodingAreIdentical(t *testing.T) {
	t.Parallel()
	stringsInput := []string{"1", "日本語", "comma,value", "", "line\nbreak"}
	bytesInput := make([][]byte, len(stringsInput))
	for i := range stringsInput {
		bytesInput[i] = []byte(stringsInput[i])
	}
	mapping := []int{4, 2, 0, 3, 1}
	keyIndexes := []int{0, 2}
	stringKey, stringRow, err := encodeStringFields(stringsInput, mapping, keyIndexes, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	byteKey, byteRow, err := encodeByteFields(bytesInput, mapping, keyIndexes, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stringKey, byteKey) || !bytes.Equal(stringRow, byteRow) {
		t.Fatalf("string and byte encoders differ:\nkey %x / %x\nrow %x / %x", stringKey, byteKey, stringRow, byteRow)
	}
}

func TestXXHash64OfficialLongVectors(t *testing.T) {
	t.Parallel()
	// Official zero-seed vectors and input generator from xxHash's primary
	// sanity suite: tests/sanity_test_vectors.h and its generator.
	// https://github.com/Cyan4973/xxHash/tree/dev/tests
	buffer := make([]byte, 65)
	byteGen := uint64(2654435761)
	const prime64 = uint64(11400714785074694797)
	for i := range buffer {
		buffer[i] = byte(byteGen >> 56)
		byteGen *= prime64
	}
	for _, tc := range []struct {
		length int
		want   uint64
	}{
		{length: 32, want: 0x18B216492BB44B70},
		{length: 64, want: 0xEF558F8ACAC2B5CD},
		{length: 65, want: 0xDE0F20DC2631AF7A},
	} {
		if got := xxhash64(buffer[:tc.length]); got != tc.want {
			t.Fatalf("xxhash64(official[%d]) = 0x%x, want 0x%x", tc.length, got, tc.want)
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
