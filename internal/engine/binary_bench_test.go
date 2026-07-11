package engine

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func BenchmarkBinRecordReaderNextInto(b *testing.B) {
	const recordCount = 4096
	var encoded bytes.Buffer
	record := binRecord{Key: []byte("account-000001"), Row: bytes.Repeat([]byte("payload"), 8)}
	for range recordCount {
		if err := writeBinRecord(&encoded, record); err != nil {
			b.Fatal(err)
		}
	}
	data := encoded.Bytes()
	source := bytes.NewReader(data)
	reader := newBinRecordReader(source, 64*1024, 1024)
	var dst binRecord
	// Warm the reusable buffers, then rewind before allocation measurement.
	if err := reader.NextInto(&dst); err != nil {
		b.Fatal(err)
	}
	source.Reset(data)
	reader.r.Reset(source)
	b.ReportAllocs()
	b.SetBytes(int64(len(data) / recordCount))
	b.ResetTimer()
	for range b.N {
		err := reader.NextInto(&dst)
		if errors.Is(err, io.EOF) {
			source.Reset(data)
			reader.r.Reset(source)
			err = reader.NextInto(&dst)
		}
		if err != nil {
			b.Fatal(err)
		}
	}
}
