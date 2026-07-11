$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Version = if ($env:VERSION) { $env:VERSION } else { (git -C $Root describe --tags --always --dirty) }
$Dist = Join-Path $Root "dist"
Remove-Item -Recurse -Force $Dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $Dist | Out-Null
$Targets = Get-Content (Join-Path $Root "scripts/targets.txt") | ForEach-Object {
    $Line = $_.Trim()
    if ($Line -and -not $Line.StartsWith("#")) {
        $Parts = $Line -split '\s+'
        if ($Parts[0] -eq "windows") {
            @{ Arch = $Parts[1]; Name = "ayame-diff-windows-$($Parts[1])$($Parts[2])" }
        }
    }
}

Push-Location $Root
try {
    foreach ($Target in $Targets) {
        $Output = Join-Path $Dist $Target.Name
        Write-Host "building $Output"
        $env:CGO_ENABLED = "0"
        $env:GOOS = "windows"
        $env:GOARCH = $Target.Arch
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output ./cmd/ayame-diff
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
