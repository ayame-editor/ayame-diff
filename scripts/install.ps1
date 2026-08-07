<#
.SYNOPSIS
    Installs ayame-diff on Windows.
.DESCRIPTION
    Downloads the latest (or a pinned) ayame-diff Windows release from GitHub,
    verifies its SHA-256 checksum, and installs ayame-diff.exe into a per-user
    programs directory. Picks the ARM64 build automatically on ARM64 hosts.
.PARAMETER Version
    Release tag to install (e.g. v0.5.1). Defaults to $env:VERSION, else latest.
.PARAMETER InstallDir
    Target directory. Defaults to %LOCALAPPDATA%\Programs\ayame-diff.
.EXAMPLE
    irm https://raw.githubusercontent.com/hjosugi/ayame-diff/main/scripts/install.ps1 | iex
.EXAMPLE
    .\install.ps1 -Version v0.5.1
#>
[CmdletBinding()]
param(
    [string]$Version = $env:VERSION,
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "Programs\ayame-diff")
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Repo = "ayame-editor/ayame-diff"
$Binary = "ayame-diff"

function Info($msg) { Write-Host "==> $msg" }

# Prefer TLS 1.2 on older PowerShell / .NET Framework hosts.
try {
    [Net.ServicePointManager]::SecurityProtocol =
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {}

# --- detect architecture -----------------------------------------------------
$archEnv = "$env:PROCESSOR_ARCHITECTURE $env:PROCESSOR_ARCHITEW6432"
$IsArm64 = $archEnv -match "ARM64"
if ($IsArm64) { Info "detected Windows on ARM64" } else { Info "detected Windows on x64" }

# --- resolve version ---------------------------------------------------------
if ([string]::IsNullOrWhiteSpace($Version)) {
    Info "resolving latest release tag from GitHub"
    $latest = Invoke-RestMethod -UseBasicParsing `
        -Uri "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "ayame-diff-installer" }
    $Version = $latest.tag_name
    if ([string]::IsNullOrWhiteSpace($Version)) {
        throw "could not determine the latest release tag"
    }
}
Info "installing $Binary $Version"

$Asset = "$Binary-$Version-windows.zip"
$Base = "https://github.com/$Repo/releases/download/$Version"

# --- workspace ---------------------------------------------------------------
$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("ayame-diff-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $Work | Out-Null
try {
    $ZipPath = Join-Path $Work $Asset
    $SumsPath = Join-Path $Work "SHA256SUMS"

    Info "downloading $Asset"
    Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Asset" -OutFile $ZipPath
    Info "downloading SHA256SUMS"
    Invoke-WebRequest -UseBasicParsing -Uri "$Base/SHA256SUMS" -OutFile $SumsPath

    # --- verify checksum -----------------------------------------------------
    Info "verifying sha256 checksum"
    # SHA256SUMS lines look like: "<hash>  ./ayame-diff-<tag>-windows.zip"
    $pattern = "(^|[\s/])" + [regex]::Escape($Asset) + "\s*$"
    $line = Get-Content $SumsPath | Where-Object { $_ -match $pattern } | Select-Object -First 1
    if (-not $line) { throw "no checksum for $Asset in SHA256SUMS" }
    $expected = (($line -split '\s+') | Where-Object { $_ } | Select-Object -First 1).ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $ZipPath).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
        throw "checksum mismatch for ${Asset}: expected $expected, got $actual"
    }
    Info "checksum OK"

    # --- extract -------------------------------------------------------------
    Info "extracting archive"
    $ExtractDir = Join-Path $Work "unzipped"
    Expand-Archive -Path $ZipPath -DestinationPath $ExtractDir -Force

    # The zip contains "ayame-diff-<tag>-windows\" with ayame-diff.exe (x64)
    # and arm64\ayame-diff.exe.
    $Top = Join-Path $ExtractDir "$Binary-$Version-windows"
    if ($IsArm64) {
        $Source = Join-Path $Top "arm64\$Binary.exe"
    } else {
        $Source = Join-Path $Top "$Binary.exe"
    }
    if (-not (Test-Path $Source)) {
        throw "expected binary not found in archive: $Source"
    }

    # --- install -------------------------------------------------------------
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $Dest = Join-Path $InstallDir "$Binary.exe"
    Info "installing to $Dest"
    Copy-Item -Path $Source -Destination $Dest -Force

    Info "installed $Binary $Version to $Dest"

    # --- PATH note -----------------------------------------------------------
    $onPath = ($env:PATH -split ';') |
        Where-Object { $_ } |
        ForEach-Object { $_.TrimEnd('\') } |
        Where-Object { $_ -ieq $InstallDir.TrimEnd('\') }
    if (-not $onPath) {
        Write-Host ""
        Write-Host "NOTE: $InstallDir is not on your PATH."
        Write-Host "Add it for your user with this PowerShell command:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', (`"$InstallDir;`" + [Environment]::GetEnvironmentVariable('Path','User')), 'User')"
        Write-Host "Then open a new terminal and run: $Binary --version"
    } else {
        Info "run '$Binary --version' to verify"
    }
}
finally {
    Remove-Item -Recurse -Force -Path $Work -ErrorAction SilentlyContinue
}
