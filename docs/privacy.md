# Privacy Policy / Политика конфиденциальности

**Languages:** [English](#english) · [Русский](#русский)  
**Repository:** https://github.com/clickrax/clickrax

---

<a id="english"></a>

## English

ClickRAX is a local Windows backup client. It runs on your machine and connects only to destinations you configure.

### Data collection

ClickRAX **does not** collect analytics, telemetry, or usage statistics. There is no phone-home to ClickRAX or third-party analytics services.

### Network connections

| Destination | Purpose | Data sent |
|-------------|---------|-----------|
| Proxmox Backup Server (HTTPS) | Backup and restore | Backup data, API token |
| SMB / FTP (user-configured) | Archive backup | Backup archives |
| Webhook URL (optional) | Job status notifications | Job name, status, error text |
| SMTP server (optional) | Email notifications | Job status summary |
| GitHub API (optional) | Update check if enabled at build time | Repository name only |

### Local data

Under `%ProgramData%\ClickRAX\`:

- `config.json` — jobs and settings (no passwords)
- `secrets\` — DPAPI-encrypted credentials
- `index\` — per-job backup indexes
- `logs\` — application logs

### SmartScreen

Unsigned or newly signed binaries may show Windows SmartScreen warnings. Signed releases via SignPath Foundation display publisher **SignPath Foundation**.

---

<a id="русский"></a>

## Русский

ClickRAX — локальный клиент резервного копирования Windows. Работает на вашей машине и подключается только к настроенным вами назначениям.

### Сбор данных

ClickRAX **не** собирает аналитику, телеметрию и статистику использования. Нет «phone-home» на серверы ClickRAX или сторонней аналитики.

### Сетевые подключения

| Назначение | Назначение | Передаваемые данные |
|------------|------------|---------------------|
| Proxmox Backup Server (HTTPS) | Бэкап и restore | Данные бэкапа, API-токен |
| SMB / FTP (настраивается) | Архивный бэкап | ZIP-архивы |
| Webhook (опционально) | Уведомления о заданиях | Имя задания, статус, ошибка |
| SMTP (опционально) | Email-оповещения | Сводка по заданию |
| GitHub API (опционально) | Проверка обновлений при сборке | Только имя репозитория |

### Локальные данные

В `%ProgramData%\ClickRAX\`:

- `config.json` — задания и настройки (без паролей)
- `secrets\` — зашифрованные DPAPI креды
- `index\` — локальные индексы
- `logs\` — журнал приложения

### SmartScreen

Неподписанные или новые бинарники могут вызывать предупреждение SmartScreen. Подписанные релизы через SignPath Foundation показывают издателя **SignPath Foundation**.
