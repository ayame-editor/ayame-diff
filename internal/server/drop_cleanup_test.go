package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestDropCleanupSpareLiveSessions covers #168: opportunistic cleanup reclaims
// an orphaned (previous-run) drop directory but never a live session's — even
// when the live directory is old enough to otherwise qualify.
func TestDropCleanupSpareLiveSessions(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // isolate drop storage on Linux
	saved := dropSessionTTL
	dropSessionTTL = time.Nanosecond // every directory qualifies by age
	defer func() { dropSessionTTL = saved }()

	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	rootA, err := s.dropRoot("A")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(rootA, "upload.txt")
	if err := os.WriteFile(marker, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	// An orphan directory from a prior run: present on disk, not in s.drops.
	base := filepath.Dir(rootA)
	orphan := filepath.Join(base, "session-orphan")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatal(err)
	}

	// A second session triggers cleanup.
	if _, err := s.dropRoot("B"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("live session A's upload was deleted by cleanup: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan directory was not reclaimed: err=%v", err)
	}
}

// TestDropRootConcurrent covers #168: many concurrent first-drops must be
// race-free (run under -race) and each session gets its own directory.
func TestDropRootConcurrent(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}

	const n = 24
	roots := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			root, err := s.dropRoot(fmt.Sprintf("session-%d", i))
			if err != nil {
				t.Errorf("dropRoot(%d): %v", i, err)
				return
			}
			roots[i] = root
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, root := range roots {
		if root == "" {
			t.Fatalf("session %d got no root", i)
		}
		if seen[root] {
			t.Fatalf("duplicate root %q", root)
		}
		seen[root] = true
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			t.Fatalf("session %d root missing: %v", i, err)
		}
	}
	// A repeat call for a known session returns the same directory.
	again, err := s.dropRoot("session-0")
	if err != nil || again != roots[0] {
		t.Fatalf("repeat dropRoot: %q (%v), want %q", again, err, roots[0])
	}
}
