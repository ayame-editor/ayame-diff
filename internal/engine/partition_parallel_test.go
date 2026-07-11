package engine

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAlignSimpleRangeStart(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "lines.tsv")
	if err := os.WriteFile(path, []byte("aa\r\nbbb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		start     int64
		dataStart int64
		want      int64
	}{
		{name: "data start", start: 0, dataStart: 0, want: 0},
		{name: "before CRLF", start: 2, want: 4},
		{name: "inside CRLF", start: 3, want: 4},
		{name: "after CRLF", start: 4, want: 4},
		{name: "on LF", start: 7, want: 8},
		{name: "after LF", start: 8, want: 8},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, err := alignSimpleRangeStart(f, tt.start, tt.dataStart)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("alignSimpleRangeStart(%d, %d) = %d, want %d", tt.start, tt.dataStart, got, tt.want)
			}
		})
	}
}

func TestPartitionSimpleParallelMatchesSequential(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(46))
	for trial := 0; trial < 25; trial++ {
		var input strings.Builder
		rows := 50 + rng.Intn(200)
		for row := 0; row < rows; row++ {
			leftLen := rng.Intn(80)
			rightLen := rng.Intn(120)
			input.WriteString(strings.Repeat(string(rune('a'+rng.Intn(26))), leftLen))
			input.WriteByte('\t')
			input.WriteString(strings.Repeat(string(rune('A'+rng.Intn(26))), rightLen))
			if rng.Intn(2) == 0 {
				input.WriteString("\r\n")
			} else {
				input.WriteByte('\n')
			}
		}
		sequential, sequentialRows := runSimplePartitionMode(t, input.String(), false)
		parallel, parallelRows := runSimplePartitionMode(t, input.String(), true)
		if sequentialRows != parallelRows || !reflect.DeepEqual(sequential, parallel) {
			t.Fatalf("trial %d differs: sequential rows=%d records=%d, parallel rows=%d records=%d",
				trial, sequentialRows, len(sequential), parallelRows, len(parallel))
		}
	}
}

func TestPartitionSimpleParallelLongLine(t *testing.T) {
	t.Parallel()
	// The range reader uses a 4 MiB bufio.Reader. This forces readPhysicalLine
	// through one or more bufio.ErrBufferFull fragments.
	input := "key\t" + strings.Repeat("x", 4*1024*1024+257)
	sequential, sequentialRows := runSimplePartitionMode(t, input, false)
	parallel, parallelRows := runSimplePartitionMode(t, input, true)
	if sequentialRows != 1 || parallelRows != 1 || !reflect.DeepEqual(sequential, parallel) {
		t.Fatalf("long line differs: sequential rows=%d parallel rows=%d", sequentialRows, parallelRows)
	}
}

func runSimplePartitionMode(t *testing.T, content string, parallel bool) ([]string, uint64) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.tsv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := inputSpec{Path: path, Delimiter: '\t', Parser: parserSimple, Label: "test"}
	info, err := inspectSimple(spec, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := validConfigForValidation()
	cfg.LeftPath, cfg.RightPath, cfg.OutputPath = path, path, filepath.Join(dir, "out.tsv")
	cfg.HasHeader = false
	cfg.AlignColumnsByName = false
	cfg.Partitions = 4
	cfg.ParseWorkers = 4
	cfg.MaxRecordText = "8MiB"
	resolved, err := cfg.resolve()
	if err != nil {
		t.Fatal(err)
	}
	set, err := newPartitionSet(filepath.Join(dir, "parts"), resolved.Partitions, resolved.PartitionBufferBytes)
	if err != nil {
		t.Fatal(err)
	}
	p := startProgress(context.Background(), "partition", "test", false, nil, nil)
	var rows uint64
	if parallel {
		rows, err = partitionSimpleParallelWithChunk(context.Background(), spec, info, identityMap(2), []int{0, 1}, true, resolved, set, p, int64(len(content)), 1)
	} else {
		rows, err = partitionSimpleSequential(context.Background(), spec, info, identityMap(2), []int{0, 1}, true, resolved, set, p)
	}
	closeErr := set.close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return collectPartitionRecords(t, set.paths, resolved.MaxRecordBytes), rows
}

func collectPartitionRecords(t *testing.T, paths []string, maxRecordBytes int64) []string {
	t.Helper()
	var records []string
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		reader := newBinRecordReader(bufio.NewReader(f), 4096, maxRecordBytes)
		for {
			record, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				_ = f.Close()
				t.Fatal(err)
			}
			records = append(records, string(record.Key)+"\x00"+string(record.Row))
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(records)
	return records
}
