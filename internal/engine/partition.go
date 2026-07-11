package engine

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type partitionSink struct {
	mu sync.Mutex
	f  *os.File
	w  *bufio.Writer
}
type partitionSet struct {
	paths []string
	sinks []partitionSink
}

func newPartitionSet(dir string, count, buf int) (*partitionSet, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &partitionSet{make([]string, count), make([]partitionSink, count)}
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("part-%05d.bin", i))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			s.close()
			return nil, err
		}
		s.paths[i] = path
		s.sinks[i] = partitionSink{f: f, w: bufio.NewWriterSize(f, buf)}
	}
	return s, nil
}
func (s *partitionSet) write(i int, data []byte) error {
	x := &s.sinks[i]
	x.mu.Lock()
	defer x.mu.Unlock()
	_, err := x.w.Write(data)
	return err
}
func (s *partitionSet) close() error {
	var result error
	for i := range s.sinks {
		x := &s.sinks[i]
		if x.w != nil {
			if err := x.w.Flush(); err != nil && result == nil {
				result = err
			}
		}
		if x.f != nil {
			if err := x.f.Close(); err != nil && result == nil {
				result = err
			}
		}
	}
	return result
}

func partitionInput(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, outDir string) ([]string, uint64, error) {
	set, err := newPartitionSet(outDir, cfg.Partitions, cfg.PartitionBufferBytes)
	if err != nil {
		return nil, 0, err
	}
	progress := startProgress(ctx, "partition", spec.Label, cfg.Progress || cfg.OnProgress != nil, cfg.Log, cfg.OnProgress)
	defer progress.stop()
	var rows uint64
	if spec.Parser == parserSimple {
		stat, e := os.Stat(spec.Path)
		parallel := e == nil && stat.Mode().IsRegular() && !spec.Compressed && cfg.ParseWorkers > 1
		if parallel {
			rows, err = partitionSimpleParallel(ctx, spec, info, mapping, keyIndexes, keyIsFullRow, cfg, set, progress, stat.Size())
		} else {
			rows, err = partitionSimpleSequential(ctx, spec, info, mapping, keyIndexes, keyIsFullRow, cfg, set, progress)
		}
	} else {
		rows, err = partitionRFC4180(ctx, spec, info, mapping, keyIndexes, keyIsFullRow, cfg, set, progress)
	}
	closeErr := set.close()
	if err != nil {
		return nil, 0, err
	}
	if closeErr != nil {
		return nil, 0, closeErr
	}
	return append([]string(nil), set.paths...), rows, nil
}
func partitionRFC4180(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, set *partitionSet, p *progressCounter) (uint64, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	reader := csv.NewReader(bufio.NewReaderSize(r, ioBufferBytes))
	if !cfg.HasHeader && info.DataOffset > 0 {
		if _, err := io.CopyN(io.Discard, r, info.DataOffset); err != nil {
			return 0, fmt.Errorf("skip %s BOM: %w", spec.Label, err)
		}
		reader = csv.NewReader(bufio.NewReaderSize(r, ioBufferBytes))
	}
	reader.Comma = rune(spec.Delimiter)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	reader.LazyQuotes = cfg.LazyQuotes
	reader.TrimLeadingSpace = cfg.TrimLeadingSpace
	if cfg.HasHeader {
		if _, err := reader.Read(); err != nil {
			return 0, fmt.Errorf("read %s header: %w", spec.Label, err)
		}
	}
	previousOffset := reader.InputOffset()
	var rows uint64
	var key, row, stored []byte
	for {
		record, e := reader.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return rows, fmt.Errorf("read %s record %d: %w", spec.Label, rows+1, e)
		}
		currentOffset := reader.InputOffset()
		rawBytes := currentOffset - previousOffset
		previousOffset = currentOffset
		if len(record) != info.ColumnCount {
			return rows, fmt.Errorf("%s record %d has %d columns; expected %d", spec.Label, rows+1, len(record), info.ColumnCount)
		}
		key, row, err = encodeStringFields(record, mapping, keyIndexes, keyIsFullRow, key, row)
		if err != nil {
			return rows, err
		}
		stored, err = makeStoredRecord(stored, key, row, cfg.MaxRecordBytes)
		if err != nil {
			return rows, err
		}
		part := int(xxhash64(key) & uint64(cfg.Partitions-1))
		if err := set.write(part, stored); err != nil {
			return rows, err
		}
		rows++
		p.add(1, uint64(rawBytes))
		if rows&cancellationCheckMask == 0 {
			if err := ctx.Err(); err != nil {
				return rows, err
			}
		}
	}
	return rows, nil
}
func partitionSimpleSequential(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, set *partitionSet, p *progressCounter) (uint64, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	reader := bufio.NewReaderSize(r, ioBufferBytes)
	if cfg.HasHeader {
		_, raw, _, err := readPhysicalLine(reader, nil)
		if err != nil && !(errors.Is(err, io.EOF) && raw > 0) {
			return 0, err
		}
	} else if info.DataOffset > 0 {
		if _, err := reader.Discard(int(info.DataOffset)); err != nil {
			return 0, fmt.Errorf("skip %s BOM: %w", spec.Label, err)
		}
	}
	var rows uint64
	var scratch []byte
	var fields [][]byte
	var key, row, stored []byte
	for {
		line, raw, owned, e := readPhysicalLine(reader, scratch)
		if errors.Is(e, io.EOF) && raw == 0 {
			break
		}
		if e != nil && !errors.Is(e, io.EOF) {
			return rows, e
		}
		if owned {
			scratch = line[:0]
		}
		line = trimLineEnding(line)
		if len(line) == 0 {
			if errors.Is(e, io.EOF) {
				break
			}
			continue
		}
		fields = splitSimpleLine(line, spec.Delimiter, fields)
		if len(fields) != info.ColumnCount {
			return rows, fmt.Errorf("%s record %d has %d columns; expected %d", spec.Label, rows+1, len(fields), info.ColumnCount)
		}
		key, row, err = encodeByteFields(fields, mapping, keyIndexes, keyIsFullRow, key, row)
		if err != nil {
			return rows, err
		}
		stored, err = makeStoredRecord(stored, key, row, cfg.MaxRecordBytes)
		if err != nil {
			return rows, err
		}
		part := int(xxhash64(key) & uint64(cfg.Partitions-1))
		if err := set.write(part, stored); err != nil {
			return rows, err
		}
		rows++
		p.add(1, uint64(raw))
		if rows&cancellationCheckMask == 0 {
			if err := ctx.Err(); err != nil {
				return rows, err
			}
		}
		if errors.Is(e, io.EOF) {
			break
		}
	}
	return rows, nil
}
func partitionSimpleParallel(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, set *partitionSet, p *progressCounter, fileSize int64) (uint64, error) {
	return partitionSimpleParallelWithChunk(ctx, spec, info, mapping, keyIndexes, keyIsFullRow, cfg, set, p, fileSize, minParallelSpanBytes)
}

