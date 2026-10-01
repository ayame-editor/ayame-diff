package e2e

import (
	"os"
	"testing"
	"time"
)

// TestEndToEndPerformanceBudget is the CI regression gate (#279). It runs one
// representative iteration of each end-to-end case at the default size and
// fails when a case is dramatically slower than its budget.
//
// The budgets are deliberately generous — roughly 20-50x the steady-state time
// on a quiet developer machine — because shared CI runners are noisy. They do
// not catch a few-percent drift; they catch a structural regression (an
// accidental O(n^2), a lost streaming path, a hang) that turns seconds into
// minutes. The measured times are logged so the CI job publishes them.
//
// It stays out of the default `go test ./...` run so that stays fast; set
// AYAME_E2E_REGRESSION=1 (the e2e-benchmarks CI job does) to run it.
func TestEndToEndPerformanceBudget(t *testing.T) {
	if os.Getenv("AYAME_E2E_REGRESSION") != "1" {
		t.Skip("set AYAME_E2E_REGRESSION=1 to run the end-to-end regression budget")
	}

	cases := []struct {
		name   string
		budget time.Duration
		run    func(t *testing.T)
	}{
		{"csv", 30 * time.Second, func(t *testing.T) {
			c := newCSVCase(t, ciCSVRows, 1000)
			summary, _ := c.run(t)
			if summary.DiffRows == 0 {
				t.Fatal("csv case produced no differences")
			}
		}},
		{"text", 30 * time.Second, func(t *testing.T) {
			c := newTextCase(t, ciTextBytes, 1000)
			if res, _, err := c.run(t, 0, 512); err != nil || res.HunkCount == 0 {
				t.Fatalf("text case: hunks=%d err=%v", res.HunkCount, err)
			}
		}},
		{"folder", 20 * time.Second, func(t *testing.T) {
			c := newFolderCase(t, ciFolderFiles, 10)
			if res, _, err := c.run(t, 0); err != nil || res.Changed == 0 {
				t.Fatalf("folder case: changed=%d err=%v", res.Changed, err)
			}
		}},
		{"long-line", 20 * time.Second, func(t *testing.T) {
			c := newLongLineCase(t, ciLineBytes)
			if res, _, err := c.run(t, ciLineBytes); err != nil || res.HunkCount != 1 {
				t.Fatalf("long-line case: hunks=%d err=%v", res.HunkCount, err)
			}
		}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			tc.run(t)
			elapsed := time.Since(start)
			t.Logf("end-to-end %s: %s (budget %s)", tc.name, elapsed.Round(time.Millisecond), tc.budget)
			if elapsed > tc.budget {
				t.Errorf("end-to-end %s took %s, past the generous %s budget; on a noisy runner that means a dramatic regression or a hang",
					tc.name, elapsed.Round(time.Millisecond), tc.budget)
			}
		})
	}

	// A single long line used to re-scan the whole accumulated line on every
	// read-buffer fill, so its cost was quadratic in the line length (#279).
	// The absolute budget above is far too loose to notice that at 4 MiB, so
	// this compares two sizes: quadrupling the bytes must not cost far more
	// than quadruple the time.
	t.Run("long-line-scaling", func(t *testing.T) {
		const smallBytes, largeBytes = 16 << 20, 64 << 20
		small := newLongLineCase(t, smallBytes)
		_, smallTime, err := small.run(t, smallBytes)
		if err != nil {
			t.Fatal(err)
		}
		large := newLongLineCase(t, largeBytes)
		_, largeTime, err := large.run(t, largeBytes)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("end-to-end long-line scaling: %d MiB %s, %d MiB %s",
			smallBytes>>20, smallTime.Round(time.Millisecond), largeBytes>>20, largeTime.Round(time.Millisecond))
		if smallTime < 10*time.Millisecond {
			return // too fast to compare reliably
		}
		// 4x the bytes should cost about 4x time; allow 12x for noise. The
		// quadratic shape was 16x and growing.
		if largeTime > 12*smallTime {
			t.Errorf("long-line comparison is super-linear: %d MiB took %s against %s for %d MiB",
				largeBytes>>20, largeTime.Round(time.Millisecond), smallTime.Round(time.Millisecond), smallBytes>>20)
		}
	})
}
