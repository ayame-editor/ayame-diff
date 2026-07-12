param(
    [Parameter(Mandatory = $true)]
    [string]$Archive,

    [Parameter(Mandatory = $true)]
    [string]$ManifestArchive,

    [Parameter(Mandatory = $true)]
    [string]$Version
)

$ErrorActionPreference = "Stop"
$Archive = (Resolve-Path -LiteralPath $Archive).Path
$ManifestArchive = (Resolve-Path -LiteralPath $ManifestArchive).Path
$PackageVersion = $Version.TrimStart("v")
$RootName = "ayame-diff-$Version-windows"
$Scratch = Join-Path ([System.IO.Path]::GetTempPath()) ("ayame-diff-package-smoke-" + [guid]::NewGuid())

function Invoke-Ayame {
    param([string[]]$Arguments)

    $Output = & $script:Exe @Arguments 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "ayame-diff $($Arguments -join ' ') exited $LASTEXITCODE`n$Output"
    }
    return $Output
}

try {
    $PackageDir = Join-Path $Scratch "package"
    $ManifestDir = Join-Path $Scratch "manifest"
    Expand-Archive -LiteralPath $Archive -DestinationPath $PackageDir
    Expand-Archive -LiteralPath $ManifestArchive -DestinationPath $ManifestDir

    $script:Exe = Join-Path $PackageDir "$RootName\ayame-diff.exe"
    $ArmExe = Join-Path $PackageDir "$RootName\arm64\ayame-diff.exe"
    foreach ($Path in @($script:Exe, $ArmExe)) {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
            throw "release archive is missing $Path"
        }
        if ((Get-Item -LiteralPath $Path).Length -eq 0) {
            throw "release executable is empty: $Path"
        }
    }

    $VersionOutput = Invoke-Ayame -Arguments @("--version")
    if ($VersionOutput -notmatch [regex]::Escape($Version)) {
        throw "version output does not contain ${Version}: $VersionOutput"
    }
    $HelpOutput = Invoke-Ayame -Arguments @("--help")
    if ($HelpOutput -notmatch "Subcommands:") {
        throw "--help output is missing the subcommand list"
    }
    $ProbeOutput = Invoke-Ayame -Arguments @()
    if ($ProbeOutput -notmatch "Subcommands:") {
        throw "no-argument package-manager probe did not print help"
    }

    $Old = Join-Path $Scratch "old.txt"
    $New = Join-Path $Scratch "new.txt"
    Set-Content -LiteralPath $Old -Value "same" -Encoding utf8NoBOM
    Set-Content -LiteralPath $New -Value "same" -Encoding utf8NoBOM
    [void](Invoke-Ayame -Arguments @("text", $Old, $New))

    $InstallerManifest = Join-Path $ManifestDir "manifests\h\Hjosugi\AyameDiff\$PackageVersion\Hjosugi.AyameDiff.installer.yaml"
    if (-not (Test-Path -LiteralPath $InstallerManifest -PathType Leaf)) {
        throw "WinGet archive is missing its installer manifest"
    }
    $Manifest = Get-Content -LiteralPath $InstallerManifest -Raw
    $ArchiveHash = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash
    if (($Manifest | Select-String -Pattern "InstallerSha256: $ArchiveHash" -AllMatches).Matches.Count -ne 2) {
        throw "WinGet manifest does not bind both architectures to the release ZIP SHA-256"
    }
    foreach ($Expected in @(
        "RelativeFilePath: $RootName\ayame-diff.exe",
        "RelativeFilePath: $RootName\arm64\ayame-diff.exe",
        "PortableCommandAlias: ayame-diff"
    )) {
        if (-not $Manifest.Contains($Expected)) {
            throw "WinGet manifest is missing: $Expected"
        }
    }

    Write-Host "Windows release package smoke passed for $Version ($ArchiveHash)"
}
finally {
    Remove-Item -LiteralPath $Scratch -Recurse -Force -ErrorAction SilentlyContinue
}
