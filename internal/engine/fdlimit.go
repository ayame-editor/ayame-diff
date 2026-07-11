package engine

import "fmt"

const fdSafetyReserve = 16

// estimatedFileDescriptors returns a conservative upper bound for descriptors
// opened by the two descriptor-heavy phases. Partitioning keeps every output
// partition open; external merge sort opens up to MergeFanIn runs per worker.
func estimatedFileDescriptors(cfg resolvedConfig) uint64 {
	partitionPhase := cfg.Partitions + fdSafetyReserve
	workers := minInt(cfg.Workers, cfg.Partitions)
	mergePhase := workers*(cfg.MergeFanIn+4) + fdSafetyReserve
	return uint64(max(partitionPhase, mergePhase))
}

func ensureFileDescriptorBudget(cfg resolvedConfig) error {
	required := estimatedFileDescriptors(cfg)
	soft, hard, supported, err := openFileLimits()
	if err != nil || !supported || soft >= required {
		// Platforms without RLIMIT_NOFILE support retain the existing behavior.
		// A failed probe is also non-fatal: the eventual open error still carries
		// the OS cause, while rejecting a valid run here would be worse.
		return nil
	}
	return meetOpenFileLimit(required, soft, hard, setOpenFileSoftLimit)
}

func meetOpenFileLimit(required, soft, hard uint64, raise func(uint64) error) error {
	if soft >= required {
		return nil
	}
	if hard >= required {
		if err := raise(required); err == nil {
			return nil
		}
	}
	return fmt.Errorf(
		"open-file limit is too low: need about %d descriptors, soft limit is %d; raise ulimit -n or lower --partitions, --workers, or --merge-fan-in",
		required, soft,
	)
}
