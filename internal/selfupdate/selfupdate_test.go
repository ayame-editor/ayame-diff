package selfupdate

import (
	"runtime"
	"strings"
	"testing"
)

func TestNeedsUpdate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.5.0", "v0.6.0", true},
		{"v0.6.0", "v0.6.0", false},
		{"v0.6.1", "v0.6.0", false},
		{"0.6.0", "v0.6.1", true},
		{"v0.6.0-dirty", "v0.6.1", true},
		{"ayame-diff v0.6.0 (linux/amd64, go1.26)", "v0.6.1", true},
		{"dev", "v0.6.0", true}, // unknown build always updatable
		{"v1.0.0", "v0.9.9", false},
		{"v0.9.0", "v0.10.0", true}, // numeric, not lexical
	}
	for _, c := range cases {
		if got := NeedsUpdate(c.current, c.latest); got != c.want {
			t.Errorf("NeedsUpdate(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()
	if got := parseVersion("v1.2.3"); got == nil || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("parseVersion(v1.2.3) = %v", got)
	}
	if parseVersion("dev") != nil || parseVersion("1.2") != nil || parseVersion("x.y.z") != nil {
		t.Fatal("non-versions must parse to nil")
	}
}

func TestAssetName(t *testing.T) {
	t.Parallel()
	got := AssetName("v0.6.2")
	if runtime.GOOS == "windows" {
		if got != "ayame-diff-v0.6.2-windows.zip" {
			t.Fatalf("windows asset = %q", got)
		}
	} else {
		want := "ayame-diff-v0.6.2-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
		if got != want {
			t.Fatalf("asset = %q, want %q", got, want)
		}
	}
}

func TestExpectedSHA(t *testing.T) {
	t.Parallel()
	sums := "abc123  ./ayame-diff-v0.6.2-linux-amd64.tar.gz\n" +
		"def456  ayame-diff-v0.6.2-windows.zip\n"
	if h, ok := expectedSHA([]byte(sums), "ayame-diff-v0.6.2-linux-amd64.tar.gz"); !ok || h != "abc123" {
		t.Fatalf("linux sha = %q, ok=%v", h, ok)
	}
	if h, ok := expectedSHA([]byte(sums), "ayame-diff-v0.6.2-windows.zip"); !ok || h != "def456" {
		t.Fatalf("windows sha = %q, ok=%v", h, ok)
	}
	if _, ok := expectedSHA([]byte(sums), "missing.tar.gz"); ok {
		t.Fatal("missing asset should not be found")
	}
}

func TestBinaryInArchive(t *testing.T) {
	t.Parallel()
	got := binaryInArchive("v0.6.2")
	switch runtime.GOOS {
	case "windows":
		if !strings.HasSuffix(got, "ayame-diff.exe") {
			t.Fatalf("windows binary path = %q", got)
		}
	default:
		if !strings.HasSuffix(got, "/ayame-diff") {
			t.Fatalf("unix binary path = %q", got)
		}
	}
}
