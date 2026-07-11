# ClickRAX — руководство администратора

[Русский](admin-guide-ru.md) · [English](admin-guide-en.md)

## Установка

1. Установите через NSIS-инсталлятор (`clickrax-amd64-installer.exe`) из [Releases](https://github.com/clickrax/clickrax/releases) или скопируйте `clickrax.exe`.
2. Для службы: **Настройки → Установить службу Windows** (от администратора).
3. Или вручную: `clickrax.exe -install-service`

Служба Windows зарегистрирована как **`PbsWinBackup`** (legacy-имя для совместимости с существующими установками).

## Первичная настройка PBS

1. **Серверы** → Добавить назначение → тип PBS:
   - URL: `https://pbs.example.com:8007`
   - Datastore: имя datastore на PBS (например `backup`)
   - Namespace: имя хоста Windows или пусто
   - Token ID: `user@pbs!token-name`
   - Secret: значение токена из PBS (хранится в DPAPI, не в config)
   - Fingerprint: кнопка «Получить fingerprint»

2. ACL на PBS для namespace: `DatastoreBackup` + `DatastoreAudit` (без prune/forget на production-токене).

## Задание бэкапа

1. **Задания** → Новое:
   - Источники: пути для бэкапа (например `D:\`)
   - Backup ID: обычно hostname Windows
   - VSS: включён для открытых файлов
   - Расписание: например 02:00

2. «Запустить сейчас» для проверки.

## Восстановление

- **Файл:** выберите в списке → восстановление в оригинал или в каталог.
- **Папка:** выберите каталог → восстановление папки.

## Полный бэкап

Кнопка **Полный** очищает локальный индекс chunks. CLI: `--force-full`.

## Инкременты и prune на PBS

См. [prune-and-increments.md](prune-and-increments.md) и [fast-pbs-incremental.md](fast-pbs-incremental.md).

## Zabbix

Файл: `%ProgramData%\ClickRAX\last_status.json`  
Скрипт: `scripts\zabbix-read-status.ps1`

## CLI

```powershell
clickrax-cli.exe status
clickrax-cli.exe test --server-id <uuid>
clickrax-cli.exe backup --job-id <uuid>
clickrax-cli.exe backup --job-id <uuid> --force-full
clickrax-cli.exe restore --job-id <uuid> --file "D:\Data\file.docx"
clickrax-cli.exe restore --job-id <uuid> --folder "D:\Data\Projects"
```

## Сборка из исходников

```powershell
.\build.ps1              # exe + CLI
.\build.ps1 -Installer   # NSIS installer (нужен makensis)
```

Требования: Windows, Go 1.26+, Node.js, Wails CLI, Python 3. См. [CONTRIBUTING.md](../CONTRIBUTING.md).

## Данные приложения

| Путь | Содержимое |
|------|------------|
| `%ProgramData%\ClickRAX\config.json` | Серверы, задания, настройки (без паролей) |
| `%ProgramData%\ClickRAX\config.json.hmac` | HMAC-подпись конфигурации |
| `%ProgramData%\ClickRAX\secrets\user\` | DPAPI-секреты GUI |
| `%ProgramData%\ClickRAX\secrets\service\` | DPAPI-секреты службы |
| `%ProgramData%\ClickRAX\index\` | Локальные индексы |
| Windows Credential Manager | Миграция legacy-секретов (префикс `PbsWinBackup:`) |

Старые установки с `%ProgramData%\PbsWinBackup\` подхватываются автоматически.

## Безопасность

См. [SECURITY.md](../SECURITY.md) — threat model, ограничения при захвате хоста, рекомендации по PBS/SMB.

## Язык интерфейса

GUI поддерживает **русский** и **английский**. Переключение: **Настройки → Язык**.
