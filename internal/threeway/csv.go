package threeway

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/atomicfile"
	"github.com/hjosugi/ayame-diff/internal/encoding"
	"github.com/hjosugi/ayame-diff/internal/engine"
)

type CSVEvent struct {
	ID    string     `json:"id"`
	Kind  Kind       `json:"kind"`
	Key   []string   `json:"key"`
	Base  [][]string `json:"base,omitempty"`
	Left  [][]string `json:"left,omitempty"`
	Right [][]string `json:"right,omitempty"`
}

type CSVResult struct {
	Header           []string   `json:"header"`
	HasHeader        bool       `json:"has_header"`
	KeyIndexes       []int      `json:"key_indexes"`
	Events           []CSVEvent `json:"events"`
	Conflicts        int        `json:"conflicts"`
	LeftOnly         int        `json:"left_only"`
	RightOnly        int        `json:"right_only"`
	Same             int        `json:"same_change"`
	BaseDelimiter    rune       `json:"-"`
	LazyQuotes       bool       `json:"-"`
	TrimLeadingSpace bool       `json:"-"`
}

type csvDiffRecord struct {
	Kind string   `json:"kind"`
	Old  []string `json:"old"`
	New  []string `json:"new"`
}
type csvState struct {
	key                       []string
	leftBase, rightBase       [][]string
	leftRows, rightRows       [][]string
	leftTouched, rightTouched bool
}

// CompareCSV runs the existing external-sort diff twice and joins only the
// changed key groups in memory. Inputs themselves remain streaming/bounded.
func CompareCSV(ctx context.Context, basePath, leftPath, rightPath string, cfg engine.Config) (CSVResult, error) {
	cfg.LeftPath, cfg.RightPath = basePath, leftPath
	inspection, err := engine.InspectInputs(cfg)
	if err != nil {
		return CSVResult{}, err
	}
	indexes, err := csvKeyIndexes(inspection.Header, cfg)
	if err != nil {
		return CSVResult{}, err
	}
	dir, err := os.MkdirTemp("", "ayame-three-way-csv-")
	if err != nil {
		return CSVResult{}, err
	}
	defer os.RemoveAll(dir)
	states := make(map[string]*csvState)
	runSide := func(side string, path string) error {
		pair := cfg
		pair.LeftPath, pair.RightPath = basePath, path
		pair.OutputPath = filepath.Join(dir, side+".jsonl")
		pair.CellDiff, pair.OutputFormat, pair.OutputHeader = true, "jsonl", false
		pair.Reconcile, pair.MergeChoices, pair.MergeDefault = false, nil, ""
		if _, err := engine.Run(ctx, pair); err != nil {
			return err
		}
		file, err := os.Open(pair.OutputPath)
		if err != nil {
			return err
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		for {
			var item csvDiffRecord
			if err := decoder.Decode(&item); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			row := item.Old
			if len(row) == 0 {
				row = item.New
			}
			key, encoded, err := csvKey(row, indexes)
			if err != nil {
				return err
			}
			state := states[encoded]
			if state == nil {
				state = &csvState{key: key}
				states[encoded] = state
			}
			if side == "left" {
				state.leftTouched = true
				if len(item.Old) > 0 {
					state.leftBase = append(state.leftBase, item.Old)
				}
				if len(item.New) > 0 {
					state.leftRows = append(state.leftRows, item.New)
				}
			} else {
				state.rightTouched = true
				if len(item.Old) > 0 {
					state.rightBase = append(state.rightBase, item.Old)
				}
				if len(item.New) > 0 {
					state.rightRows = append(state.rightRows, item.New)
				}
			}
		}
	}
	if err := runSide("left", leftPath); err != nil {
		return CSVResult{}, fmt.Errorf("base/left: %w", err)
	}
	if err := runSide("right", rightPath); err != nil {
		return CSVResult{}, fmt.Errorf("base/right: %w", err)
	}
	result := CSVResult{Header: inspection.Header, HasHeader: cfg.HasHeader, KeyIndexes: indexes, BaseDelimiter: inputDelimiter(basePath, cfg.LeftFormat, cfg.LeftDelimiter), LazyQuotes: cfg.LazyQuotes, TrimLeadingSpace: cfg.TrimLeadingSpace}
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, encoded := range keys {
		state := states[encoded]
		baseRows := maxRowMultiset(state.leftBase, state.rightBase)
		leftRows, rightRows := state.leftRows, state.rightRows
		if !state.leftTouched {
			leftRows = cloneRows(baseRows)
		}
		if !state.rightTouched {
			rightRows = cloneRows(baseRows)
		}
		sortRows(baseRows)
		sortRows(leftRows)
		sortRows(rightRows)
		kind := Conflict
		switch {
		case !state.leftTouched:
			kind = RightOnly
		case !state.rightTouched:
			kind = LeftOnly
		case equalRows(leftRows, rightRows):
			kind = Same
		case equalRows(leftRows, baseRows):
			kind = RightOnly
		case equalRows(rightRows, baseRows):
			kind = LeftOnly
		}
		hash := sha256.Sum256([]byte(encoded))
		event := CSVEvent{ID: hex.EncodeToString(hash[:16]), Kind: kind, Key: state.key, Base: baseRows, Left: leftRows, Right: rightRows}
		result.Events = append(result.Events, event)
		switch kind {
		case Conflict:
			result.Conflicts++
		case LeftOnly:
			result.LeftOnly++
		case RightOnly:
			result.RightOnly++
		case Same:
			result.Same++
		}
	}
	return result, nil
}

func csvKeyIndexes(header []string, cfg engine.Config) ([]int, error) {
	var indexes []int
	byName := make(map[string]int, len(header))
	for i, name := range header {
		byName[name] = i
	}
	for _, name := range cfg.KeyNames {
		index, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("key column %q not found", name)
		}
		indexes = append(indexes, index)
	}
	for _, index := range cfg.KeyIndexes {
		indexes = append(indexes, index-cfg.IndexBase)
	}
	if len(indexes) == 0 && len(cfg.ExcludeKeyNames)+len(cfg.ExcludeKeyIndexes) > 0 {
		excluded := make(map[int]bool)
		for _, name := range cfg.ExcludeKeyNames {
			index, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("excluded key column %q not found", name)
			}
			excluded[index] = true
		}
		for _, index := range cfg.ExcludeKeyIndexes {
			excluded[index-cfg.IndexBase] = true
		}
		for index := range header {
			if !excluded[index] {
				indexes = append(indexes, index)
			}
		}
	}
	if len(indexes) == 0 {
		return nil, fmt.Errorf("three-way CSV requires explicit key or exclude-key columns")
	}
	for _, index := range indexes {
		if index < 0 || index >= len(header) {
			return nil, fmt.Errorf("key index %d is out of range", index+cfg.IndexBase)
		}
	}
	return indexes, nil
}

