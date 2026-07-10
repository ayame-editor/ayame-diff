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
	"strings"
	"sync"
)

func Run(ctx context.Context, cfg Config) (Summary, error) {
	var summary Summary
	if err := cfg.Validate(); err != nil {
		return summary, err
	}
	if err := validateDistinctOutput(cfg.LeftPath, cfg.RightPath, cfg.OutputPath); err != nil {
		return summary, err
	}

	leftSpec, err := resolveInputSpec(cfg.LeftPath, cfg.LeftFormat, cfg.LeftDelimiter, cfg.LeftParser, "left")
	if err != nil {
		return summary, err
	}
	rightSpec, err := resolveInputSpec(cfg.RightPath, cfg.RightFormat, cfg.RightDelimiter, cfg.RightParser, "right")
	if err != nil {
		return summary, err
	}
	leftInfo, err := inspectInput(leftSpec, cfg.HasHeader, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return summary, err
	}
	rightInfo, err := inspectInput(rightSpec, cfg.HasHeader, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return summary, err
	}
	resolvedSchema, err := buildSchema(leftInfo, rightInfo, cfg)
	if err != nil {
		return summary, err
	}

	workRoot, createdByUs, err := createWorkRoot(cfg)
	if err != nil {
		return summary, err
	}
	if !cfg.KeepTemp {
		defer os.RemoveAll(workRoot)
	} else {
		fmt.Fprintln(os.Stderr, "work directory:", workRoot)
	}

	leftParts, leftRows, err := partitionInput(ctx, leftSpec, leftInfo, resolvedSchema.LeftMap, resolvedSchema.KeyIndexes, resolvedSchema.KeyIsFullRow, cfg, filepath.Join(workRoot, "partitions-left"))
	if err != nil {
		return summary, fmt.Errorf("partition left input: %w", err)
	}
	rightParts, rightRows, err := partitionInput(ctx, rightSpec, rightInfo, resolvedSchema.RightMap, resolvedSchema.KeyIndexes, resolvedSchema.KeyIsFullRow, cfg, filepath.Join(workRoot, "partitions-right"))
	if err != nil {
		return summary, fmt.Errorf("partition right input: %w", err)
	}

	stats, outputParts, err := compareAllPartitions(ctx, leftParts, rightParts, resolvedSchema.ColumnCount, resolvedSchema.KeyIsFullRow, cfg, workRoot)
	if err != nil {
		return summary, err
	}
	if err := assembleOutput(ctx, cfg.OutputPath, outputParts, resolvedSchema.Header, cfg.OutputHeader); err != nil {
		return summary, err
	}

	summary = Summary{
		LeftRows:     leftRows,
		RightRows:    rightRows,
		EqualRows:    stats.EqualRows,
		LeftOnly:     stats.LeftOnly,
		RightOnly:    stats.RightOnly,
		ChangedLeft:  stats.ChangedLeft,
		ChangedRight: stats.ChangedRight,
		DiffRows:     stats.DiffRows,
		Partitions:   cfg.Partitions,
		Workers:      minInt(cfg.Workers, cfg.Partitions),
	}
	if createdByUs && cfg.KeepTemp {
		fmt.Fprintln(os.Stderr, "temporary data kept at:", workRoot)
	}
	return summary, nil
}

type partitionResult struct {
	index int
	path  string
	stats partitionStats
	err   error
}

func compareAllPartitions(ctx context.Context, leftParts, rightParts []string, columnCount int, keyIsFullRow bool, cfg Config, workRoot string) (partitionStats, []string, error) {
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
			if firstErr == nil {
				firstErr = fmt.Errorf("process partition %d: %w", result.index, result.err)
			}
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

func processPartition(ctx context.Context, index int, leftPart, rightPart string, columnCount int, keyIsFullRow bool, cfg Config, workRoot string) (partitionStats, string, error) {
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
	if chunkBytes < 8*1024*1024 {
		chunkBytes = 8 * 1024 * 1024
	}
	leftSorted, err := makeSortedFile(ctx, leftPart, partitionWork, "left", chunkBytes, cfg.MergeFanIn, cfg.MaxRecordBytes)
	if err != nil {
		return stats, "", fmt.Errorf("sort left: %w", err)
	}
	rightSorted, err := makeSortedFile(ctx, rightPart, partitionWork, "right", chunkBytes, cfg.MergeFanIn, cfg.MaxRecordBytes)
	if err != nil {
		return stats, "", fmt.Errorf("sort right: %w", err)
	}
	stats, err = compareSortedFiles(ctx, leftSorted, rightSorted, outputPath, columnCount, keyIsFullRow, cfg.MaxRecordBytes)
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

func assembleOutput(ctx context.Context, outputPath string, parts []string, header []string, writeHeader bool) (resultErr error) {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".ayame-diff-output-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if resultErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	var compressed *gzip.Writer
	var destination io.Writer = temp
	if strings.HasSuffix(strings.ToLower(outputPath), ".gz") {
		compressed, err = gzip.NewWriterLevel(temp, gzip.BestSpeed)
		if err != nil {
			_ = temp.Close()
			return err
		}
		destination = compressed
	}
	buffer := bufio.NewWriterSize(destination, 4*1024*1024)

	if writeHeader {
		writer := csv.NewWriter(buffer)
		writer.Comma = '\t'
		outputHeader := make([]string, 2+len(header))
		outputHeader[0] = "_diff"
		outputHeader[1] = "_side"
		copy(outputHeader[2:], header)
		if err := writer.Write(outputHeader); err != nil {
			_ = temp.Close()
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			_ = temp.Close()
			return err
		}
	}

	copyBuffer := make([]byte, 4*1024*1024)
	for _, partPath := range parts {
		if err := ctx.Err(); err != nil {
			_ = temp.Close()
			return err
		}
		part, err := os.Open(partPath)
		if err != nil {
			_ = temp.Close()
			return err
		}
		_, copyErr := io.CopyBuffer(buffer, part, copyBuffer)
		closeErr := part.Close()
		if copyErr != nil {
			_ = temp.Close()
			return copyErr
		}
		if closeErr != nil {
			_ = temp.Close()
			return closeErr
		}
	}
	if err := buffer.Flush(); err != nil {
		_ = temp.Close()
		return err
	}
	if compressed != nil {
		if err := compressed.Close(); err != nil {
			_ = temp.Close()
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
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
		return "", false, fmt.Errorf("--work-dir must be empty: %s", path)
	}
	return path, false, nil
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
