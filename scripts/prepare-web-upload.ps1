# Copy only files safe for GitHub web upload (no .git, node_modules, dist, build/bin, secrets).
# Run:  .\scripts\prepare-web-upload.ps1
# Output folder: ..\clickrax-github-upload\  (sibling of this repo)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$out = Join-Path (Split-Path $root -Parent) "clickrax-github-upload"

$excludeDirs = @(
    ".git", ".cursor", ".vscode", ".idea",
    "node_modules", "dist", "bin",
    "tmp", "secrets", "logs", "index", "coverage"
)

if (Test-Path $out) {
    Write-Host "Remove old: $out" -ForegroundColor DarkYellow
    Remove-Item $out -Recurse -Force
}
New-Item -ItemType Directory -Path $out | Out-Null

function ShouldSkipDir([string]$name) {
    foreach ($d in $excludeDirs) {
        if ($name -ieq $d) { return $true }
    }
    return $false
}

function Copy-Tree {
    param([string]$Src, [string]$Dst)
    Get-ChildItem $Src -Force | ForEach-Object {
        if ($_.PSIsContainer) {
            if (ShouldSkipDir $_.Name) {
                Write-Host "Skip dir: $($_.FullName.Substring($root.Length))" -ForegroundColor DarkGray
                return
            }
            $target = Join-Path $Dst $_.Name
            New-Item -ItemType Directory -Path $target -Force | Out-Null
            Copy-Tree $_.FullName $target
        } else {
            if ($_.Extension -ieq ".exe") { return }
            if ($_.Extension -ieq ".zip") { return }
            if ($_.Name -ieq "config.json") { return }
            if ($_.Name -match '\.hmac$|\.dpapi$|\.env') { return }
            if ($_.Name -match '^rsrc_windows_.*\.syso$') { return }
            $target = Join-Path $Dst $_.Name
            Copy-Item $_.FullName $target -Force
        }
    }
}

Copy-Tree $root $out

# Skip build/bin but keep build/windows, build/darwin, build/README.md, build/appicon.png
$buildBin = Join-Path $out "build\bin"
if (Test-Path $buildBin) { Remove-Item $buildBin -Recurse -Force }
$installerTmp = Join-Path $out "build\windows\installer\tmp"
if (Test-Path $installerTmp) { Remove-Item $installerTmp -Recurse -Force }

& (Join-Path $PSScriptRoot "copy-release-assets.ps1") -DestRoot $out

$count = (Get-ChildItem $out -Recurse -File).Count
$sizeMB = [math]::Round((Get-ChildItem $out -Recurse -File | Measure-Object Length -Sum).Sum / 1MB, 2)

Write-Host ""
Write-Host "=== Ready for GitHub web upload ===" -ForegroundColor Green
Write-Host "Folder: $out"
Write-Host "Files:  $count (~$sizeMB MB)"
Write-Host ""
Write-Host "Includes release/v2.3/ with exe, installer, zip - README links work after upload."
Write-Host "No .git, no node_modules, no build/bin in upload folder."
Write-Host ""
Write-Host "If release/v2.3 is empty, run .\scripts\prepare-github.ps1 first."