// partitionSimpleParallelWithChunk exposes the minimum bytes per worker as an
// argument so tests can exercise true multi-worker range splitting with small
// deterministic fixtures.
func partitionSimpleParallelWithChunk(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, set *partitionSet, p *progressCounter, fileSize, minChunkBytes int64) (uint64, error) {
	start := info.DataOffset
	if start >= fileSize {
		return 0, nil
	}
	workers := cfg.ParseWorkers
	data := fileSize - start
	if minChunkBytes < 1 {
		minChunkBytes = 1
	}
	if max := int(data / minChunkBytes); max+1 < workers {
		workers = max + 1
	}
	if workers < 1 {
		workers = 1
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type res struct {
		rows uint64
		err  error
	}
	ch := make(chan res, workers)
	for w := 0; w < workers; w++ {
		a := start + data*int64(w)/int64(workers)
		b := start + data*int64(w+1)/int64(workers)
		last := w == workers-1
		go func(a, b int64, last bool) {
			rows, err := partitionSimpleRange(workerCtx, spec, info, mapping, keyIndexes, keyIsFullRow, cfg, set, p, start, a, b, last)
			if err != nil {
				cancel()
			}
			ch <- res{rows, err}
		}(a, b, last)
	}
	var total uint64
	var first error
	for i := 0; i < workers; i++ {
		x := <-ch
		total += x.rows
		if x.err != nil {
			first = preferRootCause(first, x.err)
		}
	}
	if first != nil {
		return total, first
	}
	return total, ctx.Err()
}
func partitionSimpleRange(ctx context.Context, spec inputSpec, info inspectedInput, mapping, keyIndexes []int, keyIsFullRow bool, cfg resolvedConfig, set *partitionSet, p *progressCounter, dataStart, nominalStart, nominalEnd int64, last bool) (uint64, error) {
	f, err := os.Open(spec.Path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	start, err := alignSimpleRangeStart(f, nominalStart, dataStart)
	if err != nil {
		return 0, err
	}
	if start >= nominalEnd && !last {
		return 0, nil
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}
	reader := bufio.NewReaderSize(f, ioBufferBytes)
	pos := start
	var rows uint64
	var scratch []byte
	var fields [][]byte
	var key, row, stored []byte
	for last || pos < nominalEnd {
		if err := ctx.Err(); err != nil {
			return rows, err
		}
		line, raw, owned, e := readPhysicalLine(reader, scratch)
		if errors.Is(e, io.EOF) && raw == 0 {
			break
		}
		if e != nil && !errors.Is(e, io.EOF) {
			return rows, e
		}
		offset := pos
		pos += int64(raw)
		if owned {
			scratch = line[:0]
		}
		line = trimLineEnding(line)
		if len(line) == 0 {
			if errors.Is(e, io.EOF) {
				break
			}
			continue
		}
		fields = splitSimpleLine(line, spec.Delimiter, fields)
		if len(fields) != info.ColumnCount {
			return rows, fmt.Errorf("%s record near byte %d has %d columns; expected %d", spec.Label, offset, len(fields), info.ColumnCount)
		}
		key, row, err = encodeByteFields(fields, mapping, keyIndexes, keyIsFullRow, key, row)
		if err != nil {
			return rows, err
		}
		stored, err = makeStoredRecord(stored, key, row, cfg.MaxRecordBytes)
		if err != nil {
			return rows, err
		}
		part := int(xxhash64(key) & uint64(cfg.Partitions-1))
		if err := set.write(part, stored); err != nil {
			return rows, err
		}
		rows++
		p.add(1, uint64(raw))
		if errors.Is(e, io.EOF) {
			break
		}
	}
	return rows, nil
}
func alignSimpleRangeStart(f *os.File, start, dataStart int64) (int64, error) {
	if start <= dataStart {
		return dataStart, nil
	}
	var prev [1]byte
	if _, err := f.ReadAt(prev[:], start-1); err != nil {
		return 0, err
	}
	if prev[0] == '\n' {
		return start, nil
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}
	r := bufio.NewReaderSize(f, 256*1024)
	_, n, _, err := readPhysicalLine(r, nil)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	return start + int64(n), nil
}
func readPhysicalLine(r *bufio.Reader, scratch []byte) ([]byte, int, bool, error) {
	scratch = scratch[:0]
	total := 0
	for {
		fragment, err := r.ReadSlice('\n')
		total += len(fragment)
		if len(scratch) == 0 && !errors.Is(err, bufio.ErrBufferFull) {
			return fragment, total, false, err
		}
		scratch = append(scratch, fragment...)
		if !errors.Is(err, bufio.ErrBufferFull) {
			return scratch, total, true, err
		}
	}
}
