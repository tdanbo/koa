#!/usr/bin/env pwsh
#
# koa installer.
#
#   irm https://raw.githubusercontent.com/tdanbo/koa/main/install.ps1 | iex
#
# Downloads the latest koa release from GitHub and installs the binary. koa
# dogfoods its own naming convention, so the asset it looks for is exactly the
# one maintainers must publish to be koa-installable:
#
#   koa-{version}-amd64-windows.zip
#
# Environment:
#   KOA_REPO         owner/name to install from       (default tdanbo/koa)
#   KOA_VERSION      release tag to install           (default: latest)
#   KOA_INSTALL_DIR  where to place the binary        (default: %LOCALAPPDATA%\Programs\koa)
#   GITHUB_TOKEN     token for private repos / higher rate limits
#   KOA_API          GitHub API root                 (default api.github.com)
#
# `irm | iex` can't take script parameters, so configuration is
# environment-only — the same shape as install.sh — rather than command-line
# flags.

$ErrorActionPreference = "Stop"

function Die($Message) {
    Write-Host "koa: $Message" -ForegroundColor Red
    exit 1
}

# --- preflight ---------------------------------------------------------------

if ($env:OS -ne "Windows_NT") {
    Die "this script installs the Windows build; use install.sh on Linux"
}

$Arch = $env:PROCESSOR_ARCHITEW6432
if (-not $Arch) { $Arch = $env:PROCESSOR_ARCHITECTURE }
if ($Arch -notin @("AMD64", "x86_64")) {
    Die "unsupported architecture: $Arch. koa ships amd64 binaries only."
}

$Repo = if ($env:KOA_REPO) { $env:KOA_REPO } else { "tdanbo/koa" }
$Version = $env:KOA_VERSION
$InstallDir = $env:KOA_INSTALL_DIR
if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) { Die "%LOCALAPPDATA% is not set; pass KOA_INSTALL_DIR explicitly" }
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\koa"
}
$ApiRoot = if ($env:KOA_API) { $env:KOA_API } else { "https://api.github.com" }
$Api = "$ApiRoot/repos/$Repo"

$Headers = @{
    "Accept"                 = "application/vnd.github+json"
    "User-Agent"             = "koa-install"
    "X-GitHub-Api-Version"   = "2022-11-28"
}
if ($env:GITHUB_TOKEN) {
    $Headers["Authorization"] = "Bearer $($env:GITHUB_TOKEN)"
}

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) "koa-install-$([System.Guid]::NewGuid())"

try {
    # --- resolve the release ---------------------------------------------------

    $ReleaseUrl = if ($Version) { "$Api/releases/tags/$Version" } else { "$Api/releases/latest" }

    Write-Host "Fetching release metadata from $Repo…"
    try {
        $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers -UseBasicParsing
    } catch {
        Die "could not read $ReleaseUrl`: $($_.Exception.Message)"
    }

    $Tag = $Release.tag_name
    if (-not $Tag) { Die "no release found for $Repo" }

    # koa's own naming convention. The version segment is whatever the tag
    # holds, with a leading `v` stripped, which is how koa publishes its assets.
    $Stripped = $Tag -replace '^v', ''
    $AssetName = "koa-$Stripped-amd64-windows.zip"

    $Asset = $Release.assets | Where-Object { $_.name -eq $AssetName } | Select-Object -First 1
    if (-not $Asset) { Die "release $Tag publishes no asset named $AssetName" }

    # --- download and unpack -----------------------------------------------

    New-Item -ItemType Directory -Path $Tmp -Force | Out-Null
    $ZipPath = Join-Path $Tmp $AssetName

    Write-Host "Downloading $AssetName ($Tag)…"
    # The asset's API url (not browser_download_url) is what makes an install
    # from a private repo work with a token, same as install.sh.
    $DownloadHeaders = $Headers.Clone()
    $DownloadHeaders["Accept"] = "application/octet-stream"
    Invoke-WebRequest -Uri $Asset.url -Headers $DownloadHeaders -OutFile $ZipPath -UseBasicParsing

    $ExtractDir = Join-Path $Tmp "extracted"
    Expand-Archive -Path $ZipPath -DestinationPath $ExtractDir

    $Binary = Get-ChildItem -Path $ExtractDir -Filter "koa.exe" -Recurse | Select-Object -First 1
    if (-not $Binary) { Die "no koa.exe inside $AssetName" }

    # --- install -------------------------------------------------------------

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $Binary.FullName -Destination (Join-Path $InstallDir "koa.exe") -Force

    Write-Host ""
    Write-Host "koa $Tag installed to $InstallDir\koa.exe"

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathEntries = @()
    if ($UserPath) { $PathEntries = $UserPath -split ";" }
    if ($PathEntries -contains $InstallDir) {
        Write-Host "Run it with: koa"
    } else {
        Write-Host ""
        Write-Host "$InstallDir is not on your PATH. Add it with:"
        Write-Host ""
        Write-Host "    [Environment]::SetEnvironmentVariable('Path', `"`$([Environment]::GetEnvironmentVariable('Path','User'));$InstallDir`", 'User')"
        Write-Host ""
        Write-Host "then open a new terminal."
    }
} finally {
    Remove-Item -Path $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
