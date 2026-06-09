# Sync generated metadata from repo root source files.
#
# - VERSION -> apps/relay-engine/internal/version/version.go
# - go.work -> apps/relay-engine/go.mod
#
# Usage:
#   .\scripts\dev\sync-repo.ps1

$ErrorActionPreference = "Stop"

$scriptDir = $PSScriptRoot
& (Join-Path $scriptDir "sync-version.ps1")
& (Join-Path $scriptDir "sync-go.ps1")