func csvKey(row []string, indexes []int) ([]string, string, error) {
	key := make([]string, len(indexes))
	for i, index := range indexes {
		if index >= len(row) {
			return nil, "", fmt.Errorf("row has too few columns")
		}
		key[i] = row[index]
	}
	data, _ := json.Marshal(key)
	return key, string(data), nil
}
func rowString(row []string) string { data, _ := json.Marshal(row); return string(data) }
func sortRows(rows [][]string) {
	sort.Slice(rows, func(i, j int) bool { return rowString(rows[i]) < rowString(rows[j]) })
}
func equalRows(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if rowString(a[i]) != rowString(b[i]) {
			return false
		}
	}
	return true
}
func cloneRows(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i := range rows {
		out[i] = append([]string(nil), rows[i]...)
	}
	return out
}
func maxRowMultiset(a, b [][]string) [][]string {
	counts := map[string]int{}
	values := map[string][]string{}
	for _, rows := range [][][]string{a, b} {
		local := map[string]int{}
		for _, row := range rows {
			key := rowString(row)
			local[key]++
			values[key] = row
		}
		for key, n := range local {
			if n > counts[key] {
				counts[key] = n
			}
		}
	}
	var out [][]string
	for key, n := range counts {
		for range n {
			out = append(out, append([]string(nil), values[key]...))
		}
	}
	return out
}

