package linediff

// MoveOptions bounds the optional post-processing pass. Detection is exact:
// hashes only select candidates and every paired block is compared line by line.
type MoveOptions struct {
	MinLines      uint64
	MaxCandidates int
}

type moveKey struct {
	lines uint64
	hash  uint64
}

// DetectMoves pairs stored Delete and Insert hunks whose contents are exactly
// equal. It returns the number of move pairs and annotates both hunks in res.
// Callers opt in explicitly, so the normal Diff path pays no hashing cost.
func DetectMoves(old, new Lines, res *Result, opts MoveOptions) uint64 {
	if res == nil || len(res.Hunks) == 0 || res.OmittedHunks != 0 {
		return 0
	}
	if opts.MinLines == 0 {
		opts.MinLines = 2
	}
	if opts.MaxCandidates <= 0 {
		opts.MaxCandidates = 10_000
	}
	// Make repeated calls idempotent before recomputing annotations/stats.
	res.Added += res.MovedLines
	res.Deleted += res.MovedLines
	res.MovedBlocks, res.MovedLines = 0, 0
	deletes := make(map[moveKey][]int)
	deleteCandidates := 0
	for i := range res.Hunks {
		h := &res.Hunks[i]
		h.MoveID, h.MovePeer = 0, 0
		if h.Kind != Delete || h.OldLen < opts.MinLines || deleteCandidates >= opts.MaxCandidates {
			continue
		}
		key := moveKey{lines: h.OldLen, hash: hashBlock(old, h.OldStart, h.OldLen)}
		deletes[key] = append(deletes[key], i)
		deleteCandidates++
	}
	var pairs uint64
	insertCandidates := 0
	for i := range res.Hunks {
		insert := &res.Hunks[i]
		if insert.Kind != Insert || insert.NewLen < opts.MinLines || insertCandidates >= opts.MaxCandidates {
			continue
		}
		insertCandidates++
		key := moveKey{lines: insert.NewLen, hash: hashBlock(new, insert.NewStart, insert.NewLen)}
		indexes := deletes[key]
		for len(indexes) > 0 {
			deleteIndex := indexes[0]
			indexes = indexes[1:]
			deleted := &res.Hunks[deleteIndex]
			if deleted.MoveID != 0 || !blocksEqual(old, deleted.OldStart, new, insert.NewStart, insert.NewLen) {
				continue
			}
			pairs++
			deleted.MoveID, insert.MoveID = pairs, pairs
			deleted.MovePeer, insert.MovePeer = insert.NewStart, deleted.OldStart
			if res.Deleted >= deleted.OldLen {
				res.Deleted -= deleted.OldLen
			}
			if res.Added >= insert.NewLen {
				res.Added -= insert.NewLen
			}
			res.MovedBlocks++
			res.MovedLines += insert.NewLen
			break
		}
		deletes[key] = indexes
	}
	return pairs
}

func hashBlock(lines Lines, start, count uint64) uint64 {
	const (
		offset = uint64(14695981039346656037)
		prime  = uint64(1099511628211)
	)
	hash := offset
	for i := start; i < start+count; i++ {
		line, _ := lines.Line(i)
		for j := 0; j < len(line); j++ {
			hash ^= uint64(line[j])
			hash *= prime
		}
		// Include an unambiguous record boundary and length.
		hash ^= uint64(len(line))
		hash *= prime
	}
	return hash
}

func blocksEqual(left Lines, leftStart uint64, right Lines, rightStart, count uint64) bool {
	for offset := uint64(0); offset < count; offset++ {
		l, lok := left.Line(leftStart + offset)
		r, rok := right.Line(rightStart + offset)
		if !lok || !rok || l != r {
			return false
		}
	}
	return true
}
