#Requires -Version 5.1
<#
.SYNOPSIS
  Validates Codex-facing generated assets are in sync with .ai-agents.

.DESCRIPTION
  Checks:
  - .agents/skills and .agents/commands discovery paths exist.
  - every .ai-agents/commands/*.md command has a generated .codex/prompts/*.md prompt.
  - with -Global, every command also has a $CODEX_HOME/prompts/vibe-*.md prompt.
  - every .ai-agents/agents/*.md persona has a generated .codex/agents/*.toml.
  - generated TOML avoids stale relative links and common mojibake from non-UTF8 generation.
#>
param(
  [switch]$Global
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

$fail = 0
function Fail([string]$Message) {
  $script:fail = 1
  Write-Error $Message -ErrorAction Continue
}

function Get-CodexHome {
  if (-not [string]::IsNullOrWhiteSpace($env:CODEX_HOME)) {
    return $env:CODEX_HOME
  }
  $homeDir = if ($env:USERPROFILE) { $env:USERPROFILE } else { $HOME }
  if ([string]::IsNullOrWhiteSpace($homeDir)) {
    return ''
  }
  return (Join-Path $homeDir '.codex')
}

if (-not (Test-Path '.agents/skills' -PathType Container)) {
  Fail 'Missing .agents/skills. Run scripts/link-ai-agents.ps1 or scripts/link-ai-agents.sh.'
}
if (-not (Test-Path '.agents/commands' -PathType Container)) {
  Fail 'Missing .agents/commands. Run scripts/link-ai-agents.ps1 or scripts/link-ai-agents.sh.'
}
if (-not (Test-Path '.codex/prompts' -PathType Container)) {
  Fail 'Missing .codex/prompts. Run scripts/link-ai-agents.ps1 or scripts/link-ai-agents.sh.'
}

$exclude = @('README.md', 'ROUTER.md', 'TEMPLATE.md')
$sourceCommands = Get-ChildItem '.ai-agents/commands' -Filter '*.md' -File |
  Where-Object { $exclude -notcontains $_.Name } |
  ForEach-Object { $_.Name } |
  Sort-Object

$generatedPrompts = @()
if (Test-Path '.codex/prompts' -PathType Container) {
  $generatedPrompts = Get-ChildItem '.codex/prompts' -Filter '*.md' -File |
    ForEach-Object { $_.Name } |
    Sort-Object
}

$missingPrompts = Compare-Object $sourceCommands $generatedPrompts |
  Where-Object SideIndicator -eq '<=' |
  ForEach-Object InputObject
if ($missingPrompts) {
  Fail ("Missing generated Codex prompts: " + ($missingPrompts -join ', '))
}

$stalePrompts = Compare-Object $sourceCommands $generatedPrompts |
  Where-Object SideIndicator -eq '=>' |
  ForEach-Object InputObject
if ($stalePrompts) {
  Fail ("Stale generated Codex prompts: " + ($stalePrompts -join ', '))
}

if (Test-Path '.codex/prompts' -PathType Container) {
  foreach ($file in Get-ChildItem '.codex/prompts' -Filter '*.md' -File) {
    $source = Join-Path '.ai-agents/commands' $file.Name
    if ((Test-Path -LiteralPath $source) -and
        ((Get-FileHash $source -Algorithm MD5).Hash -ne (Get-FileHash $file.FullName -Algorithm MD5).Hash)) {
      Fail "$($file.Name) differs from .ai-agents/commands/$($file.Name)."
    }
  }
}

if ($Global) {
  $codexHome = Get-CodexHome
  if ([string]::IsNullOrWhiteSpace($codexHome)) {
    Fail 'Cannot resolve CODEX_HOME or a home directory for global Codex prompts.'
  } else {
    $globalPrompts = Join-Path $codexHome 'prompts'
    $promptPrefix = if ($env:LINK_CODEX_PROMPT_PREFIX) { $env:LINK_CODEX_PROMPT_PREFIX } else { 'vibe-' }
    if (-not (Test-Path -LiteralPath $globalPrompts -PathType Container)) {
      Fail "Missing global Codex prompts directory: $globalPrompts. Run scripts/link-ai-agents.ps1 or install-global.ps1."
    } else {
      foreach ($name in $sourceCommands) {
        $source = Join-Path '.ai-agents/commands' $name
        $generated = Join-Path $globalPrompts ($promptPrefix + $name)
        if (-not (Test-Path -LiteralPath $generated -PathType Leaf)) {
          Fail "Missing global Codex prompt: $generated"
          continue
        }
        if ((Get-FileHash $source -Algorithm MD5).Hash -ne (Get-FileHash $generated -Algorithm MD5).Hash) {
          Fail "$generated differs from .ai-agents/commands/$name."
        }
      }

      $manifest = Join-Path $codexHome '.vibe-agent-prompts.manifest'
      if (Test-Path -LiteralPath $manifest -PathType Leaf) {
        $expected = $sourceCommands | ForEach-Object { $promptPrefix + $_ }
        foreach ($entry in Get-Content -LiteralPath $manifest -ErrorAction SilentlyContinue) {
          if ([string]::IsNullOrWhiteSpace($entry)) {
            continue
          }
          $leaf = Split-Path -Leaf $entry
          if (($leaf.StartsWith($promptPrefix)) -and ($expected -notcontains $leaf)) {
            Fail "Stale global Codex prompt in manifest: $entry"
          }
        }
      }
    }
  }
}

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
