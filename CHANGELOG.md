# Changelog / История изменений

[English](#english) · [Русский](#русский)

---

<a id="english"></a>

## English

**2.3 is the first public release.** Versions 2.0–2.2 were used privately on real Windows PCs and local PBS servers before the repo went public.

Started in 2018: new **HP ProLiant DL380 Gen9** (~**$48k** at 2018 FX), StoreOnce 14 TB licensed / 40 TB disks — HP wanted almost the full server price to unlock capacity; controller swap instead. Then PBS; **PbsWinBackup** → **ClickRAX** after the vendor quote and because the Windows CLI client wasn't enough day to day.

### [2.3.16] — 2026-07-13

Audit hotspot fixes (v2.3.15 follow-up):

- Passphrase lazy migration and bulk migrate no longer delete WinCred until DPAPI write succeeds (prevents loss of encryption keys)
- Zip archive: open source file before creating zip entry (no phantom 0-byte entries on access errors)
- Backup lock heartbeat during long runs; stale locks with missing/invalid timestamp expire by age; compare-and-delete before removing stale locks (backuplock + datalock)
- Schedule state: single read-modify-write under datalock; JSON parse errors no longer ignored
- Last status: `warning` runs update `last_success` like `ok`
- Legacy config migration uses durable atomic writes
- Pxar restore aborts partial payload on error; history append errors logged; net connections close on context cancel

### [2.3.15] — 2026-07-13

Tray while backup is running:

- Left-click show window no longer blocks the tray message loop (fixes frozen tray during active backup)
- Tooltip updates throttled during progress to reduce systray contention

### [2.3.14] — 2026-07-13

Security and reliability audit fixes:

- Re-verify known PBS chunks before reuse (session cache, one probe per digest); re-upload when data is available, clear error after server prune when not
- Config HMAC mismatch no longer silently re-signs tampered `config.json`
- Quarantine auto-recovery validates webhook and PBS URLs before restore
- **Stop backup** always stops the local engine (fixes stale job ID in UI)
- `LoadResilient()` used when reloading config for backups, schedule, import, and service queue
- PBS HTTP/2 upgrade: write error checks and 5-minute deadline; `AssignFixedChunks` sends Authorization
- Credential migration removes legacy WinCred copies; passphrase dual-write rolls back on service-scope failure
- Backup retry backoff respects cancel context; webhook response bodies drained

### [2.3.13] — 2026-07-13

System tray on window close:

- Setting **Minimize to tray when closing** (default on): closing the window hides to notification area; backups and schedule keep running
- Tray context menu (right-click): all main sections including Settings, plus Exit
- Left-click tray icon opens the window; menu only on right-click
- Tray tooltip shows idle / running job with phase and percent while minimized
- Tray starts only when the window is first hidden (avoids systray/Wails startup crash)

### [2.3.12] — 2026-07-13

Fix PBS backup task stuck in «running» after successful finish:

- Close the upgraded HTTP/2 backup connection after `POST /finish` (PBS commits the snapshot on finish but keeps the worker task open until the client disconnects)

### [2.3.11] — 2026-07-13

Reliability and incremental backup fixes (audit follow-up):

- Fix PBS API HTTP error handling (`CreateDynamicIndex`, `Finish`, `CloseFixedIndex`, `AssignFixedChunks`, body leaks)
- Stop per-chunk `GET /chunk` probes — trust previous index (eliminates millions of 404s on large backups)
- Fall back to local `chunks.json` when PBS previous index is missing; do not wipe local index on transient PBS errors
- Narrow `previousIndexUnavailable` — network/auth errors no longer force full re-upload
- After successful PBS `Finish`, local index save failures are warnings (snapshot already committed)
- UI: show active service/checkpoint backup instead of stale terminal progress; warning status toast; restore email errors surfaced
- `ConfigSnapshot()` returns a clone (no data race)

### [2.3.10] — 2026-07-13

Fix PBS `Finish` on large backups:

- Do not upload WinMeta / PXAR file index blobs to PBS (local-only; matches official PBS client flow)
- Avoids `finish HTTP 400: unable to update manifest blob - Invalid string length` on multi-million-file backups

### [2.3.9] — 2026-07-13

Fix PBS blob upload file names:

- Rename `backup.winmeta.json` → `backup.winmeta.blob` and `backup.pxar.index.json` → `backup.pxar.index.blob` (PBS requires `.blob` extension)
- Restore downloads try legacy `.json` names for older snapshots

### [2.3.8] — 2026-07-13

PBS finalize fixes for very large backups (millions of files):

- Fix `UploadBlob` ignoring HTTP errors (false success e-mail, PBS task stuck open)
- Skip `backup.winmeta.json` / `backup.pxar.index.json` PBS upload when payload exceeds ~16 MiB (PBS limit); warn and keep local copy
- Cache chunk existence probes — one `GET /chunk` per digest instead of millions of 404s on reuse

### [2.3.7] — 2026-07-12

Fix false error / stuck UI after successful PBS backup:

- Stop progress ticker before post-backup finalize so UI is not overwritten at 97%
- Local chunk index save failure is a warning, not a failed backup (PBS snapshot already committed)
- Execution view ignores stale checkpoints after a terminal done event

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

### [2.3.16] — 2026-07-13

Исправления по аудиту (продолжение 2.3.15):

- Ленивая миграция и массовая миграция passphrase не удаляют WinCred до успешной записи DPAPI (защита ключей шифрования)
- Zip: открытие файла до создания записи в архиве (нет фантомных 0-байтных записей при ошибках доступа)
- Heartbeat lock-файла при длинном бэкапе; устаревшие lock с битым timestamp истекают по возрасту; compare-and-delete (backuplock + datalock)
- Состояние расписания: один RMW под datalock; ошибки JSON больше не игнорируются
- Last status: `warning` обновляет `last_success` как `ok`
- Миграция legacy-конфига через durable atomic write
- Pxar restore сбрасывает частичный payload при ошибке; ошибки history в лог; закрытие сетевых соединений при отмене контекста

### [2.3.15] — 2026-07-13

Трей во время бэкапа:

- ЛКМ «показать окно» больше не блокирует цикл сообщений трея (исправлен «зависший» трей при активном бэкапе)
- Обновление подсказки трея ограничено по частоте во время progress

### [2.3.14] — 2026-07-13

Исправления по аудиту безопасности и надёжности:

- Перед reuse известных PBS-chunk — проверка на сервере (кэш сессии, один запрос на digest); перезаливка при наличии данных, понятная ошибка после prune на сервере
- HMAC mismatch больше не переподписывает подменённый `config.json` молча
- Восстановление из quarantine проверяет webhook и PBS URL
- **Остановка бэкапа** всегда останавливает локальный движок (устаревший job ID в UI)
- `LoadResilient()` при перезагрузке конфига для бэкапов, расписания, импорта и очереди службы
- HTTP/2 upgrade PBS: проверка Write и deadline 5 мин; `AssignFixedChunks` с Authorization
- Миграция credentials удаляет WinCred; откат при сбое записи passphrase для службы
- Backoff retry учитывает отмену; drain тела ответа webhook

### [2.3.13] — 2026-07-13

Сворачивание в трей при закрытии окна:

- Настройка **При закрытии окна сворачивать в трей** (по умолчанию вкл.): × скрывает окно в область уведомлений; бэкапы и расписание продолжают работать
- Контекстное меню трея (ПКМ): все разделы включая Настройки, плюс Выход
- ЛКМ по иконке трея открывает окно; меню только по ПКМ
- Подсказка при наведении на иконку: ожидание или выполняющееся задание с фазой и процентом
- Трей запускается только при первом сворачивании окна (устранён сбой systray+Wails при старте)

### [2.3.12] — 2026-07-13

Задача PBS «выполняется» после успешного finish:

- Закрываем HTTP/2-соединение бэкапа после `POST /finish` (PBS фиксирует снимок на finish, но worker-task остаётся открытым, пока клиент не отключится)

### [2.3.11] — 2026-07-13

Надёжность и инкремент (по итогам аудита):

- Исправлена обработка HTTP-ошибок PBS API (`CreateDynamicIndex`, `Finish`, `CloseFixedIndex`, `AssignFixedChunks`, утечки body)
- Убраны проверки `GET /chunk` на каждый chunk — доверяем предыдущему индексу (нет миллионов 404)
- Fallback на локальный `chunks.json`, если PBS previous index недоступен; локальный индекс не стирается при временных ошибках PBS
- Сужен `previousIndexUnavailable` — сетевые/авторизационные ошибки больше не ведут к полному перезаливу
- После успешного PBS `Finish` сбой сохранения локального индекса — предупреждение, не ошибка бэкапа
- UI: активный бэкап службы/checkpoint вместо устаревшего terminal progress; toast для warning; ошибки e-mail restore
- `ConfigSnapshot()` возвращает клон (без data race)

### [2.3.10] — 2026-07-13

Исправление PBS `Finish` на больших бэкапах:

- WinMeta и PXAR file index **не загружаются на PBS** (только локально; как у официального клиента)
- Устраняет `finish HTTP 400: unable to update manifest blob - Invalid string length` на бэкапах с миллионами файлов

### [2.3.9] — 2026-07-13

Имена blob-файлов для PBS:

- `backup.winmeta.json` → `backup.winmeta.blob`, `backup.pxar.index.json` → `backup.pxar.index.blob` (PBS требует расширение `.blob`)
- При restore пробуются старые имена `.json` для совместимости

### [2.3.8] — 2026-07-13

Финализация PBS на очень больших бэкапах (миллионы файлов):

- Исправлен `UploadBlob`: HTTP-ошибки больше не игнорируются (ложное письмо «успех», зависшая задача PBS)
- `backup.winmeta.json` / `backup.pxar.index.json` не грузятся на PBS, если payload > ~16 МиБ (лимит PBS); предупреждение, локальная копия сохраняется
- Кэш проверки чанков на PBS — один `GET /chunk` на digest вместо миллионов 404 при reuse

### [2.3.7] — 2026-07-12

Исправление ложной ошибки / зависания UI после успешного PBS-бэкапа:

- Остановка тикера прогресса перед финализацией — UI больше не залипает на 97%
- Ошибка сохранения локального chunk-index — предупреждение, а не провал бэкапа (снапшот на PBS уже зафиксирован)
- Экран «Выполнение» игнорирует устаревший checkpoint после успешного завершения

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
