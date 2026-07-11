package engine

import (
	"strconv"
	"testing"
)

func BenchmarkChangedEncodedColumns1000(b *testing.B) {
	fields := make([]string, 1000)
	for i := range fields {
		fields[i] = strconv.Itoa(i)
	}
	left, _, err := encodeStringFields(fields, identityMap(len(fields)), nil, true, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	rightFields := append([]string(nil), fields...)
	rightFields[501] = "changed"
	right, _, err := encodeStringFields(rightFields, identityMap(len(fields)), nil, true, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var scratch cellDiffScratch
	for i := 0; i < b.N; i++ {
		changed, err := scratch.indexes(left, right, len(fields))
		if err != nil || len(changed) != 1 {
			b.Fatal(err)
		}
	}
}
