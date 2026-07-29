<#
.SYNOPSIS
  Downloads the vibe-agent runtime binary for this machine.

.DESCRIPTION
  The runtime is optional. Every asset under .ai-agents/ works without it; the
  binary adds enforced workflow transitions, persisted run state, and memory.

  You do not need Go, a C compiler, or SQLite to run it.

.PARAMETER Version
  Release to install, for example v0.1.0. Defaults to the latest release.

.PARAMETER InstallDir
  Where to place the binary. Defaults to %LOCALAPPDATA%\Programs\vibe-agent.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1
#>

[CmdletBinding()]
param(
  [string]$Version = 'latest',
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\vibe-agent'),
  [string]$Repo = 'ducnd58233/vibe-agent'
)

$ErrorActionPreference = 'Stop'
$binary = 'vibe-agent.exe'

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE. Build from source: cd runtime; make install" }
}

if ($Version -eq 'latest') {
  Write-Host 'resolving the latest release'
  $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $release.tag_name -replace '^runtime/', ''
  if (-not $Version) { throw 'Could not resolve the latest release; pass -Version explicitly.' }
}

$asset = "vibe-agent_${Version}_windows_${arch}.exe"
$base = "https://github.com/$Repo/releases/download/runtime/$Version"
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $temp -Force | Out-Null

try {
  Write-Host "downloading $asset"
  $downloaded = Join-Path $temp $asset
  Invoke-WebRequest -Uri "$base/$asset" -OutFile $downloaded -UseBasicParsing

  # A binary that runs with the session's own privileges is worth verifying.
  try {
    $sums = Join-Path $temp 'SHA256SUMS'
    Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile $sums -UseBasicParsing
    $expected = (Select-String -Path $sums -Pattern ([regex]::Escape($asset)) |
      Select-Object -First 1).Line -replace '\s.*$', ''
    $actual = (Get-FileHash -Path $downloaded -Algorithm SHA256).Hash.ToLower()
    if ($expected -and $expected.ToLower() -ne $actual) {
      throw 'Checksum mismatch; do not run this file.'
    }
    Write-Host 'checksum verified'
  } catch [System.Net.WebException] {
    Write-Warning "No SHA256SUMS published for $Version, skipping verification."
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $target = Join-Path $InstallDir $binary
  Move-Item -Path $downloaded -Destination $target -Force
  Write-Host "installed $target"

  # Hooks invoke the binary by name, so PATH matters.
  $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
  if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable('PATH', "$userPath;$InstallDir", 'User')
    Write-Host "added $InstallDir to your user PATH; open a new terminal to pick it up"
  }
} finally {
  Remove-Item -Path $temp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host 'check the install with:'
Write-Host '  vibe-agent version'
Write-Host '  vibe-agent doctor'
