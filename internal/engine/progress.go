package engine

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/panicguard"
)

// ProgressEvent is a transport-neutral snapshot suitable for CLI logging,
// GUI polling, or SSE delivery.
type ProgressEvent struct {
	Phase         string        `json:"phase"`
	Label         string        `json:"label,omitempty"`
	Done          bool          `json:"done"`
	Rows          uint64        `json:"rows,omitempty"`
	Bytes         uint64        `json:"bytes,omitempty"`
	Elapsed       time.Duration `json:"elapsed"`
	RowsPerSecond float64       `json:"rows_per_second,omitempty"`
	MiBPerSecond  float64       `json:"mib_per_second,omitempty"`
}

type progressCounter struct {
	phase, label string
	enabled      bool
	log          io.Writer
	onProgress   func(ProgressEvent)
	rows, bytes  atomic.Uint64
	start        time.Time
	cancel       context.CancelFunc
	done         chan struct{}
	once         sync.Once
}

func startProgress(parent context.Context, phase, label string, enabled bool, log io.Writer, onProgress func(ProgressEvent)) *progressCounter {
	p := &progressCounter{phase: phase, label: label, enabled: enabled, log: log, onProgress: onProgress, start: time.Now()}
	if !enabled {
		return p
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		// Progress reporting is cosmetic: a panic while formatting a tick must
		// not kill a comparison that is otherwise succeeding (#137). Dropping
		// the ticker leaves stop() to print the final line.
		_ = panicguard.Call(func() { p.tick(ctx) })
	}()
	return p
}

// tick prints a progress line every interval until ctx is cancelled.
func (p *progressCounter) tick(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.print(false)
		}
	}
}

func (p *progressCounter) add(rows, bytes uint64) { p.rows.Add(rows); p.bytes.Add(bytes) }
func (p *progressCounter) stop() {
	p.once.Do(func() {
		if p.enabled {
			p.cancel()
			<-p.done
			p.print(true)
		}
	})
}
func (p *progressCounter) print(final bool) {
	duration := time.Since(p.start)
	elapsedSeconds := duration.Seconds()
	if elapsedSeconds <= 0 {
		elapsedSeconds = .001
	}
	rows := p.rows.Load()
	b := p.bytes.Load()
	event := ProgressEvent{
		Phase: p.phase, Label: p.label, Done: final, Rows: rows, Bytes: b,
		Elapsed: duration, RowsPerSecond: float64(rows) / elapsedSeconds,
	}
	if b > 0 {
		event.MiBPerSecond = float64(b) / (1024 * 1024) / elapsedSeconds
	}
	if p.onProgress != nil {
		p.onProgress(event)
	}
	if p.log == nil {
		return
	}
	prefix := "progress"
	if final {
		prefix = "stage done"
	}
	if b > 0 {
		fmt.Fprintf(p.log, "%s: %s %s rows=%d bytes=%s rows/s=%.0f MiB/s=%.1f elapsed=%s\n", prefix, p.phase, p.label, rows, formatBytes(b), event.RowsPerSecond, event.MiBPerSecond, duration.Round(time.Second))
		return
	}
	fmt.Fprintf(p.log, "%s: %s %s rows=%d rows/s=%.0f elapsed=%s\n", prefix, p.phase, p.label, rows, event.RowsPerSecond, duration.Round(time.Second))
}

func emitProgress(cfg resolvedConfig, event ProgressEvent) {
	if cfg.OnProgress != nil {
		cfg.OnProgress(event)
	}
}
func formatBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%dB", v)
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(v)/float64(div), "KMGTPE"[exp])
}
