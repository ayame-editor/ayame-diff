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
        @{ OS = $Parts[0]; Arch = $Parts[1]; Ext = $(if ($Parts[2] -eq "-") { "" } else { $Parts[2] }) }
    }
}

Push-Location $Root
try {
    foreach ($Target in $Targets) {
        $Output = Join-Path $Dist ("ayame-diff-{0}-{1}{2}" -f $Target.OS, $Target.Arch, $Target.Ext)
        Write-Host "building $Output"
        $env:CGO_ENABLED = "0"
        $env:GOOS = $Target.OS
        $env:GOARCH = $Target.Arch
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output ./cmd/ayame-diff
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    }

    $Lines = Get-ChildItem $Dist -Filter "ayame-diff-*" -File | Sort-Object Name | ForEach-Object {
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
