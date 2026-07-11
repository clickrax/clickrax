# Restore config from quarantine backups (run with ClickRAX closed).
#   .\scripts\restore-config.ps1

$ErrorActionPreference = "Stop"
$dir = Join-Path $env:ProgramData "ClickRAX"
$cfg = Join-Path $dir "config.json"

Get-Process clickrax -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "Stop ClickRAX (PID $($_.Id))..." -ForegroundColor Yellow
    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Seconds 2
Remove-Item (Join-Path $dir "config.lock") -Force -ErrorAction SilentlyContinue

$corrupt = Get-ChildItem $dir -Filter "config.json.corrupt-*" -ErrorAction SilentlyContinue |
    Sort-Object Length -Descending
if (-not $corrupt) {
    Write-Host "No config.json.corrupt-* backups in $dir" -ForegroundColor Red
    exit 1
}

$best = $corrupt[0]
Write-Host "Restore from: $($best.Name) ($($best.Length) bytes)" -ForegroundColor Cyan
Copy-Item $best.FullName $cfg -Force
Write-Host "Restored config.json. Rebuild/install ClickRAX 2.3.2+ and start the app." -ForegroundColor Green
