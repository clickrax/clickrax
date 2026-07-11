# Changelog / История изменений

**Languages:** [English](#english) · [Русский](#русский)  
**Repository:** https://github.com/clickrax/clickrax

---

<a id="english"></a>

## English

**2.3 is the first public release.** Versions 2.0–2.2 were used privately on real Windows PCs and local PBS servers before the repo went public.

Started in 2018: new **HP ProLiant DL380 Gen9** (~**$48k** at 2018 FX), StoreOnce 14 TB licensed / 40 TB disks — HP wanted almost the full server price to unlock capacity; controller swap instead. Then PBS; **PbsWinBackup** → **ClickRAX** after the vendor quote and because the Windows CLI client wasn't enough day to day.

### [2.3.3] — 2026-07-11

Post-release fixes before public launch:

- Production GUI: Vite embed paths and WebView2 asset loading
- Config: read without blocking on service lock; auto-recover from quarantine backups
- HMAC integrity no longer wipes servers on startup

### [2.3] — 2026-07-11 (public)

First public release as **ClickRAX**.

- Fast PBS incremental: unchanged files are not re-read (size + mtime, chunk reuse from server)
- GUI and Windows service share the same backup runner
- DPAPI secrets split for user session vs service (`secrets/user/`, `secrets/service/`)
- Config HMAC (`config.json.hmac`)
- Cancel a running backup from GUI while the service holds the job
- Docs and UI in Russian and English

### [2.2] — 2025-11 (private)

Used in production at home and on a few office PCs. Project name was **PbsWinBackup**.

- PBS incrementals with chunk dedup, but each run still read the whole volume
- SMB/FTP incremental ZIP + NTFS sidecar metadata
- Windows service, schedules, VSS, CLI
- Restore: pick a file or folder from PBS snapshot or SMB/FTP chain

### [2.1] — 2025-08 (private)

- Stable PBS path: volumes and folder selections, verify after backup
- Secrets in Credential Manager, data under `%ProgramData%\PbsWinBackup\` (still auto-migrated)

### [2.0] — 2025-06 (private)

- First version that actually backed up to PBS on a schedule
- Tested on Windows 10/11 against PBS 3.x on a home server

### Before 2.0

Scripts and experiments that grew into the client. Nothing was published.

---

<a id="русский"></a>

## Русский

**2.3 — первый публичный релиз.** Версии 2.0–2.2 несколько лет крутились приватно на своих ПК и локальных PBS, потом выложили на GitHub.

С 2018: новый **HP ProLiant DL380 Gen9** (~**$48k** по курсу 2018), StoreOnce 14 ТБ / 40 ТБ дисков — HP за разблокировку места выставили почти цену сервера, обошлись сменой контроллера. Потом PBS; **PbsWinBackup** → **ClickRAX** — и после такого ценника, и потому что консольного клиента на Windows мало.

### [2.3.3] — 2026-07-11

Исправления перед публикацией:

- GUI в production: пути Vite и загрузка assets в WebView2
- Конфиг: чтение без блокировки службой; восстановление из quarantine-бэкапов
- HMAC больше не обнуляет серверы при старте

### [2.3] — 2026-07-11 (публичный)

Первый публичный релиз под именем **ClickRAX**.

- Быстрый PBS-инкремент: неизменённые файлы не перечитываются (size + mtime, chunks с сервера)
- GUI и служба Windows используют один и тот же код бэкапа
- DPAPI-секреты разделены для пользователя и службы
- HMAC конфигурации (`config.json.hmac`)
- Отмена бэкапа из GUI, когда работает служба
- Документация и интерфейс на русском и английском

### [2.2] — 2025-11 (приватная)

Гоняли на домашних машинах и паре рабочих ПК. Имя проекта — **PbsWinBackup**.

- PBS-инкременты с dedup, но том всё равно читался целиком
- SMB/FTP — инкрементальные ZIP с метаданными NTFS
- Служба, расписание, VSS, CLI
- Восстановление файла или папки из PBS или цепочки SMB/FTP

### [2.1] — 2025-08 (приватная)

- Стабильный бэкап на PBS: тома и папки, verify после прогона
- Секреты в Credential Manager, данные в `%ProgramData%\PbsWinBackup\`

### [2.0] — 2025-06 (приватная)

- Первая версия, которая реально бэкапила на PBS по расписанию
- Тесты на Windows 10/11 + PBS 3.x дома

### До 2.0

Скрипты и наброски, из которых вырос клиент. Публичных релизов не было.
