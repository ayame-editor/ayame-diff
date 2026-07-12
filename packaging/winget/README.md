# WinGet manifests

WinGet manifests are generated from release archives rather than edited here
with placeholder hashes:

```bash
go run ./cmd/packaging-gen \
  -version v1.2.3 \
  -checksums release/SHA256SUMS \
  -out dist/packaging
```

The result is under
`dist/packaging/winget/manifests/h/Hjosugi/AyameDiff/<version>/` and uses the
current three-file 1.12 schema. `scripts/package-release.sh` runs this command
and creates a release attachment automatically.

Initial community-repository submission:
[microsoft/winget-pkgs#400883](https://github.com/microsoft/winget-pkgs/pull/400883).
