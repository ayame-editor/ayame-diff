package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestEstimatedFileDescriptors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  resolvedConfig
		want uint64
	}{
		{
			name: "partitions dominate",
			cfg:  resolvedConfig{Config: Config{Partitions: 1024, Workers: 2, MergeFanIn: 4}},
			want: 1040,
		},
		{
			name: "merge workers dominate",
			cfg:  resolvedConfig{Config: Config{Partitions: 256, Workers: 8, MergeFanIn: 256}},
			want: 2096,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := estimatedFileDescriptors(tt.cfg); got != tt.want {
				t.Fatalf("estimatedFileDescriptors = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMeetOpenFileLimit(t *testing.T) {
	t.Parallel()
	t.Run("already sufficient", func(t *testing.T) {
		called := false
		if err := meetOpenFileLimit(100, 100, 100, func(uint64) error {
			called = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if called {
			t.Fatal("raise called for sufficient soft limit")
		}
	})
	t.Run("raises soft limit", func(t *testing.T) {
		var raised uint64
		if err := meetOpenFileLimit(100, 50, 200, func(v uint64) error {
			raised = v
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if raised != 100 {
			t.Fatalf("raised to %d, want 100", raised)
		}
	})
	t.Run("rejects low hard limit", func(t *testing.T) {
		err := meetOpenFileLimit(100, 50, 75, func(uint64) error {
			return errors.New("must not be called")
		})
		if err == nil || !strings.Contains(err.Error(), "--partitions") {
			t.Fatalf("error = %v, want actionable limit error", err)
		}
	})
	t.Run("rejects failed raise", func(t *testing.T) {
		err := meetOpenFileLimit(100, 50, 200, func(uint64) error {
			return errors.New("permission denied")
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
