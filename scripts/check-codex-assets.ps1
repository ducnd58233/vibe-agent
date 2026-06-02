#Requires -Version 5.1
<#
.SYNOPSIS
  Validates Codex-facing generated assets are in sync with .ai-agents.

.DESCRIPTION
  Checks:
  - .agents/skills and .agents/commands discovery paths exist.
  - every .ai-agents/agents/*.md persona has a generated .codex/agents/*.toml.
  - generated TOML avoids stale relative links and common mojibake from non-UTF8 generation.
#>

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

$fail = 0
function Fail([string]$Message) {
  $script:fail = 1
  Write-Error $Message -ErrorAction Continue
}

if (-not (Test-Path '.agents/skills' -PathType Container)) {
  Fail 'Missing .agents/skills. Run scripts/link-ai-agents.ps1 or scripts/link-ai-agents.sh.'
}
if (-not (Test-Path '.agents/commands' -PathType Container)) {
  Fail 'Missing .agents/commands. Run scripts/link-ai-agents.ps1 or scripts/link-ai-agents.sh.'
}

$exclude = @('README.md', 'ROUTER.md', 'TEMPLATE.md')
$sourceAgents = Get-ChildItem '.ai-agents/agents' -Filter '*.md' -File |
  Where-Object { $exclude -notcontains $_.Name } |
  ForEach-Object { $_.BaseName } |
  Sort-Object

$generatedAgents = @()
if (Test-Path '.codex/agents' -PathType Container) {
  $generatedAgents = Get-ChildItem '.codex/agents' -Filter '*.toml' -File |
    ForEach-Object { $_.BaseName } |
    Sort-Object
}

$missing = Compare-Object $sourceAgents $generatedAgents |
  Where-Object SideIndicator -eq '<=' |
  ForEach-Object InputObject
if ($missing) {
  Fail ("Missing generated Codex agents: " + ($missing -join ', '))
}

$stale = Compare-Object $sourceAgents $generatedAgents |
  Where-Object SideIndicator -eq '=>' |
  ForEach-Object InputObject
if ($stale) {
  Fail ("Stale generated Codex agents: " + ($stale -join ', '))
}

if (Test-Path '.codex/agents' -PathType Container) {
  foreach ($file in Get-ChildItem '.codex/agents' -Filter '*.toml' -File) {
    $content = Get-Content -LiteralPath $file.FullName -Raw -Encoding UTF8
    if ($content -match '\.\./(skills|references|commands|agents|stack-profiles)/') {
      Fail "$($file.Name) contains stale relative .ai-agents links."
    }

    # Detect common UTF-8-as-ANSI mojibake markers without embedding mojibake literals
    # in this script, because those literals can break Windows PowerShell parsing.
    $hasMojibake = $content.Contains([char]0x00C3) -or
      $content.Contains([char]0x00C2) -or
      $content.Contains([char]0x00E2)
    if ($hasMojibake) {
      Fail "$($file.Name) appears to contain mojibake; regenerate with UTF-8 link script."
    }
  }
}

if ($fail -ne 0) {
  Write-Host 'check-codex-assets: FAILED'
  exit 1
}

Write-Host 'check-codex-assets: OK'
