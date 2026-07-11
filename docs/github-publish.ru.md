# Выложить ClickRAX на GitHub — пошагово

**Репозиторий:** https://github.com/clickrax/clickrax  
**Версия:** 2.3 · тег `v2.3`

---

## 1. Подготовка на компьютере (один раз)

### 1.1. Git

Установите [Git for Windows](https://git-scm.com/download/win), если ещё не стоит.

```powershell
git config --global user.name "John Watson"
git config --global user.email "ваш@email.com"
```

### 1.2. Очистка и сборка

В PowerShell из корня проекта:

```powershell
cd <путь-к-репозиторию-clickrax>
.\scripts\prepare-github.ps1
```

Скрипт удалит лишнее (`node_modules`, старые exe в корне, `build\bin`), соберёт GUI + CLI + установщик и прогонит `go test`.  
`frontend\dist\` после сборки есть локально, но в git не попадает (нужен для embed; на CI собирается заново).

> **В GitHub заливается только исходный код.** Папка `build\bin\` в репозиторий не попадает (см. `.gitignore`). Бинарники для пользователей создаёт GitHub Actions при теге `v2.3`.

---

## 2. Что НЕ должно попасть на GitHub

| Не коммитить | Почему |
|--------------|--------|
| `build\bin\` (*.exe, installer, zip) | Собирается в CI / локально |
| `frontend\node_modules\` | `npm ci` на CI |
| `frontend\dist\` | Собирается при `npm run build` |
| `config.json`, `*.hmac`, `secrets\` | Ваши данные и пароли |
| `.env`, `.cursor\`, `.vscode\` | Локальное окружение |
| `cli.exe`, `pbs-win-backup.exe` в корне | Старые локальные сборки |
| `*.zip` в корне | Архивы релиза |

Всё это уже перечислено в `.gitignore`. Если используете **git push**, лишнее не уйдёт. Если копируете папку вручную — не копируйте строки из таблицы.

---

## 3. Создать репозиторий на GitHub

1. Войти на https://github.com  
2. **New repository**  
3. Owner: `clickrax` (или ваш аккаунт)  
4. Name: `clickrax`  
5. **Public**  
6. **Не** ставить галочки «Add README / .gitignore / license» — они уже есть в проекте  
7. **Create repository**

---

## 4. Первый push (из папки проекта)

```powershell
cd <путь-к-репозиторию-clickrax>

# если remote ещё не настроен:
git remote add origin https://github.com/clickrax/clickrax.git

# добавить все файлы (лишнее отсечёт .gitignore)
git add -A

# проверить, что нет секретов и exe:
git status

git commit -m "Release 2.3: ClickRAX — GUI PBS backup for Windows"

git branch -M main
git push -u origin main
```

Если репозиторий уже существовал с README — один раз:

```powershell
git pull origin main --rebase
git push -u origin main
```

---

## 5. Настройки репозитория на GitHub

**Settings → General**

| Поле | Значение |
|------|----------|
| Description | см. `.github/DESCRIPTION.md` (блок English) |
| Website | `https://github.com/clickrax/clickrax` |
| Topics | `windows`, `backup`, `proxmox-backup-server`, `pbs`, `proxmox-backup-client`, `golang`, `wails`, `gui`, `vss`, `windows-service` |

**Settings → General → Features**

- Issues: включить  
- Discussions: по желанию  

**Settings → Security**

- Dependabot alerts: включить (есть `.github/dependabot.yml`)  
- Private vulnerability reporting: по желанию  

**About → Sponsor** — подтянется из `.github/FUNDING.yml` (Telegram).

---

## 6. Exe уже в репозитории (`release/v2.3/`)

1. Локально: `.\scripts\prepare-github.ps1` — соберёт exe и положит в `release/v2.3/`
2. `.\scripts\prepare-web-upload.ps1` — папка `clickrax-github-upload` **с exe внутри**
3. Залейте **всё** из `clickrax-github-upload` на GitHub через веб
4. Ссылки в README заработают сразу (ветка `main`)

Файлы в `release/v2.3/`:

| Файл |
|------|
| `clickrax.exe` |
| `clickrax-cli.exe` |
| `clickrax-amd64-installer.exe` |
| `clickrax-windows-amd64-portable.zip` |

Отдельный **GitHub Release** — по желанию, не обязателен для скачивания.

Подробно: [docs/release-v2.3.ru.md](release-v2.3.ru.md).

---

## 7. Проверка после публикации

- [ ] README открывается, ссылки на Releases работают  
- [ ] В репозитории **нет** `node_modules`, `build/bin`, `config.json`  
- [ ] **Release v2.3** создан, 4 файла прикреплены — **ссылки на exe в README работают**
- [ ] Проверены URL из [docs/release-v2.3.ru.md](docs/release-v2.3.ru.md) (не 404)
- [ ] Установщик и portable exe запускаются на чистой Windows  
- [ ] В программе: автор John Watson, Telegram @Johnwatson7777  

---

## 8. Обновления в будущем

1. Правки в коде  
2. Поднять версию в `internal/version/version.go` и `wails.json` → `productVersion`  
3. Запись в `CHANGELOG.md`  
4. `.\scripts\prepare-github.ps1`  
5. `git commit` → `git push`  
6. `git tag vX.Y` → `git push origin vX.Y`  
7. Обновить ссылки на версию в README при смене major/minor  

---

## English summary

1. Run `.\scripts\prepare-github.ps1`  
2. `git add -A && git commit && git push` to `github.com/clickrax/clickrax`  
3. Configure repo description/topics (`.github/DESCRIPTION.md`)  
4. `git tag v2.3 && git push origin v2.3` for automated release binaries  
5. Never commit `build/bin`, `node_modules`, `config.json`, or secrets  
