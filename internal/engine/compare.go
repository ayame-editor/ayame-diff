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
	record, err := c.reader.Next()
	if errors.Is(err, io.EOF) {
		c.record = binRecord{}
		c.eof = true
		return nil
	}
	if err != nil {
		return err
	}
	c.record = record
	c.eof = false
	return nil
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
	buffer := bufio.NewWriterSize(out, 4*1024*1024)
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

	var rowFields []string
	var operations uint64
	writeDiff := func(kind, side string, record binRecord) error {
		encoded := record.Row
		if keyIsFullRow {
			encoded = record.Key
		}
		var err error
		rowFields, err = decodeRow(encoded, columnCount, rowFields)
		if err != nil {
			return err
		}
		output := make([]string, 2+len(rowFields))
		output[0] = kind
		output[1] = side
		copy(output[2:], rowFields)
		if err := writer.Write(output); err != nil {
			return err
		}
		stats.DiffRows++
		return nil
	}

	flushLeftKey := func(kind string, key []byte) error {
		for !left.eof && bytes.Equal(left.record.Key, key) {
			if err := writeDiff(kind, "left", left.record); err != nil {
				return err
			}
			if kind == "LEFT_ONLY" {
				stats.LeftOnly++
			} else {
				stats.ChangedLeft++
			}
			if err := left.advance(); err != nil {
				return err
			}
		}
		return nil
	}
	flushRightKey := func(kind string, key []byte) error {
		for !right.eof && bytes.Equal(right.record.Key, key) {
			if err := writeDiff(kind, "right", right.record); err != nil {
				return err
			}
			if kind == "RIGHT_ONLY" {
				stats.RightOnly++
			} else {
				stats.ChangedRight++
			}
			if err := right.advance(); err != nil {
				return err
			}
		}
		return nil
	}

	for !left.eof || !right.eof {
		if operations&0x3fff == 0 {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
		}
		operations++

		if left.eof {
			key := append([]byte(nil), right.record.Key...)
			if err := flushRightKey("RIGHT_ONLY", key); err != nil {
				return stats, err
			}
			continue
		}
		if right.eof {
			key := append([]byte(nil), left.record.Key...)
			if err := flushLeftKey("LEFT_ONLY", key); err != nil {
				return stats, err
			}
			continue
		}

		switch keyCompare := bytes.Compare(left.record.Key, right.record.Key); {
		case keyCompare < 0:
			key := append([]byte(nil), left.record.Key...)
			if err := flushLeftKey("LEFT_ONLY", key); err != nil {
				return stats, err
			}
		case keyCompare > 0:
			key := append([]byte(nil), right.record.Key...)
			if err := flushRightKey("RIGHT_ONLY", key); err != nil {
				return stats, err
			}
		default:
			key := append([]byte(nil), left.record.Key...)
			for !left.eof && !right.eof && bytes.Equal(left.record.Key, key) && bytes.Equal(right.record.Key, key) {
				switch rowCompare := bytes.Compare(left.record.Row, right.record.Row); {
				case rowCompare < 0:
					if err := writeDiff("CHANGED", "left", left.record); err != nil {
						return stats, err
					}
					stats.ChangedLeft++
					if err := left.advance(); err != nil {
						return stats, err
					}
				case rowCompare > 0:
					if err := writeDiff("CHANGED", "right", right.record); err != nil {
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
			if err := flushLeftKey("CHANGED", key); err != nil {
				return stats, err
			}
			if err := flushRightKey("CHANGED", key); err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}
