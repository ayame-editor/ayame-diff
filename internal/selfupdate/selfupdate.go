// Package selfupdate implements the `update` and `remove` subcommands: resolve
// the latest GitHub release, download the matching asset, verify the release
// signature and the asset's SHA-256 against the release's SHA256SUMS, and
// replace the running binary in place. It is standard-library only.
//
// Downloads and archive extraction are bounded, and a build carrying a release
// public key refuses an unsigned release; see verify.go and
// docs/packaging.md (#148).
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository releases are pulled from.
const Repo = "ayame-editor/ayame-diff"

// Release is the subset of the GitHub release API we use.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is one downloadable release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func httpClient() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// LatestRelease fetches the most recent published release.
func LatestRelease(ctx context.Context) (*Release, error) {
	url := "https://api.github.com/repos/" + Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github release lookup failed: HTTP %d", resp.StatusCode)
	}
	body, err := readBounded(resp.Body, maxMetadataBytes, "release metadata")
	if err != nil {
		return nil, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no release tag found")
	}
	return &rel, nil
}

// AssetName is the release asset for this platform at the given version tag.
// Windows ships a single .zip for both arches; unix ships per-arch tarballs.
func AssetName(version string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("ayame-diff-%s-windows.zip", version)
	}
	return fmt.Sprintf("ayame-diff-%s-%s-%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}

// binaryInArchive is the path of the executable inside the downloaded asset for
// this platform.
func binaryInArchive(version string) string {
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "arm64" {
			return "arm64/ayame-diff.exe"
		}
		return "ayame-diff.exe"
	}
	return fmt.Sprintf("ayame-diff-%s-%s-%s/ayame-diff", version, runtime.GOOS, runtime.GOARCH)
}

// NeedsUpdate reports whether latest is a newer version than current. A non-tag
// current (e.g. "dev") is always considered updatable.
func NeedsUpdate(current, latest string) bool {
	c := parseVersion(current)
	l := parseVersion(latest)
	if c == nil {
		return true // dev / unknown build: allow updating to a real release
	}
	if l == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parseVersion parses "v1.2.3" / "1.2.3" (ignoring any pre-release suffix) into
// [major, minor, patch], or nil if it is not a version number.
func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "ayame-diff ")
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, " -+("); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// Bounds for everything this package reads from the network or an archive
// (#148). An update must not be able to exhaust memory, whether the release is
// hostile or merely wrong.
const (
	// maxArchiveBytes bounds a release archive. The largest published archive
	// is a few tens of megabytes.
	maxArchiveBytes = 128 << 20
	// maxMetadataBytes bounds SHA256SUMS and the release API response.
	maxMetadataBytes = 4 << 20
	// maxBinaryBytes bounds the executable expanded out of the archive, which
	// is what a decompression bomb would target.
	maxBinaryBytes = 256 << 20
	// maxArchiveEntries bounds how far the archive is scanned for the binary.
	maxArchiveEntries = 10000
)

// download fetches url into memory, refusing a body larger than limit so a
// hostile or corrupt asset cannot exhaust memory.
func download(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d (%s)", resp.StatusCode, url)
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("download is %d bytes, over the %d byte limit (%s)", resp.ContentLength, limit, url)
	}
	return readBounded(resp.Body, limit, url)
}

// readBounded reads at most limit bytes and treats one byte more as a failure,
// so a truncated read is never mistaken for a complete one.
func readBounded(r io.Reader, limit int64, what string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%s is larger than the %d byte limit", what, limit)
	}
	return body, nil
}

