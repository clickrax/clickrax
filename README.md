# ClickRAX

**Автор / Author:** John Watson · Telegram [@Johnwatson7777](https://t.me/Johnwatson7777)  
**Языки / Languages:** [Русский](README.md) · [English](README.en.md)

Резервное копирование Windows на **Proxmox Backup Server** — с нормальными инкрементами, VSS и восстановлением файлов из снапшота без танцев с `proxmox-backup-client` вручную.

Помимо PBS поддерживаются **SMB** и **FTP/FTPS**: туда уходит ZIP-архив с сохранением ACL и атрибутов NTFS. Для домашних серверов и малых инфраструктур, где PBS уже стоит, а Windows-машины нужно подключить без лишней возни.

**Текущая версия: 2.3.8** — [clickrax.exe](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax.exe) · [clickrax-cli.exe](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-cli.exe) · [ZIP](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-windows-amd64-portable.zip) · [установщик](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-amd64-installer.exe) · [папка release/v2.3](https://github.com/clickrax/clickrax/tree/main/release/v2.3)

> **Скачивание exe:** готовые файлы — в папке **[release/v2.3/](https://github.com/clickrax/clickrax/tree/main/release/v2.3)** (ссылки выше).

---

## Скриншоты

Экран **«Выполнение»** во время PBS-бэкапа: прогресс, скорость, chunks, очередь и служба. Интерфейс на русском и английском.

![ClickRAX — выполнение бэкапа на PBS (RU)](docs/screenshots/progress-ru.png)

![ClickRAX — backup progress to PBS (EN)](docs/screenshots/progress-en.png)

---

## Откуда взялся ClickRAX

В 2018 купил новый **HP ProLiant DL380 Gen9** под бэкапы — вышло около **$48 000** по курсу того года. StoreOnce, лицензия на **14 ТБ**, дисков набрал на **40**. HP за разблокировку места выставили счёт почти на всю стоимость сервера — платить второй раз за то, что уже стоит в коробке, смысла не было. Поменял контроллер, получил все 40 ТБ.

После такого ценника на лицензию стало ясно: дальше либо своё, либо снова в такие же истории. StoreOnce ещё поработал, потом поставил **PBS**. Windows к нему цеплять надо, а `proxmox-backup-client` из консоли на каждый день — так себе. Решил написать своё: **PbsWinBackup**, потом GUI, службу, нормальные инкременты — **ClickRAX**.

На GitHub выложил только сейчас. До этого годами крутилось у себя — сервер, рабочие ПК, локальный PBS и SMB.

---

## Почему ClickRAX, а не proxmox-backup-client

Для Windows Proxmox даёт только **консольный** `proxmox-backup-client`: токены, скрипты, cron/Планировщик, restore — руками из терминала.

**ClickRAX — настольный GUI** (RU/EN) поверх нативного формата PBS (PXAR, chunk dedup, verify):

- окно настройки PBS, fingerprint, задания и расписание;
- служба Windows — бэкап без открытого сеанса;
- restore файла или папки из каталога снапшота;
- быстрый file-level инкремент (v2.3+) — не перечитывает весь том каждый раз;
- плюс SMB/FTP, CLI, Zabbix.

Open-source альтернатив с **полноценным GUI именно под PBS на Windows** по сути нет — остаются скрипты вокруг CLI или бэкап «куда-то ещё», не в datastore PBS.

Если ищете *Windows PBS backup GUI*, *Proxmox Backup Server graphical client*, *клиент PBS с интерфейсом* — это сюда.

---

## Зачем это нужно

Стандартный путь «поставить PBS и бэкапить Windows» обычно упирается в одно из двух: либо тащить всё через сторонние скрипты, либо мириться с тем, что инкремент на PBS всё равно читает весь диск часами. ClickRAX заточен под повседневную работу админа: GUI, служба Windows, расписание, восстановление отдельного файла или папки из каталога снапшота.

На PBS используется родной формат (PXAR + chunk dedup). С версии 2.3 добавлен **быстрый file-level инкремент**: неизменённые файлы на диске не перечитываются — сравнение по размеру и mtime, переиспользование chunks с сервера. На больших томах (сотни ГБ и выше) это принципиально меняет время второго и последующих прогонов. Первый проход после обновления по-прежнему строит локальный индекс; зато он метаданные, без копирования всего тома на системный диск.

---

## Что умеет

**Бэкап на PBS**
- Полный и инкрементальный бэкап каталогов и целых томов
- VSS для открытых и заблокированных файлов
- Chunk-level dedup на стороне PBS (видно, сколько передано и сколько переиспользовано)
- Проверка снапшота на PBS после бэкапа (verify)
- Исключения: свои маски + автоматический пропуск `$RECYCLE.BIN`, `System Volume Information`, pagefile и т.п. при бэкапе тома
- Ограничение скорости, очередь заданий, чекпоинты при сбоях

**Бэкап на SMB / FTP**
- Инкрементальные ZIP-архивы с цепочкой полных/инкрементальных снимков
- Метаданные NTFS (ACL, владелец, времена) в манифесте

**Восстановление**
- Один файл или целая папка из снапшота PBS
- Восстановление «как было» или в указанный каталог
- Просмотр каталога снапшота в интерфейсе

**Автоматизация**
- Служба Windows — бэкапы по расписанию без открытого сеанса пользователя
- Расписание: ежедневно (несколько времён в сутки), еженедельно, раз в две недели, ежемесячно
- Отдельное расписание полного бэкапа (еженедельно / раз в 2 недели / ежемесячно / только вручную)
- CLI для скриптов и планировщика задач

**Уведомления и мониторинг**
- `last_status.json` для Zabbix (готовый PowerShell-скрипт в `scripts/`)
- Webhook (JSON с результатом задания)
- SMTP-оповещения

**Безопасность**
- Secret-токен PBS и пароли — в Windows Credential Manager и DPAPI (в конфиге их нет)
- PBS только по HTTPS, опциональная привязка к fingerprint сертификата
- Секреты GUI и службы хранятся в разных DPAPI-областях (`secrets/user/` и `secrets/service/`)
- Целостность `config.json` — HMAC-подпись (`config.json.hmac`)

> **Важно:** ClickRAX — клиент бэкапа, а не защита от ransomware. При захвате хоста атакующий может украсть креды и удалить удалённые бэкапы. Для устойчивости используйте off-host хранилище, least-privilege PBS-токены и immutable/append-only SMB. Подробнее: [SECURITY.md](SECURITY.md).

Интерфейс **на русском и английском** (переключается в настройках). Есть GUI (Wails + Vue) и консольная утилита `clickrax-cli`.

---

## Системные требования

- Windows 10 / 11 или Windows Server 2016+
- [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (для GUI; на Win11 обычно уже есть)
- Proxmox Backup Server 2.x / 3.x с API-токеном — для режима PBS
- Права администратора — для установки службы и VSS

Для сборки из исходников: Go 1.26+, Node.js LTS, [Wails v2 CLI](https://wails.io/) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`), Python 3 (иконка), при установщике — NSIS. **Только Windows** — см. [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Установка

### Из релиза (рекомендуется)

**Установщик:** [clickrax-amd64-installer.exe](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-amd64-installer.exe) — Program Files, опционально служба.

**Портативно:** [clickrax.exe](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax.exe) и [clickrax-cli.exe](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-cli.exe), или [ZIP с обоими](https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-windows-amd64-portable.zip).

Установщик кладёт файлы в Program Files и при необходимости ставит службу. Портативный вариант — положить `clickrax.exe` куда угодно и запустить.

### Сборка самому

```powershell
git clone https://github.com/clickrax/clickrax.git
cd clickrax
.\build.ps1
```

Готовые файлы: `build\bin\clickrax.exe`, `build\bin\clickrax-cli.exe`.

Установщик NSIS:

```powershell
.\build.ps1 -Installer
# build\bin\clickrax-amd64-installer.exe
```

> GUI собирается через `wails build`. Обычный `go build` для основного окна не подходит — не подтянется фронтенд.

---

## Первый запуск

### 1. Назначение PBS

**Серверы** → **Добавить назначение** → тип PBS.

| Поле | Пример |
|------|--------|
| URL | `https://pbs.example.com:8007` |
| Datastore | `backup` |
| Namespace | имя хоста Windows (или пусто) |
| Token ID | `backup@pbs!win-host` |
| Secret | значение из PBS → Access → API token |

Нажмите **Получить fingerprint** и сохраните сертификат — так клиент не доверяется слепо всем CA в системе.

На PBS для этого namespace нужны права **DatastoreBackup** и **DatastoreAudit**.

### 2. Задание бэкапа

**Задания** → **Новое задание**.

- **Источник:** весь том (`D:\`) или выбранные папки
- **Backup ID:** обычно hostname машины (как в PBS)
- **VSS:** включить для томов с открытыми файлами (БД, почта, 1С)
- **Проверять после бэкапа:** имеет смысл оставить включённым
- **Расписание:** время и дни недели

Для проверки нажмите **Запустить**. На вкладке **Выполнение** видно фазы, скорость, сколько chunks новых и переиспользованных.

### 3. Служба Windows

Чтобы бэкапы шли по расписанию без входа в систему:

**Настройки** → **Установить службу** (от имени администратора).

Служба использует копию exe из `%ProgramData%\ClickRAX\bin\`. После обновления программы нажмите **Обновить службу**, если ставили новую версию вручную.

---

## Восстановление

**Восстановление** → выберите задание и снапшот → отметьте файл или папку. Работает и для PBS, и для SMB/FTP.

Восстанавливаются **отдельные файлы и папки**, не весь диск и не bare-metal.

- **В оригинал** — вернёт на те же пути, откуда бэкапились (нужен тот же диск/буква)
- **В каталог** — если машину переустанавливали или диск другой, укажите папку

На SMB/FTP для инкрементального снапшота на сервере должна быть **вся цепочка** (полный + инкременты до выбранной точки). Старые архивы из середины цепочки не удаляйте, если может понадобиться откат.

ACL и времена NTFS восстанавливаются, если они попали в бэкап (метаданные PBS / sidecar на SMB/FTP).

При конфликте с существующим файлом программа спросит, перезаписывать или нет.

Через CLI:

```powershell
clickrax-cli restore --job-id <uuid> --file "Projects\report.xlsx"
clickrax-cli restore --job-id <uuid> --folder "Projects\2024"
```

---

## Расписание и полный бэкап

| Режим | Когда срабатывает |
|-------|-------------------|
| Ежедневно | В указанные часы каждый день |
| Еженедельно | В выбранные дни недели |
| Раз в 2 недели | От якорной даты |
| Ежемесячно | Раз в месяц |

Полный бэкап на PBS можно вынести на отдельный график (например, инкременты каждую ночь, полный — по воскресеньям). Кнопка **Полный** в списке заданий или `clickrax-cli backup --job-id ... --force-full` сбрасывает локальные индексы и гоняет полный проход.

Если в момент запуска уже идёт другой бэкап, задание встаёт в очередь (или пропускается — настраивается в задании).

---

## Консольные команды

```powershell
clickrax-cli status
clickrax-cli test --server-id <uuid>
clickrax-cli backup --job-id <uuid>
clickrax-cli backup --job-id <uuid> --force-full
clickrax-cli restore --job-id <uuid> --file "path\in\snapshot"
clickrax-cli restore --job-id <uuid> --folder "path\to\folder"
```

Полезно для CI, Ansible или ручного запуска с сервера без GUI.

---

## Где лежат данные

| Путь | Содержимое |
|------|------------|
| `%ProgramData%\ClickRAX\config.json` | Серверы, задания, настройки (без паролей) |
| `%ProgramData%\ClickRAX\secrets\user\` | DPAPI-секреты GUI (CurrentUser) |
| `%ProgramData%\ClickRAX\secrets\service\` | DPAPI-секреты службы (LocalMachine) |
| `%ProgramData%\ClickRAX\config.json.hmac` | HMAC-подпись конфигурации |
| `%ProgramData%\ClickRAX\index\<job-id>\` | Локальные индексы chunks и fast-incremental |
| `%ProgramData%\ClickRAX\logs\` | Журнал |
| `%ProgramData%\ClickRAX\last_status.json` | Статус последнего запуска (Zabbix) |
| Windows Credential Manager | Secret PBS, пароли SMB/FTP, SMTP |

Старые установки с каталогом `%ProgramData%\PbsWinBackup\` подхватываются автоматически — миграция не нужна.

---

## Zabbix

Файл `last_status.json` обновляется после каждого завершённого задания. Пример опроса — `scripts\zabbix-read-status.ps1`.

Поля: hostname, имя задания, статус (`ok` / `warning` / `error`), тип бэкапа, объём переданных и переиспользованных байт, текст ошибки.

---

## Документация

- [Руководство администратора (RU)](docs/admin-guide-ru.md)
- [Administrator guide (EN)](docs/admin-guide-en.md)
- [Инкременты и prune на PBS](docs/prune-and-increments.md)
- [Как устроен быстрый PBS-инкремент](docs/fast-pbs-incremental.md)
- [Конфиденциальность (RU/EN)](docs/privacy.md)
- [Политика подписи кода](docs/code-signing-policy.md)
- [Безопасность (RU/EN)](SECURITY.md)
- [Участие в разработке (RU/EN)](CONTRIBUTING.md)
- [История изменений (RU/EN)](CHANGELOG.md)
- [English README](README.en.md)

---

## Сборка и разработка

```powershell
.\build.ps1              # exe + CLI
.\build.ps1 -Installer   # NSIS (winget install NSIS.NSIS)
go test ./...            # тесты
```

Стек: Go, Wails 2, Vue 3, TypeScript. Протокол PBS — на базе [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) (GPL-3.0, vendored в `third_party/`).

---

## Поддержать проект

ClickRAX бесплатен для скачивания с GitHub Releases. Передавать третьим лицам или выкладывать зеркала **без моего согласия нельзя** — см. [LICENSE](LICENSE).

**Telegram:** [@Johnwatson7777](https://t.me/Johnwatson7777) — вопросы, связь, донат через Telegram Wallet.

| Монета | Сеть | Адрес |
|--------|------|-------|
| GRAM (Telegram Wallet, раньше TON) | TON | `UQAPBTCRimI4qOEyFNwoyhwU5ipE5HjnXT0VUkGSCfJ7olzo` |
| USDT | Ethereum (ERC-20) | `0x89bd0a764a04234516eaa2b693a5426bc6f60b97` |
| BTC | Bitcoin | `bc1q6rncvfsn0eav7lrkcnecf52vf9te0s7kcqshvd` |

Переводите **строго в указанной сети** — USDT в ERC-20, не TRC-20 и не BEP-20.  
English: [README.en.md](README.en.md#support-the-project)

---

## Лицензия и авторские права

**Автор:** John Watson  
**Telegram:** [@Johnwatson7777](https://t.me/Johnwatson7777)

ClickRAX — объект авторского права John Watson. **Запрещено** копировать, распространять или передавать программу третьим лицам **без письменного согласия автора**. Подробности: [LICENSE](LICENSE).

Сторонний код PBS-клиента (`third_party/proxmoxbackupclient_go-master/`) распространяется под GPL-3.0 — см. `third_party/proxmoxbackupclient_go-master/LICENSE`.

---

## Участие

**Баги и косяки** — [GitHub Issues](https://github.com/clickrax/clickrax/issues) или Telegram [@Johnwatson7777](https://t.me/Johnwatson7777). Тестировщикам буду рад: гоняйте на своих Windows/PBS, пишите что сломалось — буду стараться чинить.

Предложения и pull requests — см. [CONTRIBUTING.md](CONTRIBUTING.md).

Уязвимости безопасности — **не** через публичные Issues; см. [SECURITY.md](SECURITY.md) (Advisories или Telegram).

Пожалуйста, не коммитьте в репозиторий реальные URL PBS, токены, fingerprint и IP внутренней сети. Пример схемы конфига: [config.json.example](config.json.example).

Первые релизы на GitHub — **без code signing** (exe без цифровой подписи). Подпись планируется позже, см. [docs/code-signing-policy.md](docs/code-signing-policy.md).
