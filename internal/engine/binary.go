package engine

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
)

type binRecord struct{ Key, Row []byte }

func encodeStringFields(input []string, mapping, keyIndexes []int, keyIsFullRow bool, keyDst, rowDst []byte) ([]byte, []byte, error) {
	keyDst = keyDst[:0]
	rowDst = rowDst[:0]
	if keyIsFullRow {
		for _, j := range mapping {
			if j < 0 || j >= len(input) {
				return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
			}
			var err error
			keyDst, err = appendLengthPrefixedString(keyDst, input[j])
			if err != nil {
				return nil, nil, err
			}
		}
		return keyDst, rowDst, nil
	}
	for _, j := range mapping {
		if j < 0 || j >= len(input) {
			return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
		}
		var err error
		rowDst, err = appendLengthPrefixedString(rowDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	for _, i := range keyIndexes {
		j := mapping[i]
		var err error
		keyDst, err = appendLengthPrefixedString(keyDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	return keyDst, rowDst, nil
}
func encodeByteFields(input [][]byte, mapping, keyIndexes []int, keyIsFullRow bool, keyDst, rowDst []byte) ([]byte, []byte, error) {
	keyDst = keyDst[:0]
	rowDst = rowDst[:0]
	if keyIsFullRow {
		for _, j := range mapping {
			if j < 0 || j >= len(input) {
				return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
			}
			var err error
			keyDst, err = appendLengthPrefixedBytes(keyDst, input[j])
			if err != nil {
				return nil, nil, err
			}
		}
		return keyDst, rowDst, nil
	}
	for _, j := range mapping {
		if j < 0 || j >= len(input) {
			return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
		}
		var err error
		rowDst, err = appendLengthPrefixedBytes(rowDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	for _, i := range keyIndexes {
		j := mapping[i]
		var err error
		keyDst, err = appendLengthPrefixedBytes(keyDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	return keyDst, rowDst, nil
}
func appendLengthPrefixedString(dst []byte, v string) ([]byte, error) {
	if len(v) > math.MaxUint32 {
		return nil, fmt.Errorf("field is larger than 4GiB")
	}
	start := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[start:start+4], uint32(len(v)))
	dst = append(dst, v...)
	return dst, nil
}
func appendLengthPrefixedBytes(dst, v []byte) ([]byte, error) {
	if len(v) > math.MaxUint32 {
		return nil, fmt.Errorf("field is larger than 4GiB")
	}
	start := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[start:start+4], uint32(len(v)))
	dst = append(dst, v...)
	return dst, nil
}
func makeStoredRecord(dst, key, row []byte, max int64) ([]byte, error) {
	if len(key) > math.MaxUint32 || len(row) > math.MaxUint32 {
		return nil, fmt.Errorf("encoded key or row exceeds 4GiB")
	}
	total := int64(len(key) + len(row))
	if total > max {
		return nil, fmt.Errorf("encoded record is %d bytes, larger than configured maximum %d", total, max)
	}
	dst = dst[:0]
	dst = append(dst, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[:4], uint32(len(key)))
	binary.BigEndian.PutUint32(dst[4:8], uint32(len(row)))
	dst = append(dst, key...)
	dst = append(dst, row...)
	return dst, nil
}
func writeBinRecord(w io.Writer, r binRecord) error {
	var h [8]byte
	binary.BigEndian.PutUint32(h[:4], uint32(len(r.Key)))
	binary.BigEndian.PutUint32(h[4:], uint32(len(r.Row)))
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	if _, err := w.Write(r.Key); err != nil {
		return err
	}
	_, err := w.Write(r.Row)
	return err
}

type binRecordReader struct {
	r   *bufio.Reader
	max int64
}

func newBinRecordReader(r io.Reader, buf int, max int64) *binRecordReader {
	if buf < 4096 {
		buf = 4096
	}
	return &binRecordReader{bufio.NewReaderSize(r, buf), max}
}
func (r *binRecordReader) Next() (binRecord, error) {
	var h [8]byte
	n, err := io.ReadFull(r.r, h[:])
	if err != nil {
		if errors.Is(err, io.EOF) && n == 0 {
			return binRecord{}, io.EOF
		}
		return binRecord{}, fmt.Errorf("read record header: %w", err)
	}
	kl := int64(binary.BigEndian.Uint32(h[:4]))
	rl := int64(binary.BigEndian.Uint32(h[4:]))
	if kl+rl > r.max {
		return binRecord{}, fmt.Errorf("stored record is %d bytes, larger than maximum %d", kl+rl, r.max)
	}
	key := make([]byte, int(kl))
	row := make([]byte, int(rl))
	if _, err := io.ReadFull(r.r, key); err != nil {
		return binRecord{}, fmt.Errorf("read key: %w", err)
	}
	if _, err := io.ReadFull(r.r, row); err != nil {
		return binRecord{}, fmt.Errorf("read row: %w", err)
	}
	return binRecord{key, row}, nil
}
func decodeRow(encoded []byte, n int, dst []string) ([]string, error) {
	if cap(dst) < n {
		dst = make([]string, 0, n)
	} else {
		dst = dst[:0]
	}
	pos := 0
	for pos < len(encoded) {
		if len(encoded)-pos < 4 {
			return nil, fmt.Errorf("corrupt row: truncated field length")
		}
		l := int(binary.BigEndian.Uint32(encoded[pos : pos+4]))
		pos += 4
		if len(encoded)-pos < l {
			return nil, fmt.Errorf("corrupt row: truncated field")
		}
		dst = append(dst, string(encoded[pos:pos+l]))
		pos += l
	}
	if len(dst) != n {
		return nil, fmt.Errorf("corrupt row: expected %d columns, found %d", n, len(dst))
	}
	return dst, nil
}

func xxhash64(input []byte) uint64 {
	const (
		p1 = uint64(11400714785074694791)
		p2 = uint64(14029467366897019727)
		p3 = uint64(1609587929392839161)
		p4 = uint64(9650029242287828579)
		p5 = uint64(2870177450012600261)
	)
	i := 0
	var h uint64
	if len(input) >= 32 {
		// XXH64's four-lane initialization, written in the reference form so
		// future seeded variants cannot accidentally initialize only some lanes.
		seed := uint64(0)
		v1 := seed + p1 + p2
		v2 := seed + p2
		v3 := seed
		v4 := seed - p1
		limit := len(input) - 32
		for i <= limit {
			v1 = xxRound(v1, binary.LittleEndian.Uint64(input[i:i+8]))
			i += 8
			v2 = xxRound(v2, binary.LittleEndian.Uint64(input[i:i+8]))
			i += 8
			v3 = xxRound(v3, binary.LittleEndian.Uint64(input[i:i+8]))
			i += 8
			v4 = xxRound(v4, binary.LittleEndian.Uint64(input[i:i+8]))
			i += 8
		}
		h = bits.RotateLeft64(v1, 1) + bits.RotateLeft64(v2, 7) + bits.RotateLeft64(v3, 12) + bits.RotateLeft64(v4, 18)
		h = xxMergeRound(h, v1)
		h = xxMergeRound(h, v2)
		h = xxMergeRound(h, v3)
		h = xxMergeRound(h, v4)
	} else {
		h = p5
	}
	h += uint64(len(input))
	for i+8 <= len(input) {
		lane := xxRound(0, binary.LittleEndian.Uint64(input[i:i+8]))
		h ^= lane
		h = bits.RotateLeft64(h, 27)*p1 + p4
		i += 8
	}
	if i+4 <= len(input) {
		h ^= uint64(binary.LittleEndian.Uint32(input[i:i+4])) * p1
		h = bits.RotateLeft64(h, 23)*p2 + p3
		i += 4
	}
	for i < len(input) {
		h ^= uint64(input[i]) * p5
		h = bits.RotateLeft64(h, 11) * p1
		i++
	}
	h ^= h >> 33
	h *= p2
	h ^= h >> 29
	h *= p3
	h ^= h >> 32
	return h
}
func xxRound(a, l uint64) uint64 {
	const (
		p1 = uint64(11400714785074694791)
		p2 = uint64(14029467366897019727)
	)
	a += l * p2
	a = bits.RotateLeft64(a, 31)
	return a * p1
}
func xxMergeRound(a, v uint64) uint64 {
	const (
		p1 = uint64(11400714785074694791)
		p4 = uint64(9650029242287828579)
	)
	a ^= xxRound(0, v)
	return a*p1 + p4
}
