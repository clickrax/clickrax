# Developer CLI tools / Утилиты для разработчиков

**Repository:** https://github.com/clickrax/clickrax

These programs are for **local development and troubleshooting only**. They are not built or distributed in release packages.

| Tool | Purpose |
|------|---------|
| `cmd/checkcred` | Check if a destination secret exists and print its length |
| `cmd/migratecred` | Migrate credentials from Credential Manager to DPAPI files |
| `cmd/testbackup` | Run a backup against live local config |
| `cmd/testverify` | Verify PBS backup access with live credentials |

**English:** Build with `go build -o tool.exe .\cmd\<name>`. Reads secrets from `%ProgramData%\ClickRAX\` on the machine where they run.

**Русский:** Сборка: `go build -o tool.exe .\cmd\<name>`. Читают секреты из `%ProgramData%\ClickRAX\` на локальной машине.

User-facing CLI: `cmd/cli` → `clickrax-cli.exe` (built by `build.ps1`).
