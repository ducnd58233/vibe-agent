#Requires -Version 5.1
<#
.SYNOPSIS
  Creates directory junctions so Claude Code, Cursor, and opencode see the canonical .ai-agents trees.

.DESCRIPTION
  Run from repository root (or any path): powershell -ExecutionPolicy Bypass -File scripts/link-ai-agents.ps1

  Creates:
    .claude/skills      --> .ai-agents/skills
    .claude/agents      --> .ai-agents/agents
    .claude/commands    --> .ai-agents/commands
    .cursor/skills      --> .ai-agents/skills
    .cursor/commands    --> .ai-agents/commands
    .opencode/agents    --> .ai-agents/agents
    .opencode/commands  --> .ai-agents/commands

  Existing junctions or directories at those paths are removed first.
#>
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

function Ensure-Junction {
    param(
        [Parameter(Mandatory = $true)][string] $LinkRelativePath,
        [Parameter(Mandatory = $true)][string] $TargetRelativePath
    )
    $targetFull = Join-Path $repoRoot $TargetRelativePath
    if (-not (Test-Path -LiteralPath $targetFull)) {
        throw "Missing target directory: $TargetRelativePath"
    }
    $linkFull = Join-Path $repoRoot $LinkRelativePath
    $parentDir = Split-Path -Parent $linkFull
    if (-not (Test-Path -LiteralPath $parentDir)) {
        New-Item -ItemType Directory -Path $parentDir | Out-Null
    }
    if (Test-Path -LiteralPath $linkFull) {
        Remove-Item -LiteralPath $linkFull -Force -Recurse
    }
    New-Item -ItemType Junction -Path $linkFull -Target $targetFull | Out-Null
}

Ensure-Junction '.claude\skills' '.ai-agents\skills'
Ensure-Junction '.claude\agents' '.ai-agents\agents'
Ensure-Junction '.claude\commands' '.ai-agents\commands'
Ensure-Junction '.cursor\skills' '.ai-agents\skills'
Ensure-Junction '.cursor\commands' '.ai-agents\commands'
Ensure-Junction '.opencode\agents' '.ai-agents\agents'
Ensure-Junction '.opencode\commands' '.ai-agents\commands'

Write-Host "Junctions created under .claude, .cursor, and .opencode pointing to .ai-agents."
