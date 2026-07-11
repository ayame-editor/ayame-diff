package engine

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBytesEdgeCases(t *testing.T) {
	t.Parallel()
	valid := map[string]int64{
		"1e3MB":   1_000_000_000,
		"1.5 KiB": 1536,
		"42":      42,
	}
	for input, want := range valid {
		got, err := parseBytes(input)
		if err != nil || got != want {
			t.Fatalf("parseBytes(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "-1MiB", "wat", "1e100TiB"} {
		if _, err := parseBytes(input); err == nil {
			t.Fatalf("parseBytes(%q) succeeded", input)
		}
	}
}

func TestParseDelimiterCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  byte
		ok    bool
	}{
		{input: "tab", want: '\t', ok: true},
		{input: `\t`, want: '\t', ok: true},
		{input: "comma", want: ',', ok: true},
		{input: "pipe", want: '|', ok: true},
		{input: ";", want: ';', ok: true},
		{input: "", ok: false},
		{input: "日本", ok: false},
		{input: "\n", ok: false},
		{input: `"`, ok: false},
	}
	for _, tt := range tests {
		got, err := parseDelimiter(tt.input)
		if tt.ok && (err != nil || got != tt.want) {
			t.Fatalf("parseDelimiter(%q) = %q, %v", tt.input, got, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("parseDelimiter(%q) succeeded", tt.input)
		}
	}
}

func TestSniffFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		want    string
		ok      bool
	}{
		{name: "tsv", content: "a\tb\n1\t2\n", want: "tsv", ok: true},
		{name: "csv quoted tab", content: "a,\"b\tc\"\n", want: "csv", ok: true},
		{name: "csv quoted comma", content: "a,\"b,c\"\n", want: "csv", ok: true},
		{name: "empty", content: "", ok: false},
	}
	for _, tt := range tests {
		path := filepath.Join(dir, tt.name)
		if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := sniffFormat(path)
		if tt.ok && (err != nil || got != tt.want) {
			t.Fatalf("sniffFormat(%s) = %q, %v; want %q", tt.name, got, err, tt.want)
		}
		if !tt.ok && err == nil {
			t.Fatalf("sniffFormat(%s) succeeded", tt.name)
		}
	}
}

func TestDecodeRowErrors(t *testing.T) {
	t.Parallel()
	truncatedLength := []byte{0, 0, 0}
	truncatedField := make([]byte, 4)
	binary.BigEndian.PutUint32(truncatedField, 4)
	truncatedField = append(truncatedField, 'x')
	_, row, err := encodeStringFields([]string{"only"}, []int{0}, nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
		n    int
		want string
	}{
		{name: "truncated length", data: truncatedLength, n: 1, want: "truncated field length"},
		{name: "truncated field", data: truncatedField, n: 1, want: "truncated field"},
		{name: "wrong column count", data: row, n: 2, want: "expected 2 columns"},
	}
	for _, tt := range tests {
		_, err := decodeRow(tt.data, tt.n, nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("decodeRow(%s) error = %v", tt.name, err)
		}
	}
}
