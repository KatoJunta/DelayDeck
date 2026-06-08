# Start DelayDeck Relay in mock mode for OBS plugin development.
#
# Usage:
#   .\scripts\dev\start-relay-mock.ps1
#
# Before launching OBS, set the same token in the environment:
#   $env:DELAYDECK_SESSION_TOKEN = "dev-phase3"
#   $env:DELAYDECK_RELAY_URL = "http://127.0.0.1:9400"

param(
    [string]$ListenAddress = "127.0.0.1:9400",
    [string]$Token = "dev-phase3"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$relayDir = Join-Path $repoRoot "apps\relay-engine"

Push-Location $relayDir
try {
    go run ./cmd/delaydeck-relay `
        --listen $ListenAddress `
        --token $Token `
        --mock-auto-connect
}
finally {
    Pop-Location
}
