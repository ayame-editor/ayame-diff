package threeway

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/encoding"
)

// readMergedCSV decodes a merged file with encName and returns its records, so
// assertions compare logical values rather than the raw bytes the codec chose.
func readMergedCSV(t *testing.T, path, encName string, comma rune) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	decoded, err := io.ReadAll(encoding.Decoder(strings.NewReader(string(raw)), encName))
	if err != nil {
		t.Fatalf("decode %s as %s: %v", path, encName, err)
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(decoded), "\ufeff")))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return records
}

// TestCompareCSVNonASCIIKeyPreservesEncoding is the #160 case 1 regression. The
// byte-oriented engine reported keys as raw Shift_JIS bytes while the merge
// writer read the base decoded to UTF-8, so a non-ASCII key never matched: the
// base row survived, the replacement was appended at the end still in raw
// Shift_JIS bytes, and the output was silently rewritten as UTF-8.
func TestCompareCSVNonASCIIKeyPreservesEncoding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	writeRaw(t, base, encodeBytes(t, "id,v\n日本,base\n東京,keep\n", encoding.ShiftJIS))
	writeRaw(t, left, encodeBytes(t, "id,v\n日本,左\n東京,keep\n", encoding.ShiftJIS))
	writeRaw(t, right, encodeBytes(t, "id,v\n日本,base\n東京,keep\n", encoding.ShiftJIS))
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conflicts != 0 {
		t.Fatalf("non-ASCII key produced conflicts: %+v", result.Events)
	}
	output := filepath.Join(dir, "merged.csv")
	if _, err := WriteCSVMerge(base, output, result, nil, false); err != nil {
		t.Fatal(err)
	}
	records := readMergedCSV(t, output, encoding.ShiftJIS, ',')
	want := [][]string{{"id", "v"}, {"日本", "左"}, {"東京", "keep"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v want=%#v", records, want)
	}
}

// TestCompareCSVDuplicateKeyIndependentEdits is the #160 case 2 regression.
// Within one duplicated key group, LEFT edits one base row and RIGHT edits a
// different one. Those edits are independent, so the group must auto-merge to
// hold both results instead of reporting a conflict whose every resolution
// silently drops one side's edit.
func TestCompareCSVDuplicateKeyIndependentEdits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	write := func(path, value string) {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(base, "id,v\n1,r1\n1,r2\n")
	write(left, "id,v\n1,x\n1,r2\n")
	write(right, "id,v\n1,r1\n1,y\n")
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conflicts != 0 || result.Merged != 1 {
		t.Fatalf("independent edits must auto-merge: conflicts=%d merged=%d events=%+v", result.Conflicts, result.Merged, result.Events)
	}
	if kind := result.Events[0].Kind; kind != Merged {
		t.Fatalf("kind=%s want=%s", kind, Merged)
	}
	output := filepath.Join(dir, "merged.csv")
	if _, err := WriteCSVMerge(base, output, result, nil, false); err != nil {
		t.Fatal(err)
	}
	records := readMergedCSV(t, output, encoding.UTF8, ',')
	want := [][]string{{"id", "v"}, {"1", "x"}, {"1", "y"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v want=%#v", records, want)
	}
}

// TestCompareCSVKeepsRowOrderWithinChangedGroup is the #160 case 3 regression.
// A replacement row must be emitted where the base row it replaces sat, not
// hoisted to the group's first occurrence, so rows interleaved with other keys
// keep their base positions.
func TestCompareCSVKeepsRowOrderWithinChangedGroup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	write := func(path, value string) {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(base, "id,v\n1,a\n2,b\n1,c\n")
	write(left, "id,v\n1,a\n2,b\n1,z\n")
	write(right, "id,v\n1,a\n2,b\n1,c\n")
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "merged.csv")
	if _, err := WriteCSVMerge(base, output, result, nil, false); err != nil {
		t.Fatal(err)
	}
	records := readMergedCSV(t, output, encoding.UTF8, ',')
	want := [][]string{{"id", "v"}, {"1", "a"}, {"2", "b"}, {"1", "z"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v want=%#v", records, want)
	}
}

// TestWriteCSVMergeRoundTripsBaseConventions is the #160 case 1 regression at
// the byte level: a merged CSV must come back in the base file's encoding, BOM,
// and line terminator instead of being silently rewritten as BOM-less UTF-8
// with LF. Each case edits one row on the left only, so the merge auto-resolves
// and the output equals the left content in base's bytes.
func TestWriteCSVMergeRoundTripsBaseConventions(t *testing.T) {
	t.Parallel()
	baseLines := []string{"id,v", "日本,基準", "東京,keep"}
	leftLines := []string{"id,v", "日本,左", "東京,keep"}
	for _, tc := range []struct {
		name, enc, eol string
		bom            bool
	}{
		{"utf8-lf", encoding.UTF8, "\n", false},
		{"utf8-bom-crlf", encoding.UTF8, "\r\n", true},
		{"shift_jis-crlf", encoding.ShiftJIS, "\r\n", false},
		{"euc-jp-lf", encoding.EUCJP, "\n", false},
		{"utf16le-lf", encoding.UTF16LE, "\n", false},
		{"iso2022jp-lf", encoding.ISO2022JP, "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
			writeRaw(t, base, rawFile(t, baseLines, tc.eol, true, tc.bom, tc.enc))
			writeRaw(t, left, rawFile(t, leftLines, tc.eol, true, tc.bom, tc.enc))
			writeRaw(t, right, rawFile(t, baseLines, tc.eol, true, tc.bom, tc.enc))
			cfg := threeWayTestConfig()
			cfg.KeyNames = []string{"id"}
			result, err := CompareCSV(context.Background(), base, left, right, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if result.Conflicts != 0 {
				t.Fatalf("left-only edit conflicted: %+v", result.Events)
			}
			output := filepath.Join(dir, "merged.csv")
			if _, err := WriteCSVMerge(base, output, result, nil, false); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			want := rawFile(t, leftLines, tc.eol, true, tc.bom, tc.enc)
			if !bytes.Equal(got, want) {
				t.Fatalf("merged bytes=%x\nwant        =%x", got, want)
			}
		})
	}
}
