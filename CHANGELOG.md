# Changelog / История изменений

[English](#english) · [Русский](#русский)

---

<a id="english"></a>

## English

**2.3 is the first public release.** Versions 2.0–2.2 were used privately on real Windows PCs and local PBS servers before the repo went public.

Started in 2018: new **HP ProLiant DL380 Gen9** (~**$48k** at 2018 FX), StoreOnce 14 TB licensed / 40 TB disks — HP wanted almost the full server price to unlock capacity; controller swap instead. Then PBS; **PbsWinBackup** → **ClickRAX** after the vendor quote and because the Windows CLI client wasn't enough day to day.

### [2.3.6] — 2026-07-12

Journal and e-mail notification fixes:

- Successful backups no longer show «with warning» just because fast incremental skipped unchanged files
- Journal details explain how many files were skipped; real warnings (e.g. verify timeout) stay visible
- E-mail notifications auto-enable when SMTP is configured (was silently off by default)
- Toast and event log when e-mail delivery fails

### [2.3.5] — 2026-07-12

PBS backup progress during finalization:

- Show detailed finalize stages (76–97%) after data transfer instead of freezing at 75%
- Status messages for PXAR/catalog close, manifest, Finish, and local index saves
- Immediate UI update when each finalize step starts

### [2.3.4] — 2026-07-11

PBS fast incremental fixes:

- Fix local index reuse: compare chunk span bytes to PXAR stream length, not raw file size
- Store ACL hash in index so Windows files are not re-read every run
- Enable fast cache after first indexed file (was 100-file minimum)
- CI: correct go vet flag for Windows syscall bindings

### [2.3.3] — 2026-07-11

Stability and GUI fixes:

- Production GUI: Vite embed paths and WebView2 asset loading
- Config: read without blocking on service lock; auto-recover from backup copies
- HMAC integrity no longer wipes servers on startup

### [2.3] — 2026-07-11

First GitHub release as **ClickRAX**.

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

### [2.3.6] — 2026-07-12

Журнал и e-mail уведомления:

- Успешный бэкап больше не помечается «с предупреждением» только из‑за пропущенных файлов fast-incremental
- В журнале видно, сколько файлов пропущено; настоящие предупреждения (verify timeout и т.п.) сохраняются
- E-mail уведомления включаются автоматически при настроенном SMTP (раньше по умолчанию было «выкл.»)
- Toast и запись в Event Log при ошибке отправки письма

### [2.3.5] — 2026-07-12

Прогресс PBS-бэкапа на этапе финализации:

- Подробные статусы (76–97%) после передачи данных вместо «зависания» на 75%
- Сообщения для закрытия PXAR/каталога, manifest, Finish и сохранения локальных индексов
- Мгновенное обновление UI при смене этапа

### [2.3.4] — 2026-07-11

Исправления быстрого PBS-инкремента:

- Локальный индекс: сравнение spans с длиной PXAR-потока, а не с размером файла
- Сохранение ACL hash в индексе — Windows-файлы не перечитываются каждый раз
- Кэш fast-incremental включается после первого проиндексированного файла (раньше нужно было 100)
- CI: исправлен флаг go vet для Windows syscall

### [2.3.3] — 2026-07-11

Исправления стабильности и GUI:

- GUI в production: пути Vite и загрузка assets в WebView2
- Конфиг: чтение без блокировки службой; восстановление из резервных копий
- HMAC больше не обнуляет серверы при старте

### [2.3] — 2026-07-11

Первый релиз на GitHub под именем **ClickRAX**.

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
