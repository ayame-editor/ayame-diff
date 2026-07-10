$ErrorActionPreference = "Stop"
$Version = if ($env:VERSION) { $env:VERSION } else { "v0.3.0" }
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Dist = Join-Path $Root "dist"
New-Item -ItemType Directory -Force -Path $Dist | Out-Null

$Targets = @(
    @{ OS = "linux";   Arch = "amd64"; Ext = "" },
    @{ OS = "linux";   Arch = "arm64"; Ext = "" },
    @{ OS = "darwin";  Arch = "amd64"; Ext = "" },
    @{ OS = "darwin";  Arch = "arm64"; Ext = "" },
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ OS = "windows"; Arch = "arm64"; Ext = ".exe" }
)

Push-Location $Root
try {
    foreach ($Target in $Targets) {
        $Output = Join-Path $Dist ("fcsv-diff-{0}-{1}{2}" -f $Target.OS, $Target.Arch, $Target.Ext)
        Write-Host "building $Output"
        $env:CGO_ENABLED = "0"
        $env:GOOS = $Target.OS
        $env:GOARCH = $Target.Arch
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output ./cmd/fcsv-diff
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    }

    $Lines = Get-ChildItem $Dist -Filter "fcsv-diff-*" -File | Sort-Object Name | ForEach-Object {
        $Hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant()
        "$Hash  $($_.Name)"
    }
    Set-Content -Path (Join-Path $Dist "SHA256SUMS") -Value $Lines -Encoding ascii
}
finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}
