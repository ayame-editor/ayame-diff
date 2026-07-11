package engine

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func Run(ctx context.Context, cfg Config) (Summary, error) {
	started := time.Now()
	var summary Summary
	resolved, err := cfg.resolve()
	if err != nil {
		return summary, err
	}
	if err := ensureFileDescriptorBudget(resolved); err != nil {
		return summary, err
	}
	if err := validateDistinctOutput(resolved.LeftPath, resolved.RightPath, resolved.OutputPath); err != nil {
		return summary, err
	}

	leftSpec, err := resolveInputSpec(resolved.LeftPath, resolved.LeftFormat, resolved.LeftDelimiter, resolved.LeftParser, "left")
	if err != nil {
		return summary, err
	}
	rightSpec, err := resolveInputSpec(resolved.RightPath, resolved.RightFormat, resolved.RightDelimiter, resolved.RightParser, "right")
	if err != nil {
		return summary, err
	}
	leftInfo, err := inspectInput(leftSpec, resolved.HasHeader, resolved.LazyQuotes, resolved.TrimLeadingSpace)
	if err != nil {
		return summary, err
	}
	rightInfo, err := inspectInput(rightSpec, resolved.HasHeader, resolved.LazyQuotes, resolved.TrimLeadingSpace)
	if err != nil {
		return summary, err
	}
	resolvedSchema, err := buildSchema(leftInfo, rightInfo, resolved.Config)
	if err != nil {
		return summary, err
	}
	resolved.Comparison = resolvedSchema.Comparison
	resolved.ComparisonHeader = append([]string(nil), resolvedSchema.Header...)

	workRoot, createdByUs, err := createWorkRoot(resolved.Config)
	if err != nil {
		return summary, err
	}
	if !resolved.KeepTemp {
		defer cleanupWorkRoot(workRoot, createdByUs)
	} else {
		defer func() {
			if resolved.Log != nil {
				fmt.Fprintln(resolved.Log, "temporary data kept at:", workRoot)
			}
		}()
	}

	leftParts, leftRows, err := partitionInput(ctx, leftSpec, leftInfo, resolvedSchema.LeftMap, resolvedSchema.KeyIndexes, resolvedSchema.KeyIsFullRow, resolved, filepath.Join(workRoot, "partitions-left"))
	if err != nil {
		return summary, fmt.Errorf("partition left input: %w", err)
	}
	rightParts, rightRows, err := partitionInput(ctx, rightSpec, rightInfo, resolvedSchema.RightMap, resolvedSchema.KeyIndexes, resolvedSchema.KeyIsFullRow, resolved, filepath.Join(workRoot, "partitions-right"))
	if err != nil {
		return summary, fmt.Errorf("partition right input: %w", err)
	}

	compareStarted := time.Now()
	emitProgress(resolved, ProgressEvent{Phase: "compare"})
	stats, outputParts, err := compareAllPartitions(ctx, leftParts, rightParts, resolvedSchema.ColumnCount, resolvedSchema.KeyIsFullRow, resolved, workRoot)
	if err != nil {
		return summary, err
	}
	emitProgress(resolved, ProgressEvent{Phase: "compare", Done: true, Elapsed: time.Since(compareStarted)})
	assembleStarted := time.Now()
	emitProgress(resolved, ProgressEvent{Phase: "assemble"})
	if err := assembleOutput(ctx, resolved.OutputPath, outputParts, resolvedSchema.Header, resolved.OutputHeader, resolved.CellDiff, resolved.OutputFormat, resolved.Reconcile, resolved.OutputDelimiter); err != nil {
		return summary, err
	}
	emitProgress(resolved, ProgressEvent{Phase: "assemble", Done: true, Elapsed: time.Since(assembleStarted)})

	summary = Summary{
		LeftRows:       leftRows,
		RightRows:      rightRows,
		EqualRows:      stats.EqualRows,
		LeftOnly:       stats.LeftOnly,
		RightOnly:      stats.RightOnly,
		ChangedLeft:    stats.ChangedLeft,
		ChangedRight:   stats.ChangedRight,
		DiffRows:       stats.DiffRows,
		Partitions:     resolved.Partitions,
		Workers:        minInt(resolved.Workers, resolved.Partitions),
		Elapsed:        time.Since(started).Round(time.Millisecond).String(),
		UnresolvedRows: stats.UnresolvedRows,
	}
	for index, count := range stats.ColumnChanges {
		if count > 0 {
			summary.ColumnChanges = append(summary.ColumnChanges, ColumnChange{Index: index, Name: resolvedSchema.Header[index], Count: count})
		}
	}
	sort.Slice(summary.ColumnChanges, func(i, j int) bool {
		if summary.ColumnChanges[i].Count != summary.ColumnChanges[j].Count {
			return summary.ColumnChanges[i].Count > summary.ColumnChanges[j].Count
		}
		return summary.ColumnChanges[i].Index < summary.ColumnChanges[j].Index
	})
	return summary, nil
}

