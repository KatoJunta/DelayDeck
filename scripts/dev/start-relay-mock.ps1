# Start DelayDeck Relay in mock mode (external / unmanaged relay testing).
#
# Usage:
#   .\scripts\dev\start-relay-mock.ps1
#   .\scripts\dev\start-obs-dev.ps1 -ManagedRelay:$false -SessionToken dev-phase3
#
# For normal development, OBS auto-starts Relay (Phase 4):
#   .\scripts\dev\start-obs-dev.ps1

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
