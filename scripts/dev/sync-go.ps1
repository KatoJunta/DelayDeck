# Sync apps/relay-engine/go.mod from the repo root go.work toolchain line.
#
# Usage:
#   .\scripts\dev\sync-go.ps1

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$goWorkFile = Join-Path $repoRoot "go.work"
$goModFile = Join-Path $repoRoot "apps\relay-engine\go.mod"

if (-not (Test-Path $goWorkFile)) {
    Write-Error "go.work not found: $goWorkFile"
}

$goWorkLine = Get-Content $goWorkFile | Where-Object { $_ -match '^\s*go\s+' } | Select-Object -First 1
if (-not $goWorkLine) {
    Write-Error "go.work does not contain a 'go <version>' line"
}

if ($goWorkLine -notmatch '^\s*go\s+(\d+\.\d+(?:\.\d+)?)\s*$') {
    Write-Error "go.work toolchain line is invalid: $goWorkLine"
}

$goVersion = $Matches[1]
$goModLines = @(Get-Content $goModFile)
$updated = $false
for ($i = 0; $i -lt $goModLines.Count; $i++) {
    if ($goModLines[$i] -match '^\s*go\s+\d+\.\d+(?:\.\d+)?\s*$') {
        $goModLines[$i] = "go $goVersion"
        $updated = $true
        break
    }
}

if (-not $updated) {
    Write-Error "go.mod does not contain a 'go <version>' line to update"
}

$utf8NoBom = New-Object System.Text.UTF8Encoding $false
[System.IO.File]::WriteAllText($goModFile, ($goModLines -join "`n") + "`n", $utf8NoBom)
Write-Host "Synced Go toolchain: $goVersion -> $goModFile"
