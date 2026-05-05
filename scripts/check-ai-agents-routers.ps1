<#
.SYNOPSIS
  Wrapper: runs scripts/check-ai-agents-routers.sh (same router-table check as CI).

.DESCRIPTION
  Uses Git Bash when available (same Bash as Git for Windows). Otherwise prints
  how to run the check on Linux/macOS/WSL or install Git for Windows.
#>

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$BashScript = Join-Path $ScriptDir 'check-ai-agents-routers.sh'

$gitBash = @(
  'C:\Program Files\Git\bin\bash.exe',
  'C:\Program Files (x86)\Git\bin\bash.exe'
) | Where-Object { Test-Path $_ } | Select-Object -First 1

if ($gitBash) {
  Set-Location $RepoRoot
  & $gitBash $BashScript @args
  exit $LASTEXITCODE
}

$bash = Get-Command bash -ErrorAction SilentlyContinue
if ($bash) {
  Set-Location $RepoRoot
  & bash $BashScript @args
  exit $LASTEXITCODE
}

Write-Host @'
check-ai-agents-routers: no bash found.

Install Git for Windows (includes Git Bash) or run on WSL/macOS/Linux.
You can also point directly to assets with AI_AGENTS_ROOT when needed:

  AI_AGENTS_ROOT=<toolkit-root>/.ai-agents bash scripts/check-ai-agents-routers.sh

  bash scripts/check-ai-agents-routers.sh

(from repository root)
'@

exit 1
