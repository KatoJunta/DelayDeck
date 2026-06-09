# Launch OBS with DelayDeck managed Relay (Phase 4+).
#
# Usage:
#   .\scripts\dev\start-obs-dev.ps1
#
# Normal use: configure destination once in the DelayDeck dock ("Configure Destination").
# Environment variables below are optional overrides for development.
#
# Do not run start-relay-mock.ps1 at the same time — both bind 127.0.0.1:9400
# and the Dock may appear to reconnect to the wrong relay after a kill test.
#
# Optional external relay (Phase 3 style):
#   .\scripts\dev\start-relay-mock.ps1        # terminal 1
#   .\scripts\dev\start-obs-dev.ps1 -ManagedRelay:$false -SessionToken dev-phase3

param(
    [bool]$ManagedRelay = $true,
    [string]$SessionToken = "dev-phase3",
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

    Write-Host "DELAYDECK_MANAGED_RELAY = 1"
    Write-Host "DELAYDECK_RELAY_BIN     = $($env:DELAYDECK_RELAY_BIN)"
    Write-Host "DELAYDECK_RELAY_URL     = $RelayUrl"
    Write-Host "DELAYDECK_INGEST_LISTEN = 127.0.0.1:9401"
    $env:DELAYDECK_INGEST_LISTEN = "127.0.0.1:9401"

    if ($env:DELAYDECK_RELAY_MODE -eq "forwarding") {
        if (-not $env:DELAYDECK_OUTPUT_URL -or -not $env:DELAYDECK_OUTPUT_STREAM_KEY) {
            Write-Warning "DELAYDECK_RELAY_MODE=forwarding requires DELAYDECK_OUTPUT_URL and DELAYDECK_OUTPUT_STREAM_KEY"
        } else {
            Write-Host "DELAYDECK_RELAY_MODE        = forwarding"
            Write-Host "DELAYDECK_OUTPUT_URL        = $($env:DELAYDECK_OUTPUT_URL)"
            Write-Host "DELAYDECK_OUTPUT_STREAM_KEY = (set)"
            if ($env:DELAYDECK_FIXED_DELAY_SECONDS) {
                Write-Host "DELAYDECK_FIXED_DELAY_SECONDS = $($env:DELAYDECK_FIXED_DELAY_SECONDS)"
            }
        }
    } else {
        $env:DELAYDECK_RELAY_MODE = "mock"
        Write-Host "DELAYDECK_RELAY_MODE        = mock"
    }
} else {
    $env:DELAYDECK_MANAGED_RELAY = "0"
    $env:DELAYDECK_SESSION_TOKEN = $SessionToken
    $env:DELAYDECK_RELAY_URL = $RelayUrl
    Remove-Item Env:DELAYDECK_RELAY_BIN -ErrorAction SilentlyContinue

    Write-Host "DELAYDECK_MANAGED_RELAY = 0"
    Write-Host "DELAYDECK_SESSION_TOKEN = $SessionToken"
    Write-Host "DELAYDECK_RELAY_URL     = $RelayUrl"
    Write-Host "DELAYDECK_INGEST_LISTEN = 127.0.0.1:9401"
    $env:DELAYDECK_INGEST_LISTEN = "127.0.0.1:9401"

    if ($env:DELAYDECK_RELAY_MODE -eq "forwarding") {
        Write-Host "DELAYDECK_RELAY_MODE        = forwarding"
    } else {
        $env:DELAYDECK_RELAY_MODE = "mock"
        Write-Host "DELAYDECK_RELAY_MODE        = mock"
    }
}

$obsDir = Split-Path -Parent $ObsExe

Write-Host "Starting OBS: $ObsExe"
Write-Host "Working directory: $obsDir"

# OBS resolves data/locale paths relative to the process working directory.
Start-Process -FilePath $ObsExe -WorkingDirectory $obsDir
