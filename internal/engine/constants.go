package engine

const (
	// ioBufferBytes amortizes syscalls while keeping one buffer modest relative
	// to the per-worker memory floor.
	ioBufferBytes = 4 * 1024 * 1024
	// cancellationCheckMask checks context every 16,384 records in tight loops.
	cancellationCheckMask = 0x3fff
	// minSortChunkBytes prevents tiny runs when memory is split across workers.
	minSortChunkBytes = 8 * 1024 * 1024
	// minParallelSpanBytes avoids extra file readers for very small ranges.
	minParallelSpanBytes = 8 * 1024 * 1024
	// recordMemoryOverhead estimates slice and sort bookkeeping per record.
	recordMemoryOverhead = 64
)
