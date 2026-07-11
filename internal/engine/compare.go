package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
)

type partitionStats struct {
	EqualRows    uint64
	LeftOnly     uint64
	RightOnly    uint64
	ChangedLeft  uint64
	ChangedRight uint64
	DiffRows     uint64
}

type diffKind string
type diffSide string

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

func compareSortedFiles(ctx context.Context, leftPath, rightPath, outputPath string, columnCount int, keyIsFullRow bool, maxRecordBytes int64) (stats partitionStats, resultErr error) {
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
	defer func() {
		writer.Flush()
		if err := writer.Error(); resultErr == nil && err != nil {
			resultErr = err
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
	writeDiff := func(kind diffKind, side diffSide, record binRecord) error {
		encoded := record.Row
		if keyIsFullRow {
			encoded = record.Key
		}
		var err error
		rowFields, err = decodeRow(encoded, columnCount, rowFields)
		if err != nil {
			return err
		}
		if cap(output) < 2+len(rowFields) {
			output = make([]string, 2+len(rowFields))
		} else {
			output = output[:2+len(rowFields)]
		}
		output[0] = string(kind)
		output[1] = string(side)
		copy(output[2:], rowFields)
		if err := writer.Write(output); err != nil {
			return err
		}
		stats.DiffRows++
		return nil
	}

	flushKey := func(cursor *sortedCursor, kind diffKind, side diffSide, key []byte) error {
		for !cursor.eof && bytes.Equal(cursor.record.Key, key) {
			if err := writeDiff(kind, side, cursor.record); err != nil {
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
			for !left.eof && !right.eof && bytes.Equal(left.record.Key, key) && bytes.Equal(right.record.Key, key) {
				switch rowCompare := bytes.Compare(left.record.Row, right.record.Row); {
				case rowCompare < 0:
					if err := writeDiff(diffChanged, diffLeft, left.record); err != nil {
						return stats, err
					}
					stats.ChangedLeft++
					if err := left.advance(); err != nil {
						return stats, err
					}
				case rowCompare > 0:
					if err := writeDiff(diffChanged, diffRight, right.record); err != nil {
						return stats, err
					}
					stats.ChangedRight++
					if err := right.advance(); err != nil {
						return stats, err
					}
				default:
					stats.EqualRows++
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
