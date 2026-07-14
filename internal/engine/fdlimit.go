package engine

// fileDescriptorNeed estimates the worst-case number of simultaneously open
// files for a run. Partitioning opens every partition sink at once (one side
// at a time); the compare phase opens up to MergeFanIn run files per worker.
// A fixed reserve covers inputs, outputs, progress, and the runtime.
func fileDescriptorNeed(cfg Config) uint64 {
	workers := minInt(cfg.Workers, cfg.Partitions)
	need := uint64(cfg.Partitions)
	if merge := uint64(workers) * uint64(cfg.MergeFanIn); merge > need {
		need = merge
	}
	return need + 64
}