// WriteCSVMerge streams the base file, replacing only event key groups. CSV
// conflicts default to BASE only after an explicit allowUnresolved decision.
func WriteCSVMerge(basePath, output string, result CSVResult, choices map[string]string, allowUnresolved bool) (unresolved int, resultErr error) {
	selected := make(map[string][][]string, len(result.Events))
	events := make(map[string]CSVEvent, len(result.Events))
	removals := make(map[string]map[string]int, len(result.Events))
	for _, event := range result.Events {
		encoded, _ := json.Marshal(event.Key)
		key := string(encoded)
		events[key] = event
		removals[key] = make(map[string]int)
		for _, row := range event.Base {
			removals[key][rowString(row)]++
		}
		rows := event.Left
		switch event.Kind {
		case RightOnly:
			rows = event.Right
		case Same:
			rows = event.Left
		case Conflict:
			switch choices[event.ID] {
			case "left":
				rows = event.Left
			case "right":
				rows = event.Right
			case "base":
				rows = event.Base
			default:
				unresolved++
				if !allowUnresolved {
					return unresolved, fmt.Errorf("%d CSV conflicts are unresolved", unresolved)
				}
				rows = event.Base
			}
		}
		selected[key] = rows
	}
	input, closeInput, inputComma, err := openCSV(basePath, result.BaseDelimiter)
	if err != nil {
		return unresolved, err
	}
	defer closeInput()
	writeErr := atomicfile.Write(output, atomicfile.Options{Pattern: ".ayame-three-way-csv-*"}, func(temp io.Writer) (writeResultErr error) {
		var destination io.Writer = temp
		var gz *gzip.Writer
		gzOpen := false
		defer func() {
			if gzOpen {
				if err := gz.Close(); writeResultErr == nil && err != nil {
					writeResultErr = err
				}
			}
		}()
		if strings.HasSuffix(strings.ToLower(output), ".gz") {
			gz = gzip.NewWriter(temp)
			gzOpen = true
			destination = gz
		}
		buffer := bufio.NewWriterSize(destination, 256*1024)
		writer := csv.NewWriter(buffer)
		outputLower := strings.TrimSuffix(strings.ToLower(output), ".gz")
		writer.Comma = ','
		if strings.HasSuffix(outputLower, ".tsv") {
			writer.Comma = '\t'
		}
		reader := csv.NewReader(input)
		reader.Comma = inputComma
		reader.FieldsPerRecord = -1
		reader.ReuseRecord = true
		reader.LazyQuotes = result.LazyQuotes
		reader.TrimLeadingSpace = result.TrimLeadingSpace
		if result.HasHeader {
			if _, err := reader.Read(); err != nil {
				return err
			}
			if err := writer.Write(result.Header); err != nil {
				return err
			}
		}
		written := make(map[string]bool)
		for {
			row, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			_, encoded, err := csvKey(row, result.KeyIndexes)
			if err != nil {
				return err
			}
			if _, changed := events[encoded]; changed {
				if !written[encoded] {
					for _, replacement := range selected[encoded] {
						if err := writer.Write(replacement); err != nil {
							return err
						}
					}
					written[encoded] = true
				}
				rowKey := rowString(row)
				if removals[encoded][rowKey] > 0 {
					removals[encoded][rowKey]--
					continue
				}
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		remaining := make([]string, 0, len(events))
		for key := range events {
			if !written[key] {
				remaining = append(remaining, key)
			}
		}
		sort.Strings(remaining)
		for _, key := range remaining {
			if written[key] {
				continue
			}
			for _, row := range selected[key] {
				if err := writer.Write(row); err != nil {
					return err
				}
			}
			written[key] = true
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		if err := buffer.Flush(); err != nil {
			return err
		}
		if gz != nil {
			if err := gz.Close(); err != nil {
				return err
			}
			gzOpen = false
		}
		return nil
	})
	return unresolved, writeErr
}

func inputDelimiter(path, format, explicit string) rune {
	if explicit != "" {
		for _, value := range explicit {
			return value
		}
	}
	if strings.EqualFold(format, "tsv") {
		return '\t'
	}
	lower := strings.TrimSuffix(strings.ToLower(path), ".gz")
	if strings.HasSuffix(lower, ".tsv") {
		return '\t'
	}
	return ','
}

func openCSV(path string, comma rune) (io.Reader, func(), rune, error) {
	open := func() (*os.File, io.Reader, func(), error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, nil, func() {}, err
		}
		var reader io.Reader = file
		var gz *gzip.Reader
		if strings.HasSuffix(strings.ToLower(path), ".gz") {
			gz, err = gzip.NewReader(file)
			if err != nil {
				file.Close()
				return nil, nil, func() {}, err
			}
			reader = gz
		}
		closeFn := func() {
			if gz != nil {
				_ = gz.Close()
			}
			_ = file.Close()
		}
		return file, reader, closeFn, nil
	}
	_, sampleReader, closeSample, err := open()
	if err != nil {
		return nil, func() {}, comma, err
	}
	sample, _ := io.ReadAll(io.LimitReader(sampleReader, 8192))
	closeSample()
	enc := encoding.Detect(sample, encoding.Auto)
	_, reader, closeFn, err := open()
	if err != nil {
		return nil, func() {}, comma, err
	}
	decoded := bufio.NewReader(encoding.Decoder(reader, enc))
	if prefix, _ := decoded.Peek(3); len(prefix) == 3 && prefix[0] == 0xef && prefix[1] == 0xbb && prefix[2] == 0xbf {
		_, _ = decoded.Discard(3)
	}
	return decoded, closeFn, comma, nil
}