type partitionResult struct {
	index int
	path  string
	stats partitionStats
	err   error
}

// preferRootCause chooses the error that better explains a parallel failure.
// When one worker fails it cancels the shared context, so sibling workers then
// report context.Canceled; a concrete error must win over that cancellation so
// the reported message names the real cause (#40). next is assumed non-nil.
func preferRootCause(current, next error) error {
	if current == nil || errors.Is(current, context.Canceled) {
		return next
	}
	return current
}

func compareAllPartitions(ctx context.Context, leftParts, rightParts []string, columnCount int, keyIsFullRow bool, cfg resolvedConfig, workRoot string) (partitionStats, []string, error) {
	var total partitionStats
	if len(leftParts) != cfg.Partitions || len(rightParts) != cfg.Partitions {
		return total, nil, fmt.Errorf("internal error: partition count mismatch")
	}
	workers := minInt(cfg.Workers, cfg.Partitions)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int, cfg.Partitions)
	for i := 0; i < cfg.Partitions; i++ {
		jobs <- i
	}
	close(jobs)
	results := make(chan partitionResult, cfg.Partitions)

	var wg sync.WaitGroup
	wg.Add(workers)
	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			for index := range jobs {
				if err := workerCtx.Err(); err != nil {
					results <- partitionResult{index: index, err: err}
					continue
				}
				stats, path, err := processPartition(workerCtx, index, leftParts[index], rightParts[index], columnCount, keyIsFullRow, cfg, workRoot)
				if err != nil {
					cancel()
				}
				results <- partitionResult{index: index, path: path, stats: stats, err: err}
			}
		}(workerID)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	paths := make([]string, cfg.Partitions)
	var firstErr error
	for result := range results {
		if result.err != nil {
			firstErr = preferRootCause(firstErr, fmt.Errorf("process partition %d: %w", result.index, result.err))
			continue
		}
		paths[result.index] = result.path
		total.add(result.stats)
	}
	if firstErr != nil {
		return partitionStats{}, nil, firstErr
	}
	return total, paths, nil
}

func processPartition(ctx context.Context, index int, leftPart, rightPart string, columnCount int, keyIsFullRow bool, cfg resolvedConfig, workRoot string) (partitionStats, string, error) {
	var stats partitionStats
	partitionWork := filepath.Join(workRoot, "sort", fmt.Sprintf("part-%05d", index))
	if err := os.MkdirAll(partitionWork, 0o755); err != nil {
		return stats, "", err
	}
	outputDir := filepath.Join(workRoot, "diff-parts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return stats, "", err
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("part-%05d.tsv", index))

	perWorkerMemory := cfg.MemoryBytes / int64(minInt(cfg.Workers, cfg.Partitions))
	chunkBytes := perWorkerMemory * 3 / 4
	if chunkBytes < minSortChunkBytes {
		chunkBytes = minSortChunkBytes
	}
	leftSorted, err := makeSortedFile(ctx, leftPart, partitionWork, "left", chunkBytes, cfg.MergeFanIn, cfg.MaxRecordBytes)
	if err != nil {
		return stats, "", fmt.Errorf("sort left: %w", err)
	}
	rightSorted, err := makeSortedFile(ctx, rightPart, partitionWork, "right", chunkBytes, cfg.MergeFanIn, cfg.MaxRecordBytes)
	if err != nil {
		return stats, "", fmt.Errorf("sort right: %w", err)
	}
	stats, err = compareSortedFiles(ctx, leftSorted, rightSorted, outputPath, cfg.ComparisonHeader, keyIsFullRow, cfg.MaxRecordBytes, cfg.Comparison, cfg.CellDiff, cfg.OutputFormat, reconcileConfig{
		enabled: cfg.Reconcile, choices: cfg.MergeChoices, defaultTo: cfg.MergeDefault, delimiter: cfg.OutputDelimiter, allowUnresolved: cfg.AllowUnresolved,
	})
	if err != nil {
		return stats, "", fmt.Errorf("compare: %w", err)
	}

	if !cfg.KeepTemp {
		_ = os.Remove(leftPart)
		_ = os.Remove(rightPart)
		_ = os.RemoveAll(partitionWork)
	}
	return stats, outputPath, nil
}

