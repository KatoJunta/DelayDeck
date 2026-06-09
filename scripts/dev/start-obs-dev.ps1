# Launch OBS with DelayDeck managed Relay.
#
# Usage:
#   .\scripts\dev\start-obs-dev.ps1
#
# Normal use: configure destination once in the DelayDeck dock ("Configure Destination").
# Environment variables below are optional overrides for development.
#
# Optional external relay (unmanaged):
#   .\scripts\dev\start-obs-dev.ps1 -ManagedRelay:$false -SessionToken <token>

param(
    [bool]$ManagedRelay = $true,
    [string]$SessionToken = "",
    [string]$RelayUrl = "http://127.0.0.1:9400",
    [string]$RelayBin = "",
    [string]$ObsExe = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$relayDir = Join-Path $repoRoot "apps\relay-engine"

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

if ($ManagedRelay) {
    if (-not $RelayBin) {
        $RelayBin = Join-Path $relayDir "delaydeck-relay.exe"
    }

    Write-Host "Building delaydeck-relay: $RelayBin"
    & (Join-Path $repoRoot "scripts\dev\sync-repo.ps1")
    Push-Location $relayDir
    try {
        go build -o $RelayBin ./cmd/delaydeck-relay
    }
    finally {
        Pop-Location
    }

    $env:DELAYDECK_MANAGED_RELAY = "1"
    $env:DELAYDECK_RELAY_BIN = (Resolve-Path $RelayBin).Path
    $env:DELAYDECK_RELAY_URL = $RelayUrl
    Remove-Item Env:DELAYDECK_SESSION_TOKEN -ErrorAction SilentlyContinue
    Remove-Item Env:DELAYDECK_RELAY_MODE -ErrorAction SilentlyContinue

    Write-Host "DELAYDECK_MANAGED_RELAY = 1"
    Write-Host "DELAYDECK_RELAY_BIN     = $($env:DELAYDECK_RELAY_BIN)"
    Write-Host "DELAYDECK_RELAY_URL     = $RelayUrl"
    Write-Host "DELAYDECK_INGEST_LISTEN = 127.0.0.1:9401"
    $env:DELAYDECK_INGEST_LISTEN = "127.0.0.1:9401"

    if ($env:DELAYDECK_OUTPUT_URL -and $env:DELAYDECK_OUTPUT_STREAM_KEY) {
        Write-Host "DELAYDECK_OUTPUT_URL        = $($env:DELAYDECK_OUTPUT_URL)"
        Write-Host "DELAYDECK_OUTPUT_STREAM_KEY = (set)"
    } else {
        Write-Host "Relay destination: use DelayDeck dock setup (or set DELAYDECK_OUTPUT_URL / DELAYDECK_OUTPUT_STREAM_KEY)"
    }

    if ($env:DELAYDECK_FIXED_DELAY_SECONDS) {
        Write-Host "DELAYDECK_FIXED_DELAY_SECONDS = $($env:DELAYDECK_FIXED_DELAY_SECONDS)"
    }
} else {
    if (-not $SessionToken) {
        Write-Error "Unmanaged relay requires -SessionToken"
    }

    $env:DELAYDECK_MANAGED_RELAY = "0"
    $env:DELAYDECK_SESSION_TOKEN = $SessionToken
    $env:DELAYDECK_RELAY_URL = $RelayUrl
    Remove-Item Env:DELAYDECK_RELAY_BIN -ErrorAction SilentlyContinue
    Remove-Item Env:DELAYDECK_RELAY_MODE -ErrorAction SilentlyContinue

    Write-Host "DELAYDECK_MANAGED_RELAY = 0"
    Write-Host "DELAYDECK_SESSION_TOKEN = $SessionToken"
    Write-Host "DELAYDECK_RELAY_URL     = $RelayUrl"
    Write-Host "DELAYDECK_INGEST_LISTEN = 127.0.0.1:9401"
    $env:DELAYDECK_INGEST_LISTEN = "127.0.0.1:9401"
}

$obsDir = Split-Path -Parent $ObsExe

Write-Host "Starting OBS: $ObsExe"
Write-Host "Working directory: $obsDir"

# OBS resolves data/locale paths relative to the process working directory.
Start-Process -FilePath $ObsExe -WorkingDirectory $obsDir
