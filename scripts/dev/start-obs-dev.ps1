# Launch OBS with DelayDeck Relay dev environment variables.
#
# Usage:
#   .\scripts\dev\start-relay-mock.ps1        # terminal 1
#   .\scripts\dev\start-obs-dev.ps1           # terminal 2
#
# The token must match start-relay-mock.ps1 (default: dev-phase3).

param(
    [string]$SessionToken = "dev-phase3",
    [string]$RelayUrl = "http://127.0.0.1:9400",
    [string]$ObsExe = ""
)

$ErrorActionPreference = "Stop"

if (-not $ObsExe) {
    $candidates = @(
        "C:\Program Files\obs-studio\bin\64bit\obs64.exe",
        "C:\Program Files (x86)\obs-studio\bin\64bit\obs64.exe"
    )
    foreach ($path in $candidates) {
        if (Test-Path $path) {
            $ObsExe = $path
            break
        }
    }
}

if (-not $ObsExe -or -not (Test-Path $ObsExe)) {
    Write-Error @"
obs64.exe not found. Pass -ObsExe explicitly, for example:
  .\scripts\dev\start-obs-dev.ps1 -ObsExe 'I:\dev\obs-studio\build_x64\rundir\RelWithDebInfo\bin\64bit\obs64.exe'
"@
}

$env:DELAYDECK_SESSION_TOKEN = $SessionToken
$env:DELAYDECK_RELAY_URL = $RelayUrl

$obsDir = Split-Path -Parent $ObsExe

Write-Host "DELAYDECK_SESSION_TOKEN = $SessionToken"
Write-Host "DELAYDECK_RELAY_URL     = $RelayUrl"
Write-Host "Starting OBS: $ObsExe"
Write-Host "Working directory: $obsDir"

# OBS resolves data/locale paths relative to the process working directory.
Start-Process -FilePath $ObsExe -WorkingDirectory $obsDir