func assembleOutput(ctx context.Context, outputPath string, parts []string, header []string, writeHeader, cellDiff bool, outputFormat string, reconcile bool, delimiter rune) (resultErr error) {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".ayame-diff-output-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	tempOpen := true
	var compressed *gzip.Writer
	compressedOpen := false
	defer func() {
		if compressedOpen {
			if err := compressed.Close(); resultErr == nil && err != nil {
				resultErr = err
			}
		}
		if tempOpen {
			if err := temp.Close(); resultErr == nil && err != nil {
				resultErr = err
			}
		}
		if resultErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	var destination io.Writer = temp
	if strings.HasSuffix(strings.ToLower(outputPath), ".gz") {
		compressed, err = gzip.NewWriterLevel(temp, gzip.BestSpeed)
		if err != nil {
			return err
		}
		compressedOpen = true
		destination = compressed
	}
	buffer := bufio.NewWriterSize(destination, ioBufferBytes)

	if writeHeader && outputFormat == "tsv" {
		writer := csv.NewWriter(buffer)
		writer.Comma = '\t'
		if reconcile {
			writer.Comma = delimiter
		}
		if reconcile {
			if err := writer.Write(header); err != nil {
				return err
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				return err
			}
		} else {
			extra := 0
			if cellDiff {
				extra = 1
			}
			outputHeader := make([]string, 2+extra+len(header))
			outputHeader[0] = "_diff"
			outputHeader[1] = "_side"
			if cellDiff {
				outputHeader[2] = "_changed_cols"
			}
			copy(outputHeader[2+extra:], header)
			if err := writer.Write(outputHeader); err != nil {
				return err
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				return err
			}
		}
	}

	copyBuffer := make([]byte, ioBufferBytes)
	for _, partPath := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		part, err := os.Open(partPath)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyBuffer(buffer, part, copyBuffer)
		closeErr := part.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	if compressed != nil {
		closeErr := compressed.Close()
		compressedOpen = false
		if closeErr != nil {
			return closeErr
		}
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	closeErr := temp.Close()
	tempOpen = false
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	if err := replaceFile(tempPath, outputPath); err != nil {
		return err
	}
	return nil
}

func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, destination)
}

func createWorkRoot(cfg Config) (string, bool, error) {
	if cfg.WorkDir == "" {
		path, err := os.MkdirTemp(cfg.TempDir, "ayame-diff-")
		return path, true, err
	}
	path, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", false, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", false, err
	}
	if len(entries) != 0 {
		return "", false, fmt.Errorf("work directory must be empty: %s", path)
	}
	return path, false, nil
}

// cleanupWorkRoot removes the temporary root only when Run created it. For an
// explicit --work-dir, it removes generated contents but preserves the
// user-owned directory itself.
func cleanupWorkRoot(path string, createdByUs bool) {
	if createdByUs {
		_ = os.RemoveAll(path)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = os.RemoveAll(filepath.Join(path, entry.Name()))
	}
}

func validateDistinctOutput(leftPath, rightPath, outputPath string) error {
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	for label, inputPath := range map[string]string{"left": leftPath, "right": rightPath} {
		inputAbs, err := filepath.Abs(inputPath)
		if err != nil {
			return err
		}
		if inputAbs == outputAbs {
			return fmt.Errorf("output path must differ from %s input", label)
		}
		inputInfo, inputErr := os.Stat(inputPath)
		outputInfo, outputErr := os.Stat(outputPath)
		if inputErr == nil && outputErr == nil && os.SameFile(inputInfo, outputInfo) {
			return fmt.Errorf("output path refers to the same file as %s input", label)
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
