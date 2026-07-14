//go:build linux || darwin

package engine

import (
	"fmt"
	"syscall"
)

// ensureFileDescriptorBudget fails fast when the run could exhaust the
// process file-descriptor limit mid-flight (which would otherwise surface
// as an opaque "too many open files" after significant work). It first
// tries to raise the soft limit toward the hard limit.
func ensureFileDescriptorBudget(cfg Config) error {
	need := fileDescriptorNeed(cfg)
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return nil // cannot determine the limit; do not block the run
	}
	if limit.Cur >= need {
		return nil
	}
	if limit.Max >= need {
		raised := limit
		raised.Cur = need
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &raised); err == nil {
			return nil
		}
	}
	return fmt.Errorf("this run may need up to %d open files but the current limit is %d; lower --partitions or --merge-fan-in/--workers, or raise the limit (e.g. ulimit -n %d)", need, limit.Cur, need)
}
