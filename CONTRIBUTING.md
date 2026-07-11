# Contributing / Участие в разработке

**Languages:** [English](#english) · [Русский](#русский)  
**Repository:** https://github.com/clickrax/clickrax

---

<a id="english"></a>

## English

Thank you for contributing to ClickRAX!

### Platform

ClickRAX is **Windows-only**. Development and CI must run on **Windows 10/11 or Windows Server 2016+**.

Non-Windows build tags compile stubs only; they are not supported platforms.

### Prerequisites

- Go 1.26+
- Node.js LTS
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`)
- Python 3 (icon: `scripts/generate-icon.py`)
- WebView2 Runtime (GUI)
- NSIS (optional installer: `winget install NSIS.NSIS`)

### Build

```powershell
git clone https://github.com/clickrax/clickrax.git
cd clickrax
.\build.ps1
```

Installer: `.\build.ps1 -Installer`  
Custom update repo: `.\build.ps1 -GitHubRepo "clickrax/clickrax"`

### Tests

```powershell
go test ./...
go vet ./...
```

### Pull requests

1. Fork and create a feature branch.
2. Keep changes focused; match existing style.
3. Add tests for behavior changes.
4. Ensure `go test ./...` and `go vet ./...` pass.
5. Do not commit secrets or real PBS URLs/tokens.
6. Update [CHANGELOG.md](CHANGELOG.md) for user-visible changes.

### Module name

Go module: `pbs-win-backup` (legacy). Product name: **ClickRAX**.

### Developer utilities

See [cmd/README.md](cmd/README.md) — local troubleshooting tools, not shipped in releases.

### License

ClickRAX is copyrighted by John Watson. Contributions may be incorporated only with the author's approval. See [LICENSE](LICENSE). Third-party code in `third_party/` remains under its own licenses.

---

<a id="русский"></a>

## Русский

Спасибо за участие в разработке ClickRAX!

### Платформа

ClickRAX работает **только на Windows**. Разработка и CI — на **Windows 10/11 или Windows Server 2016+**.

Сборка на Linux/macOS компилирует заглушки; это не поддерживаемые платформы.

### Требования

- Go 1.26+
- Node.js LTS
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`)
- Python 3 (иконка: `scripts/generate-icon.py`)
- WebView2 Runtime (GUI)
- NSIS (опционально: `winget install NSIS.NSIS`)

### Сборка

```powershell
git clone https://github.com/clickrax/clickrax.git
cd clickrax
.\build.ps1
```

Установщик: `.\build.ps1 -Installer`  
Проверка обновлений: `.\build.ps1 -GitHubRepo "clickrax/clickrax"`

### Тесты

```powershell
go test ./...
go vet ./...
```

### Pull requests

1. Fork и feature branch.
2. Минимальный scope изменений; следуйте стилю проекта.
3. Тесты для нового поведения.
4. `go test ./...` и `go vet ./...` должны проходить.
5. Не коммитьте секреты и реальные URL PBS.
6. Обновляйте [CHANGELOG.md](CHANGELOG.md) для пользовательских изменений.

### Имя модуля

Go module: `pbs-win-backup` (legacy). Продукт: **ClickRAX**.

### Утилиты разработчика

См. [cmd/README.md](cmd/README.md) — локальные утилиты, не входят в релиз.

### Лицензия

ClickRAX — авторское право John Watson. Вклад может быть принят только с согласия автора. См. [LICENSE](LICENSE). Код в `third_party/` остаётся под своими лицензиями.
