# Build DelayDeck OBS plugin (out-of-tree).
#
# Prerequisites:
#   - OBS Studio built from source (build_x64 preset on Windows)
#   - CMake 3.16+
#
# Usage:
#   .\scripts\dev\build-obs-plugin.ps1 -ObsStudioDir "C:\dev\obs-studio"
#
# Optional install into an existing OBS prefix:
#   .\scripts\dev\build-obs-plugin.ps1 -ObsStudioDir "C:\dev\obs-studio" `
#     -InstallPrefix "C:\Program Files\obs-studio"

param(
    [string]$BuildDir = "",
    [string]$InstallPrefix = "",
    [string]$ObsStudioDir = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$pluginDir = Join-Path $repoRoot "apps\obs-plugin"

if (-not $BuildDir) {
    $BuildDir = Join-Path $pluginDir "build"
}

function Get-ObsPrefixPath {
    param([string]$Root)

    $obsBuildDir = Join-Path $Root "build_x64"
    if (-not (Test-Path $obsBuildDir)) {
        throw "OBS build directory not found: $obsBuildDir"
    }

    $prefixes = @(
        (Join-Path $obsBuildDir "libobs"),
        (Join-Path $obsBuildDir "frontend\api"),
        (Join-Path $obsBuildDir "deps\w32-pthreads")
    )

    $depsRoot = Join-Path $Root ".deps"
    if (Test-Path $depsRoot) {
        foreach ($depDir in Get-ChildItem $depsRoot -Directory) {
            $prefixes += $depDir.FullName
        }
    }

    return ($prefixes -join ';')
}

function Get-CMakeGenerator {
    $generators = cmake -G 2>&1 | Out-String
    if ($generators -match "Visual Studio 18 2026") {
        return "Visual Studio 18 2026"
    }
    if ($generators -match "Visual Studio 17 2022") {
        return "Visual Studio 17 2022"
    }
    return $null
}

$cmakeModulePath = $null

if ($ObsStudioDir) {
    $ObsStudioDir = (Resolve-Path $ObsStudioDir).Path
    $env:CMAKE_PREFIX_PATH = Get-ObsPrefixPath -Root $ObsStudioDir
    $cmakeModulePath = Join-Path $ObsStudioDir "cmake\finders"
} elseif (-not $env:CMAKE_PREFIX_PATH) {
    Write-Error "Pass -ObsStudioDir or set CMAKE_PREFIX_PATH before running this script."
}

$generator = Get-CMakeGenerator
if (-not $generator) {
    Write-Error "No supported Visual Studio generator found. Install Visual Studio Build Tools with C++ desktop development."
}

function Convert-ToCMakePath {
    param([string]$Path)
    return ($Path -replace '\\', '/')
}

New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null

$prefixPath = Convert-ToCMakePath $env:CMAKE_PREFIX_PATH

$cmakeArgs = @(
    "-S", $pluginDir,
    "-B", $BuildDir,
    "-G", $generator,
    "-A", "x64",
    "-DCMAKE_PREFIX_PATH=$prefixPath"
)

if ($cmakeModulePath -and (Test-Path $cmakeModulePath)) {
    $cmakeArgs += "-DCMAKE_MODULE_PATH=$(Convert-ToCMakePath $cmakeModulePath)"
}

if ($InstallPrefix) {
    $cmakeArgs += "-DCMAKE_INSTALL_PREFIX=$InstallPrefix"
}

cmake @cmakeArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

cmake --build $BuildDir --config RelWithDebInfo
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($InstallPrefix) {
    cmake --install $BuildDir --config RelWithDebInfo
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "Built: $(Join-Path $BuildDir 'RelWithDebInfo\delaydeck.dll')"
