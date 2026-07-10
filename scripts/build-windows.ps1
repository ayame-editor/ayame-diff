$ErrorActionPreference = "Stop"
$Version = if ($env:VERSION) { $env:VERSION } else { "v0.3.0" }
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Dist = Join-Path $Root "dist"
New-Item -ItemType Directory -Force -Path $Dist | Out-Null

$Targets = @(
    @{ Arch = "amd64"; Name = "fcsv-diff-windows-amd64.exe" },
    @{ Arch = "arm64"; Name = "fcsv-diff-windows-arm64.exe" }
)

Push-Location $Root
try {
    foreach ($Target in $Targets) {
        $Output = Join-Path $Dist $Target.Name
        Write-Host "building $Output"
        $env:CGO_ENABLED = "0"
        $env:GOOS = "windows"
        $env:GOARCH = $Target.Arch
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output ./cmd/fcsv-diff
        if ($LASTEXITCODE -ne 0) { throw "go build failed for windows/$($Target.Arch)" }
    }

    $Lines = $Targets | ForEach-Object {
        $Path = Join-Path $Dist $_.Name
        $Hash = (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
        "$Hash  $($_.Name)"
    }
    Set-Content -Path (Join-Path $Dist "SHA256SUMS-WINDOWS.txt") -Value $Lines -Encoding ascii
}
finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}
