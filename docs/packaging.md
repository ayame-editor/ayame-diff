<!-- i18n: language-switcher -->
[English](packaging.md) | [日本語](packaging.ja.md)

# Packaging and Windows trust

## Distribution roles

| Channel | Audience and responsibility |
|---|---|
| GitHub Release ZIP / `.app` / tar | Canonical signed-by-checksum artifacts and portable GUI launchers. No package manager is required. |
| Scoop | Developer-friendly Windows bucket manifest with GitHub release autoupdate and SHA-256 verification. |
| WinGet | Default Windows discovery and per-user portable install for x64 and ARM64. The `ayame-diff` command alias is managed by WinGet. |
| Homebrew | Managed macOS CLI installation and upgrades. The `.app` remains available from Releases. |
| `install.ps1` / `install.sh` | Direct standalone install when a package manager is unavailable. |

Every release runs `cmd/packaging-gen` against the just-built
`SHA256SUMS`. It emits a three-file WinGet 1.12 manifest tree plus exact Scoop
and Homebrew manifests. They are attached to the release; the WinGet archive's
`manifests/` tree can be copied directly into `microsoft/winget-pkgs`.
The files follow Microsoft's
[manifest specification](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest)
and are checked against its official 1.12 JSON schemas.

Before a GitHub Release is published, a Windows runner downloads the exact
release candidate produced by the packaging job. It expands the Windows and
WinGet archives, executes the packaged x64 binary with no arguments,
`--help`, `--version`, and a real text comparison, confirms the ARM64 payload,
and verifies that both manifest entries contain the release ZIP's actual
SHA-256. The publish job cannot run unless this package-level gate succeeds.

```bash
go run ./cmd/packaging-gen \
  -version v1.2.3 -checksums release/SHA256SUMS -out dist/packaging
```

After the community manifest is accepted, install with:

```powershell
winget install ayame-diff
# unambiguous form:
winget install --id Hjosugi.AyameDiff --exact
```

## Inno Setup decision

An Inno installer is **not required at present**. WinGet and Scoop install the
portable executable transactionally, while `ayame-diff shell-install` performs
an explicit, current-user-only Explorer registration without elevation. It has
a matching `shell-uninstall`; release ZIPs include clickable wrappers for both.
Shell integration is not silently enabled by a package-manager install.

Before removing or moving a portable binary, users who opted into Explorer
integration should run `ayame-diff shell-uninstall`. Revisit an Inno/MSIX
installer if shell integration becomes automatic, machine-wide registration is
added, Start Menu/file associations require transactional rollback, or support
data shows that the explicit uninstall step is insufficient.

## Signing and malware scanning

Windows executables are currently unsigned. A SHA-256 list is generated inside
the release gate and artifacts are served from the project's GitHub release.
This verifies integrity but does not provide publisher identity or remove
SmartScreen reputation warnings. Purchase or managed signing is justified when
Windows download volume and warning-related support cost exceed certificate and
secret-management cost; at that point signing must occur before checksums,
package manifests, and malware scanning are generated.

The release workflow runs `scripts/virustotal-scan.sh` before publishing. When
the repository secret `VT_API_KEY` is configured, the script uses VirusTotal
API v3, waits for analysis, and blocks malicious or suspicious detections by
default. Without the secret it records an explicit skip; set `REQUIRE_VT=1` in
a stricter release environment to make credentials mandatory. API keys are
never printed.
The upload and polling flow uses the official
[VirusTotal API v3 file endpoint](https://docs.virustotal.com/reference/files-scan);
release archives are public artifacts and must not contain secrets.

VirusTotal results are an early-warning signal, not a substitute for source
review, reproducible checksums, or future code signing.
