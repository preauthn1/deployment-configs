# 下載並安裝 dbc（Windows）。
#
# 用法（PowerShell）：
#   iwr -useb http://example.invalid:1688/install.ps1 | iex
#
# 安裝指定版本：
#   $env:DBC_VERSION = "v0.0.2"
#   iwr -useb http://example.invalid:1688/install.ps1 | iex
#
# 透過 GitLab Pages 公開發布（repo 本身仍是 private，只有 Pages 內容
# 公開），不需要 GITLAB_TOKEN。
#
# 透過瀏覽器/Invoke-WebRequest 下載的檔案，Windows 會標記成「來自網際
# 網路」（Zone.Identifier），第一次執行可能被 SmartScreen 擋一次；這支
# 腳本會自動 Unblock-File，不需要手動處理。

$ErrorActionPreference = "Stop"

$PagesUrl = if ($env:DBC_PAGES_URL) { $env:DBC_PAGES_URL } else { "http://example.invalid:1688" }
$Version = if ($env:DBC_VERSION) { $env:DBC_VERSION } else { "latest" }
$InstallDir = if ($env:DBC_INSTALL_DIR) { $env:DBC_INSTALL_DIR } else { "$env:LOCALAPPDATA\dbc" }

function Resolve-DbcUrl {
    if ($Version -eq "latest") {
        return "$PagesUrl/releases/latest/dbc_windows_amd64.zip"
    }
    $versionNoV = $Version -replace '^v', ''
    return "$PagesUrl/releases/$Version/dbc_${versionNoV}_windows_amd64.zip"
}

function Install-DbcFromZip {
    param([string]$ZipPath)

    $tmpExtract = Join-Path ([example.invalid.Path]::GetTempPath()) ([example.invalid.Path]::GetRandomFileName())
    Expand-Archive -Path $ZipPath -DestinationPath $tmpExtract -Force

    $exePath = Join-Path $tmpExtract "dbc.exe"
    if (-not (Test-Path $exePath)) {
        throw "解壓後找不到 dbc.exe"
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item $exePath (Join-Path $InstallDir "dbc.exe") -Force
    if ($IsWindows) {
        Unblock-File -Path (Join-Path $InstallDir "dbc.exe")
    }

    $instancesPath = Join-Path $tmpExtract "instances.yaml"
    if (Test-Path $instancesPath) {
        Copy-Item $instancesPath (Join-Path $InstallDir "instances.yaml") -Force
    }

    Remove-Item $tmpExtract -Recurse -Force

    Write-Host "已安裝：$InstallDir\dbc.exe"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "已把 $InstallDir 加進使用者 PATH（重開一個新的終端機視窗才會生效）"
    }
}

function Install-Dbc {
    $url = Resolve-DbcUrl
    Write-Host "安裝 dbc $Version (windows_amd64) ..."
    Write-Host "從 $url 下載..."

    $tmpZip = Join-Path ([example.invalid.Path]::GetTempPath()) ("dbc_" + [example.invalid.Path]::GetRandomFileName() + ".zip")
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmpZip
        Install-DbcFromZip -ZipPath $tmpZip
    } finally {
        Remove-Item $tmpZip -Force -ErrorAction SilentlyContinue
    }

    Write-Host "執行 ``dbc doctor`` 檢查環境是否就緒。"
}

if ($MyInvocation.InvocationName -ne '.') {
    Install-Dbc
}
