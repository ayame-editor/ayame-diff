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

type fieldBytes interface {
	string | []byte
}

func encodeStringFields(input []string, mapping, keyIndexes []int, keyIsFullRow bool, keyDst, rowDst []byte) ([]byte, []byte, error) {
	return encodeFields(input, mapping, keyIndexes, keyIsFullRow, keyDst, rowDst)
}

func encodeByteFields(input [][]byte, mapping, keyIndexes []int, keyIsFullRow bool, keyDst, rowDst []byte) ([]byte, []byte, error) {
	return encodeFields(input, mapping, keyIndexes, keyIsFullRow, keyDst, rowDst)
}

// encodeFields is shared by the RFC string path and the zero-copy simple byte
// path. Both therefore produce byte-identical keys and rows by construction.
func encodeFields[T fieldBytes](input []T, mapping, keyIndexes []int, keyIsFullRow bool, keyDst, rowDst []byte) ([]byte, []byte, error) {
	keyDst = keyDst[:0]
	rowDst = rowDst[:0]
	if keyIsFullRow {
		for _, j := range mapping {
			if j < 0 || j >= len(input) {
				return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
			}
			var err error
			keyDst, err = appendLengthPrefixed(keyDst, input[j])
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
		rowDst, err = appendLengthPrefixed(rowDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	for _, i := range keyIndexes {
		j := mapping[i]
		var err error
		keyDst, err = appendLengthPrefixed(keyDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	return keyDst, rowDst, nil
}

func encodeComparedFields[T fieldBytes](input []T, mapping, keyIndexes []int, comparison comparisonConfig, keyDst, rowDst []byte) ([]byte, []byte, error) {
	keyDst, rowDst = keyDst[:0], rowDst[:0]
	for _, j := range mapping {
		if j < 0 || j >= len(input) {
			return nil, nil, fmt.Errorf("column mapping index %d outside record with %d columns", j, len(input))
		}
		var err error
		rowDst, err = appendLengthPrefixed(rowDst, input[j])
		if err != nil {
			return nil, nil, err
		}
	}
	for _, index := range keyIndexes {
		j := mapping[index]
		value := comparison.normalize(string(input[j]))
		var err error
		keyDst, err = appendLengthPrefixed(keyDst, value)
		if err != nil {
			return nil, nil, err
		}
	}
	return keyDst, rowDst, nil
}
func appendLengthPrefixed[T fieldBytes](dst []byte, v T) ([]byte, error) {
	if uint64(len(v)) > math.MaxUint32 {
		return nil, fmt.Errorf("field is larger than 4GiB")
	}
	start := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[start:start+4], uint32(len(v)))
	dst = append(dst, v...)
	return dst, nil
}

// Stored binary records use one stable wire layout:
//
//	[key length u32][row length u32][key bytes][row bytes]
const binHeaderSize = 8

func putBinHeader(dst []byte, keyLen, rowLen uint32) {
	binary.BigEndian.PutUint32(dst[:4], keyLen)
	binary.BigEndian.PutUint32(dst[4:binHeaderSize], rowLen)
}

func makeStoredRecord(dst, key, row []byte, max int64) ([]byte, error) {
	if uint64(len(key)) > math.MaxUint32 || uint64(len(row)) > math.MaxUint32 {
		return nil, fmt.Errorf("encoded key or row exceeds 4GiB")
	}
	total := int64(len(key) + len(row))
	if total > max {
		return nil, fmt.Errorf("encoded record is %d bytes, larger than configured maximum %d", total, max)
	}
	dst = dst[:0]
	dst = append(dst, make([]byte, binHeaderSize)...)
	putBinHeader(dst, uint32(len(key)), uint32(len(row)))
	dst = append(dst, key...)
	dst = append(dst, row...)
	return dst, nil
}
func writeBinRecord(w io.Writer, r binRecord) error {
	return (&binRecordWriter{w: w}).write(r)
}

type binRecordWriter struct {
	w      io.Writer
	header [binHeaderSize]byte
}

func (w *binRecordWriter) write(r binRecord) error {
	putBinHeader(w.header[:], uint32(len(r.Key)), uint32(len(r.Row)))
	if _, err := w.w.Write(w.header[:]); err != nil {
		return err
	}
	if _, err := w.w.Write(r.Key); err != nil {
		return err
	}
	_, err := w.w.Write(r.Row)
	return err
}

type binRecordReader struct {
	r      *bufio.Reader
	max    int64
	header [binHeaderSize]byte
}

type binRecordSpan struct {
	keyOffset, keyLen int
	rowOffset, rowLen int
}

func (s binRecordSpan) record(arena []byte) binRecord {
	return binRecord{
		Key: arena[s.keyOffset : s.keyOffset+s.keyLen],
		Row: arena[s.rowOffset : s.rowOffset+s.rowLen],
	}
}

func newBinRecordReader(r io.Reader, buf int, max int64) *binRecordReader {
	if buf < 4096 {
		buf = 4096
	}
	return &binRecordReader{r: bufio.NewReaderSize(r, buf), max: max}
}

func (r *binRecordReader) readLengths() (int, int, error) {
	n, err := io.ReadFull(r.r, r.header[:])
	if err != nil {
		if errors.Is(err, io.EOF) && n == 0 {
			return 0, 0, io.EOF
		}
		return 0, 0, fmt.Errorf("read record header: %w", err)
	}
	kl := int64(binary.BigEndian.Uint32(r.header[:4]))
	rl := int64(binary.BigEndian.Uint32(r.header[4:binHeaderSize]))
	if kl+rl > r.max {
		return 0, 0, fmt.Errorf("stored record is %d bytes, larger than maximum %d", kl+rl, r.max)
	}
	maxInt := int64(^uint(0) >> 1)
	if kl > maxInt || rl > maxInt || kl+rl > maxInt {
		return 0, 0, fmt.Errorf("stored record is too large for this platform")
	}
	return int(kl), int(rl), nil
}

func resizeBytes(dst []byte, length int) []byte {
	if cap(dst) < length {
		return make([]byte, length)
	}
	return dst[:length]
}

// NextInto reuses dst's key and row buffers. A reader therefore allocates only
// when a larger record is first encountered, not once per record.
func (r *binRecordReader) NextInto(dst *binRecord) error {
	kl, rl, err := r.readLengths()
	if err != nil {
		return err
	}
	dst.Key = resizeBytes(dst.Key, kl)
	dst.Row = resizeBytes(dst.Row, rl)
	if _, err := io.ReadFull(r.r, dst.Key); err != nil {
		return fmt.Errorf("read key: %w", err)
	}
	if _, err := io.ReadFull(r.r, dst.Row); err != nil {
		return fmt.Errorf("read row: %w", err)
	}
	return nil
}

// AppendToArena reads a record payload into one caller-owned chunk arena and
// returns stable offsets. The caller materializes slices after the arena stops
// growing, so reallocating it never leaves earlier records pointing at stale
// backing arrays.
func (r *binRecordReader) AppendToArena(arena []byte) ([]byte, binRecordSpan, error) {
	kl, rl, err := r.readLengths()
	if err != nil {
		return arena, binRecordSpan{}, err
	}
	keyOffset := len(arena)
	total := kl + rl
	arena = append(arena, make([]byte, total)...)
	rowOffset := keyOffset + kl
	if _, err := io.ReadFull(r.r, arena[keyOffset:rowOffset]); err != nil {
		return arena, binRecordSpan{}, fmt.Errorf("read key: %w", err)
	}
	if _, err := io.ReadFull(r.r, arena[rowOffset:rowOffset+rl]); err != nil {
		return arena, binRecordSpan{}, fmt.Errorf("read row: %w", err)
	}
	return arena, binRecordSpan{keyOffset: keyOffset, keyLen: kl, rowOffset: rowOffset, rowLen: rl}, nil
}

func (r *binRecordReader) Next() (binRecord, error) {
	var record binRecord
	if err := r.NextInto(&record); err != nil {
		return binRecord{}, err
	}
	return record, nil
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

// decodeRowBytes exposes zero-copy field boundaries over an encoded row. The
// returned slices alias encoded and are valid while that record buffer lives.
func decodeRowBytes(encoded []byte, n int, dst [][]byte) ([][]byte, error) {
	if cap(dst) < n {
		dst = make([][]byte, 0, n)
	} else {
		dst = dst[:0]
	}
	for pos := 0; pos < len(encoded); {
		if len(encoded)-pos < 4 {
			return nil, fmt.Errorf("corrupt row: truncated field length")
		}
		length := int(binary.BigEndian.Uint32(encoded[pos : pos+4]))
		pos += 4
		if len(encoded)-pos < length {
			return nil, fmt.Errorf("corrupt row: truncated field")
		}
		dst = append(dst, encoded[pos:pos+length])
		pos += length
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
