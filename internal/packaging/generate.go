// Package packaging generates release-manager manifests from one checksum file.
package packaging

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const wingetID = "Hjosugi.AyameDiff"

// Generate writes WinGet, Scoop, and Homebrew metadata for a released version.
func Generate(version, checksums, output string, releaseDate time.Time) error {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" || strings.ContainsAny(version, `/\\`) {
		return fmt.Errorf("invalid version %q", version)
	}
	hashes, err := readChecksums(checksums)
	if err != nil {
		return err
	}
	asset := func(name string) (string, error) {
		hash := hashes[name]
		if len(hash) != 64 {
			return "", fmt.Errorf("checksum for %s is missing", name)
		}
		return strings.ToUpper(hash), nil
	}
	windowsName := fmt.Sprintf("ayame-diff-v%s-windows.zip", version)
	windowsHash, err := asset(windowsName)
	if err != nil {
		return err
	}
	darwinAMD, err := asset(fmt.Sprintf("ayame-diff-v%s-darwin-amd64.tar.gz", version))
	if err != nil {
		return err
	}
	darwinARM, err := asset(fmt.Sprintf("ayame-diff-v%s-darwin-arm64.tar.gz", version))
	if err != nil {
		return err
	}
	date := releaseDate.UTC().Format("2006-01-02")
	wingetDir := filepath.Join(output, "winget", "manifests", "h", "Hjosugi", "AyameDiff", version)
	files := map[string]string{
		filepath.Join(wingetDir, wingetID+".yaml"):              wingetVersion(version),
		filepath.Join(wingetDir, wingetID+".locale.en-US.yaml"): wingetLocale(version),
		filepath.Join(wingetDir, wingetID+".installer.yaml"):    wingetInstaller(version, windowsHash, date),
		filepath.Join(output, "scoop", "ayame-diff.json"):       scoop(version, strings.ToLower(windowsHash)),
		filepath.Join(output, "homebrew", "ayame-diff.rb"):      homebrew(version, strings.ToLower(darwinAMD), strings.ToLower(darwinARM)),
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readChecksums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 {
			result[strings.TrimPrefix(fields[1], "./")] = fields[0]
		}
	}
	return result, scanner.Err()
}

func wingetVersion(version string) string {
	return fmt.Sprintf(`# yaml-language-server: $schema=https://aka.ms/winget-manifest.version.1.12.0.schema.json

PackageIdentifier: %s
PackageVersion: %s
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.12.0
`, wingetID, version)
}

func wingetLocale(version string) string {
	return fmt.Sprintf(`# yaml-language-server: $schema=https://aka.ms/winget-manifest.defaultLocale.1.12.0.schema.json

PackageIdentifier: %s
PackageVersion: %s
PackageLocale: en-US
Publisher: hjosugi
PublisherUrl: https://github.com/hjosugi
PublisherSupportUrl: https://github.com/hjosugi/ayame-diff/issues
Author: hjosugi
PackageName: ayame-diff
PackageUrl: https://github.com/hjosugi/ayame-diff
License: MIT
LicenseUrl: https://github.com/hjosugi/ayame-diff/blob/v%s/LICENSE
ShortDescription: Fast CSV, text, folder, and binary diff with a local web GUI
Description: A native, memory-bounded comparison tool with Japanese encoding support, structured CSV diff, patches, reports, and a bilingual local web GUI.
Moniker: ayame-diff
Tags:
- csv
- diff
- folder
- japanese
- patch
- text
ReleaseNotesUrl: https://github.com/hjosugi/ayame-diff/releases/tag/v%s
ManifestType: defaultLocale
ManifestVersion: 1.12.0
`, wingetID, version, version, version)
}

func wingetInstaller(version, hash, date string) string {
	name := fmt.Sprintf("ayame-diff-v%s-windows", version)
	url := fmt.Sprintf("https://github.com/hjosugi/ayame-diff/releases/download/v%s/%s.zip", version, name)
	return fmt.Sprintf(`# yaml-language-server: $schema=https://aka.ms/winget-manifest.installer.1.12.0.schema.json

PackageIdentifier: %s
PackageVersion: %s
InstallerType: zip
NestedInstallerType: portable
InstallModes:
- silent
UpgradeBehavior: install
Commands:
- ayame-diff
ReleaseDate: %s
Installers:
- Architecture: x64
  NestedInstallerFiles:
  - RelativeFilePath: %s\ayame-diff.exe
    PortableCommandAlias: ayame-diff
  InstallerUrl: %s
  InstallerSha256: %s
- Architecture: arm64
  NestedInstallerFiles:
  - RelativeFilePath: %s\arm64\ayame-diff.exe
    PortableCommandAlias: ayame-diff
  InstallerUrl: %s
  InstallerSha256: %s
ManifestType: installer
ManifestVersion: 1.12.0
`, wingetID, version, date, name, url, hash, name, url, hash)
}

func scoop(version, hash string) string {
	return fmt.Sprintf(`{
  "version": %q,
  "description": "Fast CSV, text, folder, and binary diff with a local web GUI.",
  "homepage": "https://github.com/hjosugi/ayame-diff",
  "license": "MIT",
  "architecture": {
    "64bit": {"url": "https://github.com/hjosugi/ayame-diff/releases/download/v%s/ayame-diff-v%s-windows.zip", "hash": %q, "extract_dir": "ayame-diff-v%s-windows"},
    "arm64": {"url": "https://github.com/hjosugi/ayame-diff/releases/download/v%s/ayame-diff-v%s-windows.zip", "hash": %q, "extract_dir": "ayame-diff-v%s-windows\\arm64"}
  },
  "bin": "ayame-diff.exe",
  "checkver": {"github": "https://github.com/hjosugi/ayame-diff"},
  "autoupdate": {
    "architecture": {
      "64bit": {"url": "https://github.com/hjosugi/ayame-diff/releases/download/v$version/ayame-diff-v$version-windows.zip", "extract_dir": "ayame-diff-v$version-windows"},
      "arm64": {"url": "https://github.com/hjosugi/ayame-diff/releases/download/v$version/ayame-diff-v$version-windows.zip", "extract_dir": "ayame-diff-v$version-windows\\arm64"}
    },
    "hash": {"url": "https://github.com/hjosugi/ayame-diff/releases/download/v$version/SHA256SUMS", "regex": "([a-fA-F0-9]{64})\\s+(?:\\./)?$basename"}
  }
}
`, version, version, version, hash, version, version, version, hash, version)
}

func homebrew(version, amd64, arm64 string) string {
	return fmt.Sprintf(`class AyameDiff < Formula
  desc "Fast CSV, text, folder, and binary diff with a local web GUI"
  homepage "https://github.com/hjosugi/ayame-diff"
  version %q
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-arm64.tar.gz"
      sha256 %q
    end
    on_intel do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-amd64.tar.gz"
      sha256 %q
    end
  end

  livecheck do
    url "https://github.com/hjosugi/ayame-diff"
    strategy :github_latest
  end

  def install
    bin.install "ayame-diff"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ayame-diff --version")
  end
end
`, version, arm64, amd64)
}
