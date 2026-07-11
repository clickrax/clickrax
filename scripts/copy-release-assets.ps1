# Copy built binaries into release/v2.3/ for GitHub web upload (links work without separate Release).
# Called from prepare-github.ps1 and prepare-web-upload.ps1

param(
    [Parameter(Mandatory = $true)]
    [string]$DestRoot
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path $PSScriptRoot -Parent
$bin = Join-Path $repoRoot "build\bin"
$dest = Join-Path $DestRoot "release\v2.3"

$assets = @(
    "clickrax.exe",
    "clickrax-cli.exe",
    "clickrax-amd64-installer.exe"
)
$zipName = "clickrax-windows-amd64-portable.zip"
$zipPath = Join-Path $bin $zipName

if (-not (Test-Path $bin)) {
    Write-Host "WARN: build\bin not found - run .\scripts\prepare-github.ps1 first" -ForegroundColor Yellow
    return
}

foreach ($name in $assets) {
    $src = Join-Path $bin $name
    if (-not (Test-Path $src)) {
        Write-Host "WARN: missing $src" -ForegroundColor Yellow
        return
    }
}

if (-not (Test-Path $zipPath)) {
    Write-Host "Create ZIP: $zipName" -ForegroundColor Cyan
    Compress-Archive -Path (Join-Path $bin "clickrax.exe"), (Join-Path $bin "clickrax-cli.exe") -DestinationPath $zipPath -Force
}

New-Item -ItemType Directory -Path $dest -Force | Out-Null
$readme = Join-Path $repoRoot "release\v2.3\README.md"
if (Test-Path $readme) {
    Copy-Item $readme (Join-Path $dest "README.md") -Force
}

foreach ($name in $assets) {
    Copy-Item (Join-Path $bin $name) (Join-Path $dest $name) -Force
    Write-Host "OK release/v2.3/$name" -ForegroundColor Green
}
Copy-Item $zipPath (Join-Path $dest $zipName) -Force
Write-Host "OK release/v2.3/$zipName" -ForegroundColor Green
