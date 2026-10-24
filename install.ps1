<#
.SYNOPSIS
    One-liner installer for yt-manage CLI on Windows.

.DESCRIPTION
    Downloads the latest yt-manage release from GitHub, verifies its
    SHA256 against the release checksums.txt, extracts yt-manage.exe
    to a local directory, and adds that directory to the user PATH.

.PARAMETER Version
    Install a specific version (e.g. v0.2.0). Default: latest.

.PARAMETER InstallDir
    Target directory. Default: $env:LOCALAPPDATA\yt-manage

.PARAMETER Arch
    Force architecture (amd64, arm64). Default: auto-detect.

.PARAMETER NoPath
    Skip adding to PATH.

.PARAMETER Uninstall
    Remove the installation directory and its PATH entry, then exit.

.EXAMPLE
    irm https://raw.githubusercontent.com/alimtvnetwork/yt-manage/main/install.ps1 | iex

.EXAMPLE
    & ./install.ps1 -Version v0.2.0

.NOTES
    Repository: https://github.com/alimtvnetwork/yt-manage
#>

param(
    [string]$Version = "",
    [string]$InstallDir = "",
    [string]$Arch = "",
    [switch]$NoPath,
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$Repo = "alimtvnetwork/yt-manage"

function Fail([string]$Message) {
    Write-Error "install.ps1: $Message"
    exit 1
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "yt-manage"
}

function Remove-FromUserPath([string]$Dir) {
    $current = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $current) { return }
    $parts = $current -split ";" | Where-Object { $_ -and $_.TrimEnd("\") -ne $Dir.TrimEnd("\") }
    [Environment]::SetEnvironmentVariable("Path", ($parts -join ";"), "User")
}

if ($Uninstall) {
    if (Test-Path $InstallDir) {
        Remove-Item -Recurse -Force $InstallDir
        Write-Host "Removed $InstallDir"
    }
    Remove-FromUserPath $InstallDir
    Write-Host "Uninstalled yt-manage."
    exit 0
}

# --- Detect architecture ------------------------------------------------
if (-not $Arch) {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { $Arch = "amd64" }
        "ARM64" { $Arch = "arm64" }
        default { Fail "unsupported architecture '$env:PROCESSOR_ARCHITECTURE'. Supported: amd64, arm64." }
    }
}
if ($Arch -notin @("amd64", "arm64")) {
    Fail "unsupported architecture '$Arch'. Supported: amd64, arm64."
}

# --- Resolve the release tag --------------------------------------------
if (-not $Version) {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "yt-manage-installer" }
        $Version = $release.tag_name
    } catch {
        Fail "could not query the latest release: $_"
    }
}
if (-not $Version) { Fail "could not determine the latest release tag." }

$Asset = "yt-manage_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"

# --- Download into a scratch directory -----------------------------------
$tmpDir = Join-Path $env:TEMP ("yt-manage-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
try {
    Write-Host "Downloading $Asset ($Version)..."
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile (Join-Path $tmpDir $Asset) -Headers @{ "User-Agent" = "yt-manage-installer" }
    Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile (Join-Path $tmpDir "checksums.txt") -Headers @{ "User-Agent" = "yt-manage-installer" }

    # --- Verify the checksum BEFORE installing anything -------------------
    $expected = $null
    foreach ($line in Get-Content (Join-Path $tmpDir "checksums.txt")) {
        $parts = $line -split "\s+"
        if ($parts.Count -ge 2 -and $parts[1].TrimStart("*") -eq $Asset) { $expected = $parts[0].ToLower() }
    }
    if (-not $expected) { Fail "checksums.txt has no entry for $Asset" }
    $actual = (Get-FileHash -Path (Join-Path $tmpDir $Asset) -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected) {
        Fail "checksum mismatch for $Asset (expected $expected, actual $actual)"
    }
    Write-Host "Checksum verified: $Asset"

    # --- Extract and install ----------------------------------------------
    $extractDir = Join-Path $tmpDir "extract"
    Expand-Archive -Path (Join-Path $tmpDir $Asset) -DestinationPath $extractDir -Force
    $exe = Join-Path $extractDir "yt-manage.exe"
    if (-not (Test-Path $exe)) { Fail "archive $Asset did not contain yt-manage.exe" }
    if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }
    Copy-Item $exe (Join-Path $InstallDir "yt-manage.exe") -Force
    Write-Host "Installed yt-manage $Version to $(Join-Path $InstallDir 'yt-manage.exe')"
} finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}

# --- PATH ------------------------------------------------------------------
if (-not $NoPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @()
    if ($userPath) { $entries = $userPath -split ";" }
    $already = $entries | Where-Object { $_ -and $_.TrimEnd("\") -eq $InstallDir.TrimEnd("\") }
    if (-not $already) {
        [Environment]::SetEnvironmentVariable("Path", (($entries + $InstallDir) -join ";"), "User")
        Write-Host "Added $InstallDir to your user PATH (new terminals pick it up)."
    }
    if (($env:Path -split ";") -notcontains $InstallDir) {
        $env:Path = "$env:Path;$InstallDir"
    }
} else {
    Write-Host "Note: $InstallDir was not added to PATH. Add it manually to run yt-manage from any terminal."
}
