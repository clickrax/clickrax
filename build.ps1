# ClickRAX build script
#   .\build.ps1                              - GUI + CLI in build\bin\
#   .\build.ps1 -Installer                   - NSIS installer in build\bin\
#   .\build.ps1 -GitHubRepo "owner/clickrax" - enable in-app update check

param(
    [switch]$Installer,
    [switch]$InstallNSIS,
    [string]$GitHubRepo = "clickrax/clickrax"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$env:PATH = "$env:PATH;$env:USERPROFILE\go\bin"
$outDir = Join-Path $PSScriptRoot "build\bin"

function Invoke-Step {
    param([string]$Name, [scriptblock]$Action)
    Write-Host "=== $Name ===" -ForegroundColor Cyan
    $prevEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $Action
        if ($LASTEXITCODE -ne 0) {
            throw "Step failed ($Name): exit code $LASTEXITCODE"
        }
    } finally {
        $ErrorActionPreference = $prevEAP
    }
}

function Find-MakeNSIS {
    $cmd = Get-Command makensis -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($p in @(
        "${env:ProgramFiles(x86)}\NSIS\makensis.exe",
        "$env:ProgramFiles\NSIS\makensis.exe"
    )) {
        if (Test-Path $p) { return $p }
    }
    return $null
}

function Ensure-NSIS {
    $makensis = Find-MakeNSIS
    if ($makensis) {
        $nsisDir = Split-Path $makensis -Parent
        if ($env:PATH -notlike "*$nsisDir*") {
            $env:PATH = "$nsisDir;$env:PATH"
        }
        return
    }
    if ($InstallNSIS) {
        winget install --id NSIS.NSIS -e --accept-package-agreements --accept-source-agreements
        $makensis = Find-MakeNSIS
        if ($makensis) {
            $env:PATH = "$(Split-Path $makensis -Parent);$env:PATH"
            return
        }
    }
    Write-Host "ERROR: makensis not found. Run: winget install NSIS.NSIS" -ForegroundColor Red
    exit 1
}

function Show-BuiltFiles {
    Get-ChildItem $outDir -Filter "*.exe" -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "OK: $($_.FullName) ($([math]::Round($_.Length/1MB,2)) MB, $($_.LastWriteTime))" -ForegroundColor Green
    }
}

$appVersion = (Select-String -Path (Join-Path $PSScriptRoot "internal\version\version.go") -Pattern 'const Version = "([^"]+)"').Matches.Groups[1].Value
$buildTime = Get-Date -Format "yyyy-MM-ddTHH:mm:ssK"
Write-Host "Build ClickRAX $appVersion at $buildTime" -ForegroundColor Green
Write-Host "Output: $outDir" -ForegroundColor DarkGray
if ($GitHubRepo) {
    Write-Host "GitHubRepo: $GitHubRepo" -ForegroundColor DarkGray
}

$ldflags = "-s -w"
if ($GitHubRepo) {
    $ldflags += " -X pbs-win-backup/internal/updates.GitHubRepo=$GitHubRepo"
}

New-Item -ItemType Directory -Force -Path $outDir | Out-Null

Write-Host "=== Frontend build ===" -ForegroundColor Cyan
$npm = Get-Command npm -ErrorAction SilentlyContinue
if ($npm) {
    Push-Location frontend
    try {
        Invoke-Step "npm ci" { npm ci }
        Invoke-Step "npm run build" { npm run build }
    } finally {
        Pop-Location
    }
} else {
    throw "npm not found in PATH"
}

Invoke-Step "Wails bindings" { wails generate module }

$iconScript = Join-Path $PSScriptRoot "scripts\generate-icon.py"
$logoSrc = Join-Path $PSScriptRoot "logoclickrax.png"
if (-not (Test-Path $logoSrc)) {
    throw "Logo not found: $logoSrc"
}
Invoke-Step "Generate app icon" { python $iconScript }
Get-ChildItem $PSScriptRoot -Filter "rsrc_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem (Join-Path $PSScriptRoot "build\windows") -Filter "rsrc_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force

if ($Installer) {
    Ensure-NSIS
    Invoke-Step "Wails build + NSIS installer" { wails build -clean -nsis -ldflags $ldflags }
    $installers = Get-ChildItem "$outDir\*installer*.exe" -ErrorAction SilentlyContinue
    if (-not $installers) { throw "Installer not created in $outDir" }
} else {
    Invoke-Step "Wails build (GUI)" { wails build -clean -ldflags $ldflags }
}

$gui = Join-Path $outDir "clickrax.exe"
if (-not (Test-Path $gui)) {
    throw "GUI binary not found: $gui"
}

Invoke-Step "Build CLI" { go build -ldflags $ldflags -o "$outDir\clickrax-cli.exe" .\cmd\cli }

Show-BuiltFiles
