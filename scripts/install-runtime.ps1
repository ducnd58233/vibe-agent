<#
.SYNOPSIS
  Downloads the vibe-agent runtime binary for this machine.

.DESCRIPTION
  The runtime is optional. Every asset under .ai-agents/ works without it; the
  binary adds enforced workflow transitions, persisted run state, and memory.

  Prefers a published release, which needs no Go. When no release is reachable
  it falls back to building from runtime/, which does. That fallback is what
  makes the toolkit usable before the first release exists.

.PARAMETER FromSource
  Skip the release lookup and build from runtime/ directly.

.PARAMETER SkipPathUpdate
  Leave the user PATH alone. Also honoured as $env:VIBE_SKIP_PATH_UPDATE. Use it
  when installing to a temporary directory, which would otherwise be added to
  PATH permanently.

.PARAMETER Version
  Release to install, for example v0.1.0. Defaults to the latest release.

.PARAMETER InstallDir
  Where to place the binary. Defaults to $env:VIBE_INSTALL_DIR, then
  %USERPROFILE%\.local\bin.

  One location, shared with install-runtime.sh and with `make install` in
  runtime/. They used to write to three different directories, so a machine could
  end up with several vibe-agent binaries and PATH order decided which one the
  hooks called. A `make install` was then silently ineffective, because a copy
  installed elsewhere still won.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1
#>

[CmdletBinding()]
param(
  [string]$Version = 'latest',
  [string]$InstallDir = $(if ($env:VIBE_INSTALL_DIR) { $env:VIBE_INSTALL_DIR } else { Join-Path $env:USERPROFILE '.local\bin' }),
  [string]$Repo = 'ducnd58233/vibe-agent',
  [switch]$FromSource,
  [switch]$SkipPathUpdate
)

$ErrorActionPreference = 'Stop'
$binary = 'vibe-agent.exe'
$runtimeDir = Join-Path $PSScriptRoot '../runtime'

function Show-Result {
    param([string]$Target)
    $installed = ''
    try { $installed = (& $Target version 2>$null | Select-Object -First 1).Trim() } catch { }
    if ($installed) {
        Write-Host "installed $Target ($installed)"
    } else {
        Write-Host "installed $Target"
    }
    # What matters is which copy PATH finds, not which one was just written. A
    # shadowed install is invisible, and its symptom is a hook behaving like a
    # version you thought you replaced. Reported here, in install-runtime.sh, and
    # by `make install`, because all three write the binary and any of them can be
    # the one being shadowed.
    $resolved = Get-Command vibe-agent -ErrorAction SilentlyContinue
    if ($resolved -and $installed) {
        $winner = ''
        try { $winner = (& $resolved.Source version 2>$null | Select-Object -First 1).Trim() } catch { }
        if ($winner -and $winner -ne $installed) {
            Write-Host ''
            Write-Warning "PATH resolves vibe-agent to a different build:"
            Write-Warning "  $($resolved.Source) ($winner)"
            Write-Warning "so this install does not change what the hooks run."
            Write-Warning "Remove that copy, or put $InstallDir earlier on PATH."
        }
    }

    Write-Host ''
    Write-Host 'check the install with:'
    Write-Host '  vibe-agent version'
    Write-Host '  vibe-agent doctor'
}

# Replaces the binary in place, tolerating the case where it is running.
#
# This script now fetches on every link run rather than skipping when a binary
# exists, so overwriting a live one is the normal case, not the rare one. Windows
# refuses to delete or overwrite a loaded image but allows renaming it, which is
# what makes an in-place replacement possible at all: move the old one aside,
# put the new one in place, and clear the leftover when the process lets go.
function Install-Binary {
    param([string]$Downloaded, [string]$Target)

    if (-not (Test-Path -LiteralPath $Target)) {
        Move-Item -LiteralPath $Downloaded -Destination $Target -Force
        return
    }

    $aside = "$Target.old"
    Remove-Item -LiteralPath $aside -Force -ErrorAction SilentlyContinue
    try {
        Move-Item -LiteralPath $Target -Destination $aside -Force
    } catch {
        throw @"
Could not replace ${Target}: $($_.Exception.Message)

Something is holding the file open. Close any terminal or editor running
vibe-agent, then rerun this script.
"@
    }

    try {
        Move-Item -LiteralPath $Downloaded -Destination $Target -Force
    } catch {
        # Put the working binary back rather than leaving nothing installed.
        Move-Item -LiteralPath $aside -Destination $Target -Force
        throw
    }
    Remove-Item -LiteralPath $aside -Force -ErrorAction SilentlyContinue
}

function Add-ToUserPath {
    param([string]$Directory)

    # Hooks invoke the binary by name, so PATH matters. This is the only part of
    # the script that changes state outside the install directory, which is why it
    # can be turned off: a scripted or throwaway install to a temporary directory
    # would otherwise leave that directory on the user's PATH permanently.
    if ($SkipPathUpdate -or $env:VIBE_SKIP_PATH_UPDATE) {
        Write-Host "skipping the PATH update; add $Directory yourself if hooks need it"
        return
    }

    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($userPath -notlike "*$Directory*") {
        [Environment]::SetEnvironmentVariable('PATH', "$userPath;$Directory", 'User')
        Write-Host "added $Directory to your user PATH; open a new terminal to pick it up"
    }
}

