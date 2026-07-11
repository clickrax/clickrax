# ClickRAX — Administrator Guide (English)

**Languages:** [English](admin-guide-en.md) · [Русский](admin-guide-ru.md)  
**Repository:** https://github.com/clickrax/clickrax

## Installation

1. Install via NSIS installer (`clickrax-amd64-installer.exe`) from [Releases](https://github.com/clickrax/clickrax/releases) or copy `clickrax.exe` manually.
2. For the Windows service: **Settings → Install Windows service** (as administrator).
3. Or manually: `clickrax.exe -install-service`

The service is registered as **`PbsWinBackup`** (legacy name for compatibility with existing installs).

## PBS setup

1. **Servers** → **Add destination** → type PBS:
   - URL: `https://pbs.example.com:8007`
   - Datastore: e.g. `backup`
   - Namespace: Windows hostname or empty
   - Token ID: `user@pbs!token-name`
   - Secret: token value from PBS (stored in DPAPI, not in config)
   - Fingerprint: **Get fingerprint** button

2. PBS ACL for the namespace: `DatastoreBackup` + `DatastoreAudit` (no prune/forget on production tokens).

## Backup job

1. **Jobs** → **New** — sources, Backup ID, VSS, verify, schedule.
2. Click **Run** to test.

## Restore

Select job and snapshot → file or folder → original path or target directory.

## Full backup

**Full** button or CLI: `--force-full` resets local indexes.

## PBS increments & prune

See [prune-and-increments.md](prune-and-increments.md) and [fast-pbs-incremental.md](fast-pbs-incremental.md).

## Zabbix

File: `%ProgramData%\ClickRAX\last_status.json`  
Script: `scripts\zabbix-read-status.ps1`

## CLI

```powershell
clickrax-cli.exe status
clickrax-cli.exe test --server-id <uuid>
clickrax-cli.exe backup --job-id <uuid>
clickrax-cli.exe backup --job-id <uuid> --force-full
clickrax-cli.exe restore --job-id <uuid> --file "D:\Data\file.docx"
clickrax-cli.exe restore --job-id <uuid> --folder "D:\Data\Projects"
```

## Build from source

```powershell
.\build.ps1              # exe + CLI
.\build.ps1 -Installer   # NSIS installer
```

Requirements: Windows, Go 1.26+, Node.js, Wails CLI, Python 3. See [CONTRIBUTING.md](../CONTRIBUTING.md).

## Application data

| Path | Content |
|------|---------|
| `%ProgramData%\ClickRAX\config.json` | Servers, jobs, settings (no passwords) |
| `%ProgramData%\ClickRAX\config.json.hmac` | Config HMAC signature |
| `%ProgramData%\ClickRAX\secrets\user\` | GUI DPAPI secrets |
| `%ProgramData%\ClickRAX\secrets\service\` | Service DPAPI secrets |
| `%ProgramData%\ClickRAX\index\` | Local indexes |
| Windows Credential Manager | Legacy secret migration (`PbsWinBackup:` prefix) |

Legacy `%ProgramData%\PbsWinBackup\` is migrated automatically.

## Security

See [SECURITY.md](../SECURITY.md) — threat model, host compromise limits, PBS/SMB recommendations.

## UI language

The GUI supports **English** and **Russian**. Switch in **Settings → Language**.
