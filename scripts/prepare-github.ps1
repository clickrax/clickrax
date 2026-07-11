# Prepare ClickRAX repo for GitHub publish (clean junk + full release build).
# Run from repo root:  .\scripts\prepare-github.ps1

param(
    [switch]$SkipBuild,
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root

Write-Host "=== ClickRAX: prepare for GitHub ===" -ForegroundColor Green

$removePaths = @(
    "cli.exe",
    "pbs-win-backup.exe",
    "clickrax.exe",
    "clickrax-cli.exe",
    "build\bin",
    "build\windows\installer\tmp",
    "frontend\dist",
    "frontend\node_modules"
)

foreach ($rel in $removePaths) {
    $full = Join-Path $root $rel
    if (Test-Path $full) {
        Write-Host "Remove: $rel" -ForegroundColor DarkYellow
        Remove-Item $full -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Get-ChildItem $root -Filter "rsrc_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem (Join-Path $root "build\windows") -Filter "rsrc_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force

$orphan = Join-Path $root "third_party\proxmoxbackupclient_go"
if (Test-Path $orphan) {
    $items = @(Get-ChildItem $orphan -Force -ErrorAction SilentlyContinue)
    if ($items.Count -eq 0) {
        Write-Host "Remove empty: third_party\proxmoxbackupclient_go" -ForegroundColor DarkYellow
        Remove-Item $orphan -Recurse -Force -ErrorAction SilentlyContinue
    }
}

if (-not $SkipBuild) {
    Write-Host "=== Full release build ===" -ForegroundColor Cyan
    & (Join-Path $root "build.ps1") -Installer -GitHubRepo "clickrax/clickrax"
}

if (-not $SkipTests) {
    Write-Host "=== go test ./... ===" -ForegroundColor Cyan
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }
}

& (Join-Path $PSScriptRoot "copy-release-assets.ps1") -DestRoot $root

Write-Host ""
Write-Host "=== Ready for GitHub ===" -ForegroundColor Green
Write-Host "Binaries for download: release/v2.3/ (README links point here)"
Write-Host "Do NOT upload: build/bin, node_modules, frontend/dist, config.json"
Write-Host "Web upload: .\scripts\prepare-web-upload.ps1"
Write-Host "Guide: docs\github-publish.ru.md"