# Compiling is the fallback, not the plan. A release needs no Go; this path
# exists so the toolkit still works before one is published.
function Install-FromSource {
    param([string]$Reason)

    if (-not (Test-Path -LiteralPath (Join-Path $runtimeDir 'go.mod'))) {
        throw "$Reason, and there is no runtime directory to build from."
    }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw @"
$Reason

Two ways forward:
  1. Install Go, then rerun this script. It will build the binary for you.
  2. Wait for a published release: https://github.com/$Repo/releases

Neither is required to use the toolkit. Without the binary every hook is a
quiet no-op and the markdown assets work exactly as they do today.
"@
    }

    Write-Host $Reason
    Write-Host "building from source with $(go version)"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir $binary

    Push-Location $runtimeDir
    try {
        $env:CGO_ENABLED = '0'
        & go build -ldflags='-s -w -X main.version=source' -o $target ./cmd
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed. Run 'cd runtime; make check' to see why."
        }
    } finally {
        Pop-Location
    }

    Add-ToUserPath -Directory $InstallDir
    Show-Result -Target $target
    exit 0
}

if ($FromSource) { Install-FromSource -Reason 'Building from source (-FromSource).' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE. Build from source: cd runtime; make install" }
}

# Asset names embed the version, and the rolling build's version carries a commit
# sha nobody can predict: vibe-agent_0.0.0-main.<sha>_windows_amd64.exe. So the
# release JSON is the source of truth for what to download, rather than a URL
# assembled from guesses. Assembling it is what this script used to do, and every
# lookup 404ed into a source build that needs Go, which a release exists to avoid.
function Get-Release {
  param([string]$Endpoint)
  try {
    return Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/$Endpoint"
  } catch {
    return $null
  }
}

# Resolution order, matching install-runtime.sh:
#   1. an explicit version, when one was passed
#   2. the newest stable release
#   3. the rolling build from main, which is a prerelease and so is invisible to
#      the /releases/latest endpoint. Skipping it left Windows with no release at
#      all until the first stable tag existed.
function Resolve-Release {
  if ($Version -ne 'latest') {
    return Get-Release "tags/runtime/$Version"
  }

  Write-Host 'looking for a published release'
  $release = Get-Release 'latest'
  if ($release) { return $release }

  Write-Host 'no stable release yet, trying the rolling build from main'
  return Get-Release 'tags/runtime/latest'
}

$release = Resolve-Release
if (-not $release) {
  if ($Version -ne 'latest') {
    Install-FromSource -Reason "No release tagged runtime/$Version."
  }
  Install-FromSource -Reason "No published release found for $Repo."
}

# Match on the platform suffix rather than reconstructing the name, so this keeps
# working whatever the version string turned out to be.
$assetInfo = $release.assets | Where-Object { $_.name -like "*_windows_${arch}.exe" } | Select-Object -First 1
if (-not $assetInfo) {
  Install-FromSource -Reason "Release $($release.tag_name) has no asset for windows/$arch."
}
$sumsInfo = $release.assets | Where-Object { $_.name -eq 'SHA256SUMS' } | Select-Object -First 1

$asset = $assetInfo.name
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $temp -Force | Out-Null

try {
  Write-Host "downloading $asset"
  $downloaded = Join-Path $temp $asset
  try {
    Invoke-WebRequest -Uri $assetInfo.browser_download_url -OutFile $downloaded -UseBasicParsing
  } catch {
    Install-FromSource -Reason "Download of $asset failed."
  }

  # A binary that runs with the session's own privileges is worth verifying.
  if ($sumsInfo) {
    $sums = Join-Path $temp 'SHA256SUMS'
    try {
      Invoke-WebRequest -Uri $sumsInfo.browser_download_url -OutFile $sums -UseBasicParsing
      $line = (Select-String -Path $sums -Pattern ([regex]::Escape($asset)) | Select-Object -First 1).Line
      $expected = ($line -replace '\s.*$', '').ToLower()
      $actual = (Get-FileHash -Path $downloaded -Algorithm SHA256).Hash.ToLower()
      if ($expected -and $expected -ne $actual) {
        throw "Checksum mismatch for ${asset}; do not run this file."
      }
      if (-not $expected) {
        Write-Warning "SHA256SUMS has no entry for $asset, skipping verification."
      } else {
        Write-Host 'checksum verified'
      }
    } catch [System.Net.WebException] {
      Write-Warning "Could not fetch SHA256SUMS, skipping verification."
    }
  } else {
    Write-Warning "No SHA256SUMS published alongside this asset, skipping verification."
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $target = Join-Path $InstallDir $binary
  Install-Binary -Downloaded $downloaded -Target $target

  Add-ToUserPath -Directory $InstallDir
  Show-Result -Target $target
} finally {
  Remove-Item -Path $temp -Recurse -Force -ErrorAction SilentlyContinue
}
