package engine

import (
	"bufio"
	"bytes"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func makeSortedFile(ctx context.Context, partitionPath, workDir, prefix string, chunkBytes int64, mergeFanIn int, maxRecordBytes int64) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	stat, err := os.Stat(partitionPath)
	if err != nil {
		return "", err
	}
	if stat.Size() == 0 {
		path := filepath.Join(workDir, prefix+"-empty.sorted.bin")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return path, nil
	}

	runs, err := createSortedRuns(ctx, partitionPath, workDir, prefix, chunkBytes, maxRecordBytes)
	if err != nil {
		return "", err
	}
	pass := 0
	for len(runs) > 1 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		next := make([]string, 0, (len(runs)+mergeFanIn-1)/mergeFanIn)
		for start := 0; start < len(runs); start += mergeFanIn {
			end := start + mergeFanIn
			if end > len(runs) {
				end = len(runs)
			}
			group := runs[start:end]
			if len(group) == 1 {
				next = append(next, group[0])
				continue
			}
			out := filepath.Join(workDir, fmt.Sprintf("%s-merge-%03d-%05d.bin", prefix, pass, len(next)))
			if err := mergeRunGroup(ctx, group, out, maxRecordBytes); err != nil {
				_ = os.Remove(out)
				return "", err
			}
			for _, path := range group {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return "", err
				}
			}
			next = append(next, out)
		}
		runs = next
		pass++
	}
	return runs[0], nil
}

func createSortedRuns(ctx context.Context, inputPath, workDir, prefix string, chunkBytes, maxRecordBytes int64) ([]string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := newBinRecordReader(f, ioBufferBytes, maxRecordBytes)
	runs := make([]string, 0, 8)
	eof := false
	for !eof {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		records := make([]binRecord, 0, 4096)
		var used int64
		for used < chunkBytes || len(records) == 0 {
			record, readErr := reader.Next()
			if errors.Is(readErr, io.EOF) {
				eof = true
				break
			}
			if readErr != nil {
				return nil, readErr
			}
			records = append(records, record)
			used += int64(8 + len(record.Key) + len(record.Row) + recordMemoryOverhead)
			if len(records)&cancellationCheckMask == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
		}
		if len(records) == 0 {
			break
		}
		sort.Slice(records, func(i, j int) bool {
			if c := bytes.Compare(records[i].Key, records[j].Key); c != 0 {
				return c < 0
			}
			return bytes.Compare(records[i].Row, records[j].Row) < 0
		})
		path := filepath.Join(workDir, fmt.Sprintf("%s-run-%05d.bin", prefix, len(runs)))
		if err := writeRun(path, records); err != nil {
			_ = os.Remove(path)
			return nil, err
		}
		runs = append(runs, path)
	}
	if len(runs) == 0 {
		return nil, fmt.Errorf("no sorted runs produced from non-empty partition %s", inputPath)
	}
	return runs, nil
}

func writeRun(path string, records []binRecord) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, ioBufferBytes)
	var writeErr error
	for _, record := range records {
		if err := writeBinRecord(w, record); err != nil {
			writeErr = err
			break
		}
	}
	if err := w.Flush(); writeErr == nil && err != nil {
		writeErr = err
	}
	if err := f.Close(); writeErr == nil && err != nil {
		writeErr = err
	}
	return writeErr
}

type runSource struct {
	file   *os.File
	reader *binRecordReader
}

type runHeapItem struct {
	record binRecord
	source int
}

type runHeap []runHeapItem

func (h runHeap) Len() int { return len(h) }
func (h runHeap) Less(i, j int) bool {
	if c := bytes.Compare(h[i].record.Key, h[j].record.Key); c != 0 {
		return c < 0
	}
	return bytes.Compare(h[i].record.Row, h[j].record.Row) < 0
}
func (h runHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *runHeap) Push(x any)   { *h = append(*h, x.(runHeapItem)) }
func (h *runHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = runHeapItem{}
	*h = old[:n-1]
	return x
}

func mergeRunGroup(ctx context.Context, inputs []string, output string, maxRecordBytes int64) (resultErr error) {
	sources := make([]runSource, len(inputs))
	defer func() {
		for i := range sources {
			if sources[i].file != nil {
				if err := sources[i].file.Close(); resultErr == nil && err != nil {
					resultErr = err
				}
			}
		}
	}()

	h := make(runHeap, 0, len(inputs))
	for i, path := range inputs {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		sources[i] = runSource{file: f, reader: newBinRecordReader(f, 1024*1024, maxRecordBytes)}
		record, err := sources[i].reader.Next()
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return err
		}
		h = append(h, runHeapItem{record: record, source: i})
	}
	heap.Init(&h)

	out, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	writer := bufio.NewWriterSize(out, ioBufferBytes)
	defer func() {
		if err := writer.Flush(); resultErr == nil && err != nil {
			resultErr = err
		}
		if err := out.Close(); resultErr == nil && err != nil {
			resultErr = err
		}
		if resultErr != nil {
			_ = os.Remove(output)
		}
	}()

	var count uint64
	for h.Len() > 0 {
		item := heap.Pop(&h).(runHeapItem)
		if err := writeBinRecord(writer, item.record); err != nil {
			return err
		}
		next, err := sources[item.source].reader.Next()
		if err == nil {
			heap.Push(&h, runHeapItem{record: next, source: item.source})
		} else if !errors.Is(err, io.EOF) {
			return err
		}
		count++
		if count&cancellationCheckMask == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
	return nil
}
