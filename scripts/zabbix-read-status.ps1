# Zabbix UserParameter для PBS Backup (Windows)
# Скопируйте в C:\Program Files\Zabbix Agent 2\zabbix_agent2.d\pbs-win-backup.conf
#
# UserParameter=pbs.backup.status[*],powershell -NoProfile -ExecutionPolicy Bypass -File "C:\Program Files\ClickRAX\scripts\zabbix-read-status.ps1" -Key $1

param(
    [Parameter(Mandatory = $true)]
    [string]$Key
)

$statusPath = Join-Path $env:ProgramData "ClickRAX\last_status.json"
if (-not (Test-Path $statusPath)) {
    $statusPath = Join-Path $env:ProgramData "PbsWinBackup\last_status.json"
}
if (-not (Test-Path $statusPath)) {
    switch ($Key) {
        "status" { Write-Output "unknown"; exit 0 }
        default { Write-Output ""; exit 0 }
    }
}

try {
    $data = Get-Content $statusPath -Raw | ConvertFrom-Json
} catch {
    Write-Output "error"
    exit 0
}

switch ($Key) {
    "status"       { Write-Output $data.status }
    "backup_type"  { Write-Output $data.backup_type }
    "last_run"     { Write-Output $data.last_run }
    "last_success" { Write-Output $data.last_success }
    "duration_sec" { Write-Output $data.duration_sec }
    "bytes_transferred" { Write-Output $data.bytes_transferred }
    "bytes_reused" { Write-Output $data.bytes_reused }
    "hostname"     { Write-Output $data.hostname }
    "job_name"     { Write-Output $data.job_name }
    "error"        { Write-Output $data.error }
    default        { Write-Output "" }
}
