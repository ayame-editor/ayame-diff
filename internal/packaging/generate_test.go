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

func TestGenerateRequiresAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate("1.0.0", path, t.TempDir(), time.Now()); err == nil || !strings.Contains(err.Error(), "windows.zip") {
		t.Fatalf("err=%v", err)
	}
}

func readGenerated(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