// findAsset returns the asset with the given name.
func findAsset(rel *Release, name string) (Asset, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// expectedSHA reads the hash for assetName from a SHA256SUMS body. Lines look
// like "<hex>  ./<name>" or "<hex>  <name>".
func expectedSHA(sums []byte, assetName string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "./")
		if name == assetName {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// extractBinary pulls binaryInArchive out of the archive bytes (tar.gz on unix,
// zip on windows) and returns the executable's bytes.
func extractBinary(archive []byte, version string) ([]byte, error) {
	want := binaryInArchive(version)
	if runtime.GOOS == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		if len(zr.File) > maxArchiveEntries {
			return nil, fmt.Errorf("archive holds %d entries, over the %d entry limit", len(zr.File), maxArchiveEntries)
		}
		for _, f := range zr.File {
			if f.Name != want {
				continue
			}
			// The declared size is a hint, not a promise: check it to reject an
			// obvious bomb early, then hold the read to the same bound.
			if f.UncompressedSize64 > maxBinaryBytes {
				return nil, fmt.Errorf("%s declares %d bytes, over the %d byte limit", want, f.UncompressedSize64, uint64(maxBinaryBytes))
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return readBounded(rc, maxBinaryBytes, want)
		}
		return nil, fmt.Errorf("%s not found in archive", want)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for entries := 0; entries < maxArchiveEntries; entries++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Name != want {
			continue
		}
		if h.Size > maxBinaryBytes {
			return nil, fmt.Errorf("%s declares %d bytes, over the %d byte limit", want, h.Size, int64(maxBinaryBytes))
		}
		return readBounded(tr, maxBinaryBytes, want)
	}
	return nil, fmt.Errorf("%s not found in archive", want)
}

// Update checks for a newer release and, if found, downloads it, verifies its
// checksum, and replaces the running executable in place. Progress is written
// to w. currentVersion is the running binary's version string.
func Update(ctx context.Context, currentVersion string, w io.Writer) error {
	fmt.Fprintln(w, "checking for updates...")
	rel, err := LatestRelease(ctx)
	if err != nil {
		return err
	}
	if !NeedsUpdate(currentVersion, rel.TagName) {
		fmt.Fprintf(w, "already up to date (%s)\n", currentVersion)
		return nil
	}
	fmt.Fprintf(w, "updating %s -> %s\n", currentVersion, rel.TagName)

	assetName := AssetName(rel.TagName)
	asset, ok := findAsset(rel, assetName)
	if !ok {
		return fmt.Errorf("no asset %q in release %s", assetName, rel.TagName)
	}
	sumsAsset, ok := findAsset(rel, "SHA256SUMS")
	if !ok {
		return fmt.Errorf("release %s has no SHA256SUMS", rel.TagName)
	}

	fmt.Fprintf(w, "downloading %s...\n", assetName)
	archive, err := download(ctx, asset.URL, maxArchiveBytes)
	if err != nil {
		return err
	}
	sums, err := download(ctx, sumsAsset.URL, maxMetadataBytes)
	if err != nil {
		return err
	}
	// The signature is checked before the checksums are trusted: the checksums
	// are only as good as the file they come from (#148).
	if err := verifyRelease(ctx, rel, sums, w); err != nil {
		return err
	}
	want, ok := expectedSHA(sums, assetName)
	if !ok {
		return fmt.Errorf("no checksum for %s in SHA256SUMS", assetName)
	}
	got := sha256Hex(archive)
	if got != want {
		return fmt.Errorf("checksum mismatch for %s (got %s, want %s)", assetName, got, want)
	}
	fmt.Fprintln(w, "checksum verified")

	binary, err := extractBinary(archive, rel.TagName)
	if err != nil {
		return err
	}
	if err := replaceSelf(binary); err != nil {
		return err
	}
	fmt.Fprintf(w, "updated to %s\n", rel.TagName)
	return nil
}

// verifyRelease checks the release signature over SHA256SUMS. A build with no
// compiled-in key cannot verify anything, so it says so and continues on the
// checksum alone; a build with a key refuses a release that is unsigned or
// signed by someone else.
func verifyRelease(ctx context.Context, rel *Release, sums []byte, w io.Writer) error {
	sigAsset, signed := findAsset(rel, signatureAssetName)
	if !signatureRequired() {
		if !signed {
			fmt.Fprintf(w, "warning: release %s is unsigned; verifying the checksum only\n", rel.TagName)
		}
		return nil
	}
	if !signed {
		return fmt.Errorf("release %s has no %s: %w", rel.TagName, signatureAssetName, errUnsignedRelease)
	}
	signature, err := download(ctx, sigAsset.URL, maxSignatureBytes)
	if err != nil {
		return err
	}
	if err := verifySums(sums, signature); err != nil {
		return fmt.Errorf("release %s failed signature verification: %w", rel.TagName, err)
	}
	fmt.Fprintln(w, "signature verified")
	return nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// replaceSelf overwrites the running executable with newBinary using a
// temp-file-then-rename so a crash can't leave a half-written binary. On
// Windows the running exe is moved aside first (it cannot be overwritten).
func replaceSelf(newBinary []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".ayame-diff-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (need write permission): %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(tmpName)
			return err
		}
		if err := os.Rename(tmpName, exe); err != nil {
			os.Rename(old, exe) // best-effort rollback
			return err
		}
		os.Remove(old)
		return nil
	}
	if err := os.Rename(tmpName, exe); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// ManagedInstall names the package manager that owns the current binary, or ""
// if it looks like a plain standalone install. Used by `remove` to defer to the
// package manager instead of deleting a managed file.
func ManagedInstall() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	p := strings.ToLower(exe)
	switch {
	case strings.Contains(p, "/homebrew/") || strings.Contains(p, "/cellar/"):
		return "Homebrew"
	case strings.Contains(p, "/scoop/"):
		return "Scoop"
	case strings.Contains(p, "/nix/store/"):
		return "Nix"
	default:
		return ""
	}
}

// Remove deletes the running executable (a standalone install). It refuses when
// the binary is managed by a package manager, telling the caller to use that
// instead.
func Remove(w io.Writer) error {
	if mgr := ManagedInstall(); mgr != "" {
		return fmt.Errorf("this install is managed by %s; remove it with %s instead", mgr, strings.ToLower(mgr))
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// A running exe can't delete itself directly; move it aside.
		aside := exe + ".delete-me"
		if err := os.Rename(exe, aside); err != nil {
			return err
		}
		fmt.Fprintf(w, "moved %s aside; delete %s to finish removal\n", exe, aside)
		return nil
	}
	if err := os.Remove(exe); err != nil {
		return err
	}
	fmt.Fprintf(w, "removed %s\n", exe)
	return nil
}
