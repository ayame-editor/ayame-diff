package engine

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

type partitionStats struct {
	EqualRows      uint64
	LeftOnly       uint64
	RightOnly      uint64
	ChangedLeft    uint64
	ChangedRight   uint64
	DiffRows       uint64
	UnresolvedRows uint64
	ColumnChanges  []uint64
}

type diffKind string
type diffSide string

type jsonCellChange struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type jsonRecordDiff struct {
	ID             string           `json:"id"`
	Kind           diffKind         `json:"kind"`
	Old            []string         `json:"old,omitempty"`
	New            []string         `json:"new,omitempty"`
	ChangedColumns []jsonCellChange `json:"changed_columns,omitempty"`
}

type reconcileConfig struct {
	enabled         bool
	choices         map[string]string
	defaultTo       string
	delimiter       rune
	allowUnresolved bool
}

func (r reconcileConfig) choice(id string) (string, bool) {
	if side := r.choices[id]; side != "" {
		return side, false
	}
	if r.defaultTo != "" {
		return r.defaultTo, false
	}
	return "left", true
}

func recordDiffID(kind diffKind, oldRecord, newRecord *binRecord) string {
	h := sha256.New()
	h.Write([]byte(kind))
	if oldRecord != nil {
		h.Write([]byte{0})
		h.Write(oldRecord.Key)
		h.Write([]byte{0})
		h.Write(oldRecord.Row)
	}
	if newRecord != nil {
		h.Write([]byte{1})
		h.Write(newRecord.Key)
		h.Write([]byte{0})
		h.Write(newRecord.Row)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

const (
	diffLeftOnly  diffKind = "LEFT_ONLY"
	diffRightOnly diffKind = "RIGHT_ONLY"
	diffChanged   diffKind = "CHANGED"
	diffLeft      diffSide = "left"
	diffRight     diffSide = "right"
)

func (s *partitionStats) add(other partitionStats) {
	s.EqualRows += other.EqualRows
	s.LeftOnly += other.LeftOnly
	s.RightOnly += other.RightOnly
	s.ChangedLeft += other.ChangedLeft
	s.ChangedRight += other.ChangedRight
	s.DiffRows += other.DiffRows
	s.UnresolvedRows += other.UnresolvedRows
	if len(s.ColumnChanges) < len(other.ColumnChanges) {
		s.ColumnChanges = append(s.ColumnChanges, make([]uint64, len(other.ColumnChanges)-len(s.ColumnChanges))...)
	}
	for i, count := range other.ColumnChanges {
		s.ColumnChanges[i] += count
	}
}

type sortedCursor struct {
	file   *os.File
	reader *binRecordReader
	record binRecord
	keyBuf []byte
	eof    bool
}

func openSortedCursor(path string, maxRecordBytes int64) (*sortedCursor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	c := &sortedCursor{file: f, reader: newBinRecordReader(f, 2*1024*1024, maxRecordBytes)}
	if err := c.advance(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return c, nil
}

func (c *sortedCursor) advance() error {
	err := c.reader.NextInto(&c.record)
	if errors.Is(err, io.EOF) {
		c.record.Key = c.record.Key[:0]
		c.record.Row = c.record.Row[:0]
		c.eof = true
		return nil
	}
	if err != nil {
		return err
	}
	c.eof = false
	return nil
}

// captureKey owns one stable group key while advance reuses record buffers.
// Its capacity is retained across groups, so this allocates only when a larger
// key is first seen rather than once for every group.
func (c *sortedCursor) captureKey() []byte {
	c.keyBuf = append(c.keyBuf[:0], c.record.Key...)
	return c.keyBuf
}

func (c *sortedCursor) close() error { return c.file.Close() }

func compareSortedFiles(ctx context.Context, leftPath, rightPath, outputPath string, header []string, keyIsFullRow bool, maxRecordBytes int64, comparison comparisonConfig, cellDiff bool, outputFormat string, reconcile reconcileConfig) (stats partitionStats, resultErr error) {
	columnCount := len(header)
	left, err := openSortedCursor(leftPath, maxRecordBytes)
	if err != nil {
		return stats, err
	}
	defer func() {
		if err := left.close(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	right, err := openSortedCursor(rightPath, maxRecordBytes)
	if err != nil {
		return stats, err
	}
	defer func() {
		if err := right.close(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return stats, err
	}
	buffer := bufio.NewWriterSize(out, ioBufferBytes)
	writer := csv.NewWriter(buffer)
	writer.Comma = '\t'
	if reconcile.enabled {
		writer.Comma = reconcile.delimiter
	}
	jsonWriter := json.NewEncoder(buffer)
	defer func() {
		if outputFormat == "tsv" {
			writer.Flush()
			if err := writer.Error(); resultErr == nil && err != nil {
				resultErr = err
			}
		}
		if err := buffer.Flush(); resultErr == nil && err != nil {
			resultErr = err
		}
		if err := out.Close(); resultErr == nil && err != nil {
			resultErr = err
		}
		if resultErr != nil {
			_ = os.Remove(outputPath)
		}
	}()

	var rowFields, output []string
	var operations uint64
	decodeRecord := func(record binRecord) ([]string, error) {
		encoded := record.Row
		if keyIsFullRow {
			encoded = record.Key
		}
		return decodeRow(encoded, columnCount, nil)
	}
	writeEqual := func(record binRecord) error {
		if !reconcile.enabled {
			return nil
		}
		fields, err := decodeRecord(record)
		if err != nil {
			return err
		}
		return writer.Write(fields)
	}
	changedNames := func(indexes []int) string {
		names := make([]string, len(indexes))
		for i, index := range indexes {
			names[i] = header[index]
		}
		return strings.Join(names, ",")
	}
	writeDiff := func(kind diffKind, side diffSide, record binRecord, changed []int) error {
		var err error
		rowFields, err = decodeRecord(record)
		if err != nil {
			return err
		}
		var id string
		if side == diffLeft {
			id = recordDiffID(kind, &record, nil)
		} else {
			id = recordDiffID(kind, nil, &record)
		}
		if reconcile.enabled {
			choice, unresolved := reconcile.choice(id)
			if unresolved {
				if !reconcile.allowUnresolved {
					return errors.New("CSV merge has unresolved rows")
				}
				stats.UnresolvedRows++
			}
			if (side == diffLeft && choice == "left") || (side == diffRight && choice == "right") {
				if err := writer.Write(rowFields); err != nil {
					return err
				}
			}
		} else if outputFormat == "jsonl" {
			item := jsonRecordDiff{ID: id, Kind: kind}
			if side == diffLeft {
				item.Old = rowFields
			} else {
				item.New = rowFields
			}
			if err := jsonWriter.Encode(item); err != nil {
				return err
			}
		} else {
			extra := 0
			if cellDiff {
				extra = 1
			}
			if cap(output) < 2+extra+len(rowFields) {
				output = make([]string, 2+extra+len(rowFields))
			} else {
				output = output[:2+extra+len(rowFields)]
			}
			output[0], output[1] = string(kind), string(side)
			if cellDiff {
				output[2] = changedNames(changed)
			}
			copy(output[2+extra:], rowFields)
			if err := writer.Write(output); err != nil {
				return err
			}
		}
		stats.DiffRows++
		return nil
	}
	writePair := func(leftRecord, rightRecord binRecord, changed []int) error {
		if reconcile.enabled {
			id := recordDiffID(diffChanged, &leftRecord, &rightRecord)
			choice, unresolved := reconcile.choice(id)
			if unresolved {
				if !reconcile.allowUnresolved {
					return errors.New("CSV merge has unresolved rows")
				}
				stats.UnresolvedRows++
			}
			record := leftRecord
			if choice == "right" {
				record = rightRecord
			}
			fields, err := decodeRecord(record)
			if err != nil {
				return err
			}
			if err := writer.Write(fields); err != nil {
				return err
			}
			stats.DiffRows += 2
			return nil
		}
		if outputFormat == "tsv" {
			if err := writeDiff(diffChanged, diffLeft, leftRecord, changed); err != nil {
				return err
			}
			return writeDiff(diffChanged, diffRight, rightRecord, changed)
		}
		oldFields, err := decodeRecord(leftRecord)
		if err != nil {
			return err
		}
		newFields, err := decodeRecord(rightRecord)
		if err != nil {
			return err
		}
		item := jsonRecordDiff{ID: recordDiffID(diffChanged, &leftRecord, &rightRecord), Kind: diffChanged, Old: oldFields, New: newFields}
		for _, index := range changed {
			item.ChangedColumns = append(item.ChangedColumns, jsonCellChange{Index: index, Name: header[index], Old: oldFields[index], New: newFields[index]})
		}
		if err := jsonWriter.Encode(item); err != nil {
			return err
		}
		stats.DiffRows += 2
		return nil
	}

	flushKey := func(cursor *sortedCursor, kind diffKind, side diffSide, key []byte) error {
		for !cursor.eof && bytes.Equal(cursor.record.Key, key) {
			if err := writeDiff(kind, side, cursor.record, nil); err != nil {
				return err
			}
			switch {
			case kind == diffLeftOnly:
				stats.LeftOnly++
			case kind == diffRightOnly:
				stats.RightOnly++
			case side == diffLeft:
				stats.ChangedLeft++
			case side == diffRight:
				stats.ChangedRight++
			}
			if err := cursor.advance(); err != nil {
				return err
			}
		}
		return nil
	}

	type comparedRecord struct {
		record     binRecord
		comparison preparedComparison
	}
	var cellScratch cellDiffScratch
	readGroup := func(cursor *sortedCursor, key []byte) ([]comparedRecord, error) {
		var group []comparedRecord
		for !cursor.eof && bytes.Equal(cursor.record.Key, key) {
			record := binRecord{
				Key: append([]byte(nil), cursor.record.Key...),
				Row: append([]byte(nil), cursor.record.Row...),
			}
			prepared := preparedComparison{signature: string(record.Row)}
			if comparison.enabled {
				fields, err := decodeRow(record.Row, columnCount, nil)
				if err != nil {
					return nil, err
				}
				prepared = comparison.prepare(fields)
			}
			group = append(group, comparedRecord{record: record, comparison: prepared})
			if err := cursor.advance(); err != nil {
				return nil, err
			}
		}
		return group, nil
	}
	changedColumns := func(leftRecord, rightRecord comparedRecord) ([]int, error) {
		if comparison.enabled {
			return comparison.changedIndexesPrepared(leftRecord.comparison, rightRecord.comparison), nil
		}
		return cellScratch.indexes(leftRecord.record.Row, rightRecord.record.Row, columnCount)
	}
	compareGroup := func(key []byte) error {
		leftGroup, err := readGroup(left, key)
		if err != nil {
			return err
		}
		rightGroup, err := readGroup(right, key)
		if err != nil {
			return err
		}
		matchRight, matchLeft := make([]int, len(rightGroup)), make([]int, len(leftGroup))
		for i := range matchRight {
			matchRight[i] = -1
		}
		for i := range matchLeft {
			matchLeft[i] = -1
		}

		if !comparison.hasTolerance() {
			// Exact normalized equivalence is transitive, so groups cancel in O(n).
			exactRight := make(map[string][]int, len(rightGroup))
			for i, record := range rightGroup {
				exactRight[record.comparison.signature] = append(exactRight[record.comparison.signature], i)
			}
			for i, record := range leftGroup {
				indexes := exactRight[record.comparison.signature]
				if len(indexes) == 0 {
					continue
				}
				rightIndex := indexes[len(indexes)-1]
				exactRight[record.comparison.signature] = indexes[:len(indexes)-1]
				matchLeft[i], matchRight[rightIndex] = rightIndex, i
				stats.EqualRows++
			}
		} else {
			// Tolerance equivalence is not transitive. Find a maximum bipartite
			// matching using iterative augmenting paths; pre-fixing exact pairs
			// could otherwise reduce the maximum fuzzy matching.
			for root := range leftGroup {
				if matchLeft[root] >= 0 {
					continue
				}
				if root&int(cancellationCheckMask) == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				seenLeft, seenRight := make([]bool, len(leftGroup)), make([]bool, len(rightGroup))
				fromLeft := make([]int, len(rightGroup))
				queue, freeRight := []int{root}, -1
				seenLeft[root] = true
				for len(queue) > 0 && freeRight < 0 {
					leftIndex := queue[0]
					queue = queue[1:]
					for rightIndex, rightRecord := range rightGroup {
						if seenRight[rightIndex] || !comparison.equivalentPrepared(leftGroup[leftIndex].comparison, rightRecord.comparison) {
							continue
						}
						seenRight[rightIndex], fromLeft[rightIndex] = true, leftIndex
						if matchRight[rightIndex] < 0 {
							freeRight = rightIndex
							break
						}
						nextLeft := matchRight[rightIndex]
						if !seenLeft[nextLeft] {
							seenLeft[nextLeft], queue = true, append(queue, nextLeft)
						}
					}
				}
				if freeRight < 0 {
					continue
				}
				for rightIndex := freeRight; rightIndex >= 0; {
					leftIndex := fromLeft[rightIndex]
					previousRight := matchLeft[leftIndex]
					matchLeft[leftIndex], matchRight[rightIndex] = rightIndex, leftIndex
					rightIndex = previousRight
				}
				stats.EqualRows++
			}
		}
		unmatchedLeft, unmatchedRight := make([]int, 0), make([]int, 0)
		for i := range leftGroup {
			if matchLeft[i] >= 0 {
				if err := writeEqual(leftGroup[i].record); err != nil {
					return err
				}
			} else {
				unmatchedLeft = append(unmatchedLeft, i)
			}
		}
		for i := range rightGroup {
			if matchRight[i] < 0 {
				unmatchedRight = append(unmatchedRight, i)
			}
		}
		if cellDiff {
			pairs := min(len(unmatchedLeft), len(unmatchedRight))
			for i := 0; i < pairs; i++ {
				leftRecord, rightRecord := leftGroup[unmatchedLeft[i]], rightGroup[unmatchedRight[i]]
				changed, err := changedColumns(leftRecord, rightRecord)
				if err != nil {
					return err
				}
				if len(stats.ColumnChanges) == 0 {
					stats.ColumnChanges = make([]uint64, columnCount)
				}
				for _, index := range changed {
					stats.ColumnChanges[index]++
				}
				if err := writePair(leftRecord.record, rightRecord.record, changed); err != nil {
					return err
				}
				stats.ChangedLeft++
				stats.ChangedRight++
			}
			unmatchedLeft, unmatchedRight = unmatchedLeft[pairs:], unmatchedRight[pairs:]
		}
		for _, index := range unmatchedLeft {
			if err := writeDiff(diffChanged, diffLeft, leftGroup[index].record, nil); err != nil {
				return err
			}
			stats.ChangedLeft++
		}
		for _, index := range unmatchedRight {
			if err := writeDiff(diffChanged, diffRight, rightGroup[index].record, nil); err != nil {
				return err
			}
			stats.ChangedRight++
		}
		return nil
	}

	for !left.eof || !right.eof {
		if operations&cancellationCheckMask == 0 {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
		}
		operations++

		if left.eof {
			key := right.captureKey()
			if err := flushKey(right, diffRightOnly, diffRight, key); err != nil {
				return stats, err
			}
			continue
		}
		if right.eof {
			key := left.captureKey()
			if err := flushKey(left, diffLeftOnly, diffLeft, key); err != nil {
				return stats, err
			}
			continue
		}

		switch keyCompare := bytes.Compare(left.record.Key, right.record.Key); {
		case keyCompare < 0:
			key := left.captureKey()
			if err := flushKey(left, diffLeftOnly, diffLeft, key); err != nil {
				return stats, err
			}
		case keyCompare > 0:
			key := right.captureKey()
			if err := flushKey(right, diffRightOnly, diffRight, key); err != nil {
				return stats, err
			}
		default:
			key := left.captureKey()
			if comparison.enabled || cellDiff {
				if err := compareGroup(key); err != nil {
					return stats, err
				}
				continue
			}
			for !left.eof && !right.eof && bytes.Equal(left.record.Key, key) && bytes.Equal(right.record.Key, key) {
				switch rowCompare := bytes.Compare(left.record.Row, right.record.Row); {
				case rowCompare < 0:
					if err := writeDiff(diffChanged, diffLeft, left.record, nil); err != nil {
						return stats, err
					}
					stats.ChangedLeft++
					if err := left.advance(); err != nil {
						return stats, err
					}
				case rowCompare > 0:
					if err := writeDiff(diffChanged, diffRight, right.record, nil); err != nil {
						return stats, err
					}
					stats.ChangedRight++
					if err := right.advance(); err != nil {
						return stats, err
					}
				default:
					stats.EqualRows++
					if err := writeEqual(left.record); err != nil {
						return stats, err
					}
					if err := left.advance(); err != nil {
						return stats, err
					}
					if err := right.advance(); err != nil {
						return stats, err
					}
				}
			}
			if err := flushKey(left, diffChanged, diffLeft, key); err != nil {
				return stats, err
			}
			if err := flushKey(right, diffChanged, diffRight, key); err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}
