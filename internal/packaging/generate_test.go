package packaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "SHA256SUMS")
	hash := strings.Repeat("ab", 32)
	content := hash + "  ./ayame-diff-v1.2.3-windows.zip\n" +
		hash + "  ./ayame-diff-v1.2.3-darwin-amd64.tar.gz\n" +
		hash + "  ./ayame-diff-v1.2.3-darwin-arm64.tar.gz\n"
	if err := os.WriteFile(checksums, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := Generate("v1.2.3", checksums, out, time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	installer := readGenerated(t, filepath.Join(out, "winget", "manifests", "h", "Hjosugi", "AyameDiff", "1.2.3", "Hjosugi.AyameDiff.installer.yaml"))
	for _, marker := range []string{"ManifestVersion: 1.12.0", "NestedInstallerType: portable", "Architecture: x64", "Architecture: arm64", "ReleaseDate: 2026-07-11", strings.ToUpper(hash)} {
		if !strings.Contains(installer, marker) {
			t.Errorf("installer missing %q", marker)
		}
	}
	var scoopManifest map[string]any
	if err := json.Unmarshal([]byte(readGenerated(t, filepath.Join(out, "scoop", "ayame-diff.json"))), &scoopManifest); err != nil {
		t.Fatal(err)
	}
	if scoopManifest["version"] != "1.2.3" {
		t.Fatalf("scoop=%v", scoopManifest)
	}
	formula := readGenerated(t, filepath.Join(out, "homebrew", "ayame-diff.rb"))
	if strings.Count(formula, hash) != 2 {
		t.Fatalf("formula hashes=%s", formula)
	}
}

// TestRepoManifestsMatchGenerator guards the committed Scoop and Homebrew
// placeholders against drift (#172): they are generated artifacts, not
// hand-maintained files, so they must reproduce exactly what Generate emits for
// the canonical placeholder version 0.0.0 with zero-filled hashes. Release
// automation regenerates them per tag with the real version and checksums; this
// test only pins their canonical shape (description, structure) so a divergent
// hand edit fails CI instead of shipping a stale manifest.
func TestRepoManifestsMatchGenerator(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "SHA256SUMS")
	zero := strings.Repeat("0", 64)
	content := zero + "  ayame-diff-v0.0.0-windows.zip\n" +
		zero + "  ayame-diff-v0.0.0-darwin-amd64.tar.gz\n" +
		zero + "  ayame-diff-v0.0.0-darwin-arm64.tar.gz\n"
	if err := os.WriteFile(checksums, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := Generate("0.0.0", checksums, out, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ generated, committed string }{
		{filepath.Join(out, "scoop", "ayame-diff.json"), filepath.Join("..", "..", "packaging", "scoop", "ayame-diff.json")},
		{filepath.Join(out, "homebrew", "ayame-diff.rb"), filepath.Join("..", "..", "packaging", "homebrew", "ayame-diff.rb")},
	} {
		// Compare on content, not byte-for-byte: git may check the committed
		// files out with CRLF on Windows (autocrlf) while the generator always
		// emits LF. Release generation runs on Linux, so the published bytes are
		// LF regardless; this test only pins the manifests' shape.
		if lf(readGenerated(t, m.generated)) != lf(readGenerated(t, m.committed)) {
			t.Errorf("%s is out of sync with the generator; regenerate it with\n"+
				"  go run ./cmd/packaging-gen -version 0.0.0 -checksums <zero-SHA256SUMS> -out <dir>\n"+
				"and copy the result into packaging/", m.committed)
		}
	}
}

func TestGenerateRequiresAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate("1.0.0", path, t.TempDir(), time.Now()); err == nil || !strings.Contains(err.Error(), "windows.zip") {
		t.Fatalf("err=%v", err)
	}
}

// lf normalizes CRLF to LF so manifest comparisons are line-ending agnostic.
func lf(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func readGenerated(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
