# Release v2.3 — exe в репозитории

**Repository:** https://github.com/clickrax/clickrax

---

## Как скачивают exe

Файлы лежат **в самом репозитории**, папка **`release/v2.3/`**.  
Ссылки в README ведут сюда:

`https://github.com/clickrax/clickrax/raw/main/release/v2.3/ИМЯ_ФАЙЛА`

После загрузки репозитория на GitHub ссылки работают **сразу** — отдельный Release создавать не обязательно.

---

## Перед выкладкой (локально)

```powershell
cd <путь-к-репозиторию>
.\scripts\prepare-github.ps1
```

Скрипт соберёт exe и скопирует в `release/v2.3/`:

| Файл |
|------|
| `clickrax.exe` |
| `clickrax-cli.exe` |
| `clickrax-amd64-installer.exe` |
| `clickrax-windows-amd64-portable.zip` |

Для веб-загрузки:

```powershell
.\scripts\prepare-web-upload.ps1
```

Папка `clickrax-github-upload` будет **с исходниками и release/v2.3/** — залейте всё на GitHub.

---

## Проверка ссылок

| Файл | URL |
|------|-----|
| GUI | https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax.exe |
| CLI | https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-cli.exe |
| ZIP | https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-windows-amd64-portable.zip |
| Installer | https://github.com/clickrax/clickrax/raw/main/release/v2.3/clickrax-amd64-installer.exe |
| Папка | https://github.com/clickrax/clickrax/tree/main/release/v2.3 |

---

## GitHub Releases (опционально)

Можно дополнительно создать Release **v2.3** для красивой страницы загрузки — но для ссылок в README это **не обязательно**, если `release/v2.3/` уже в репозитории.

---

## English

Binaries are in **`release/v2.3/`** inside the repo. Run `prepare-github.ps1`, then upload (or use `prepare-web-upload.ps1`). README raw links work immediately after publish. GitHub Release is optional.
