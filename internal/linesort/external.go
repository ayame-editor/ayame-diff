package linesort

import (
	"bufio"
	"container/heap"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// maxFanIn bounds how many run files one merge pass opens at once, so sorting a
// file far larger than memory cannot exhaust the process's file descriptors.
// Runs beyond the fan-in are merged in additional passes.
const maxFanIn = 64

// runWriteBuffer sizes the buffered writer used for each spilled run.
const runWriteBuffer = 256 * 1024

// writeRun sorts chunk and writes it to a new run file under dir, returning the
// path. The chunk is sorted before writing so the merge only ever streams.
func writeRun(dir string, index int, chunk []string, less func(a, b string) bool) (string, error) {
	sort.SliceStable(chunk, func(a, b int) bool { return less(chunk[a], chunk[b]) })
	path := filepath.Join(dir, fmt.Sprintf("run-%05d", index))
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	writer := bufio.NewWriterSize(file, runWriteBuffer)
	for _, line := range chunk {
		// A line can never contain LF or CR: every source splits on them, so
		// newline framing round-trips the run exactly.
		if _, err := writer.WriteString(line); err != nil {
			file.Close()
			return "", err
		}
		if err := writer.WriteByte('\n'); err != nil {
			file.Close()
			return "", err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return "", err
	}
	return path, file.Close()
}

// runReader streams one sorted run.
type runReader struct {
	file   *os.File
	reader *bufio.Reader
	line   string
	ok     bool
}

func openRun(path string) (*runReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r := &runReader{file: file, reader: bufio.NewReaderSize(file, runWriteBuffer)}
	if err := r.advance(); err != nil {
		file.Close()
		return nil, err
	}
	return r, nil
}

// advance loads the next line, clearing ok at end of run.
func (r *runReader) advance() error {
	data, err := r.reader.ReadBytes('\n')
	if len(data) == 0 {
		r.ok = false
		if err == io.EOF {
			return nil
		}
		return err
	}
	if data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	r.line, r.ok = string(data), true
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (r *runReader) Close() error { return r.file.Close() }

// runHeap orders the open runs by their current line, so the merge always pops
// the globally smallest remaining line.
type runHeap struct {
	runs []*runReader
	less func(a, b string) bool
}

func (h *runHeap) Len() int      { return len(h.runs) }
func (h *runHeap) Swap(i, j int) { h.runs[i], h.runs[j] = h.runs[j], h.runs[i] }
func (h *runHeap) Less(i, j int) bool {
	return h.less(h.runs[i].line, h.runs[j].line)
}
func (h *runHeap) Push(x any) { h.runs = append(h.runs, x.(*runReader)) }
func (h *runHeap) Pop() any {
	last := len(h.runs) - 1
	item := h.runs[last]
	h.runs = h.runs[:last]
	return item
}

// mergeRuns reduces runs to a single sorted file, in as many bounded-fan-in
// passes as needed, and returns its path. Memory stays proportional to the
// fan-in, not to the data.
func mergeRuns(dir string, runs []string, less func(a, b string) bool) (string, error) {
	pass := 0
	for len(runs) > 1 {
		var next []string
		for start := 0; start < len(runs); start += maxFanIn {
			group := runs[start:min(start+maxFanIn, len(runs))]
			if len(group) == 1 {
				next = append(next, group[0])
				continue
			}
			path := filepath.Join(dir, fmt.Sprintf("merge-%d-%05d", pass, len(next)))
			if err := mergeGroup(path, group, less); err != nil {
				return "", err
			}
			// The inputs are consumed; reclaim their space before the next pass
			// so peak disk stays close to one copy of the data.
			for _, consumed := range group {
				_ = os.Remove(consumed)
			}
			next = append(next, path)
		}
		runs, pass = next, pass+1
	}
	return runs[0], nil
}

// mergeGroup k-way merges group into path.
func mergeGroup(path string, group []string, less func(a, b string) bool) (resultErr error) {
	// opened tracks every reader for closing; the heap holds only the runs that
	// still have a line to contribute.
	var opened []*runReader
	defer func() {
		for _, r := range opened {
			if err := r.Close(); resultErr == nil && err != nil {
				resultErr = err
			}
		}
	}()
	h := &runHeap{less: less}
	for _, runPath := range group {
		reader, err := openRun(runPath)
		if err != nil {
			return err
		}
		opened = append(opened, reader)
		if reader.ok {
			h.runs = append(h.runs, reader)
		}
	}
	heap.Init(h)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	writer := bufio.NewWriterSize(file, runWriteBuffer)
	for h.Len() > 0 {
		smallest := h.runs[0]
		if _, err := writer.WriteString(smallest.line); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if err := smallest.advance(); err != nil {
			return err
		}
		if smallest.ok {
			heap.Fix(h, 0)
			continue
		}
		heap.Pop(h)
	}
	return writer.Flush()
}
