package engine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type progressCounter struct {
	label       string
	enabled     bool
	rows, bytes atomic.Uint64
	start       time.Time
	cancel      context.CancelFunc
	done        chan struct{}
	once        sync.Once
}

func startProgress(parent context.Context, label string, enabled bool) *progressCounter {
	p := &progressCounter{label: label, enabled: enabled, start: time.Now()}
	if !enabled {
		return p
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
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
	}()
	return p
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
	elapsed := time.Since(p.start).Seconds()
	if elapsed <= 0 {
		elapsed = .001
	}
	rows := p.rows.Load()
	b := p.bytes.Load()
	prefix := "progress"
	if final {
		prefix = "stage done"
	}
	if b > 0 {
		fmt.Fprintf(os.Stderr, "%s: %s rows=%d bytes=%s rows/s=%.0f MiB/s=%.1f elapsed=%s\n", prefix, p.label, rows, formatBytes(b), float64(rows)/elapsed, float64(b)/(1024*1024)/elapsed, time.Since(p.start).Round(time.Second))
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s rows=%d rows/s=%.0f elapsed=%s\n", prefix, p.label, rows, float64(rows)/elapsed, time.Since(p.start).Round(time.Second))
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
