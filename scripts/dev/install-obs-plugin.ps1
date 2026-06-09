# Copy delaydeck.dll and locale into an OBS install prefix.
#
# Usage (run PowerShell as Administrator for Program Files):
#   .\scripts\dev\install-obs-plugin.ps1
#
# Custom OBS prefix:
#   .\scripts\dev\install-obs-plugin.ps1 -ObsPrefix "I:\dev\obs-studio\build_x64\rundir\RelWithDebInfo"

param(
    [string]$ObsPrefix = "C:\Program Files\obs-studio"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$dllSrc = Join-Path $repoRoot "apps\obs-plugin\build\RelWithDebInfo\delaydeck.dll"
$relayDir = Join-Path $repoRoot "apps\relay-engine"
$relaySrc = Join-Path $relayDir "delaydeck-relay.exe"
$localeSrc = Join-Path $repoRoot "apps\obs-plugin\data\locale\en-US.ini"

if (-not (Test-Path $dllSrc)) {
    Write-Error "Plugin DLL not found. Build first: .\scripts\dev\build-obs-plugin.ps1 -ObsStudioDir <path>"
}

if (-not (Test-Path $relaySrc)) {
    Write-Host "Building delaydeck-relay: $relaySrc"
    & (Join-Path $repoRoot "scripts\dev\sync-version.ps1")
    Push-Location $relayDir
    try {
        go build -o $relaySrc ./cmd/delaydeck-relay
        if ($LASTEXITCODE -ne 0) {
            Write-Error "go build failed (exit $LASTEXITCODE)"
        }
    }
    finally {
        Pop-Location
    }
}

if (-not (Test-Path $localeSrc)) {
    Write-Error "Locale file not found: $localeSrc"
}

$pluginDest = Join-Path $ObsPrefix "obs-plugins\64bit"
$localeDest = Join-Path $ObsPrefix "data\obs-plugins\delaydeck\locale"

New-Item -ItemType Directory -Force -Path $pluginDest, $localeDest | Out-Null

Copy-Item $dllSrc (Join-Path $pluginDest "delaydeck.dll") -Force
Copy-Item $relaySrc (Join-Path $pluginDest "delaydeck-relay.exe") -Force
Copy-Item (Join-Path $repoRoot "apps\obs-plugin\data\locale\ja-JP.ini") (Join-Path $localeDest "ja-JP.ini") -Force
Copy-Item $localeSrc (Join-Path $localeDest "en-US.ini") -Force

Write-Host "Installed DLL:    $(Join-Path $pluginDest 'delaydeck.dll')"
Write-Host "Installed relay:  $(Join-Path $pluginDest 'delaydeck-relay.exe')"
Write-Host "Installed locale: $(Join-Path $localeDest 'ja-JP.ini')"
Write-Host "Installed locale: $(Join-Path $localeDest 'en-US.ini')"
Write-Host "Restart OBS after installing."
