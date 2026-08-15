#Requires -Version 5.1
<#
.SYNOPSIS
  Validates Codex-facing generated assets are in sync with .ai-agents.

.DESCRIPTION
  Checks:
  - .agents/skills and .agents/commands discovery paths exist.
  - every .ai-agents/commands/*.md command has a generated Codex skill adapter.
  - with -Global, every command also has a $HOME/.agents/skills/vibe-*/SKILL.md adapter.
    Codex invokes those as $vibe-<name>; custom /prompts were removed from Codex CLI 0.117.0.
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

$exclude = @('README.md', 'ROUTER.md', 'TEMPLATE.md')
$sourceCommands = Get-ChildItem '.ai-agents/commands' -Filter '*.md' -File |
  Where-Object { $exclude -notcontains $_.Name } |
  ForEach-Object { $_.Name } |
  Sort-Object

foreach ($name in $sourceCommands) {
  $commandName = [System.IO.Path]::GetFileNameWithoutExtension($name)
  $skillPath = Join-Path '.agents/skills' (Join-Path $commandName 'SKILL.md')
  if (-not (Test-Path -LiteralPath $skillPath -PathType Leaf)) {
    Fail "Missing workspace Codex command skill: $skillPath"
    continue
  }
  $content = Get-Content -LiteralPath $skillPath -Raw -Encoding UTF8
  if ($content -notmatch "(?m)^name:\s*$([regex]::Escape($commandName))\s*$") {
    Fail "$skillPath has the wrong skill name."
  }
  if ($content -notmatch '<command_prompt>') {
    Fail "$skillPath is missing the command prompt body."
  }
}

if ($Global) {
  $codexHome = Get-CodexHome
  if ([string]::IsNullOrWhiteSpace($codexHome)) {
    Fail 'Cannot resolve a home directory for global Codex skills.'
  } else {
    $promptPrefix = if ($env:LINK_CODEX_PROMPT_PREFIX) { $env:LINK_CODEX_PROMPT_PREFIX } else { 'vibe-' }
    $homeDir = if ($env:USERPROFILE) { $env:USERPROFILE } else { $HOME }
    $globalSkillRoot = Join-Path $homeDir '.agents/skills'
    foreach ($name in $sourceCommands) {
      $commandName = [System.IO.Path]::GetFileNameWithoutExtension($name)
      $generated = Join-Path $globalSkillRoot (Join-Path ($promptPrefix + $commandName) 'SKILL.md')
      if (-not (Test-Path -LiteralPath $generated -PathType Leaf)) {
        Fail "Missing global Codex command skill: $generated"
        continue
      }
      $content = Get-Content -LiteralPath $generated -Raw -Encoding UTF8
      if ($content -notmatch "(?m)^name:\s*$([regex]::Escape($promptPrefix + $commandName))\s*$") {
        Fail "$generated has the wrong skill name."
      }
    }
    $manifest = Join-Path $codexHome '.vibe-agent-prompts.manifest'
    if (Test-Path -LiteralPath $manifest -PathType Leaf) {
      Fail "Stale Codex prompts manifest remains from removed custom prompts support: $manifest"
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
if ($Global) {
  $promptPrefix = if ($env:LINK_CODEX_PROMPT_PREFIX) { $env:LINK_CODEX_PROMPT_PREFIX } else { 'vibe-' }
  $codexCommandForm = '$' + $promptPrefix + '<name>'
  Write-Host "Codex command form: $codexCommandForm; custom /prompts and top-level /$promptPrefix<name> are not available in Codex CLI 0.147.0."
}
