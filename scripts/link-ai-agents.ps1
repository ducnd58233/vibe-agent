#Requires -Version 5.1
<#
.SYNOPSIS
  Creates directory junctions so Claude Code, Cursor, and opencode see the canonical .ai-agents trees.

.DESCRIPTION
  Default mode (no parameters): workspace = parent of scripts/, assets = <that>/.ai-agents (this toolkit repo).

  Consumer mode: set -WorkspaceRoot to the consumer repository root and -AssetsRoot to the toolkit .ai-agents
  folder (for example .../consumer/.vibe-agent/.ai-agents). Junction targets use absolute paths.

  Creates under WorkspaceRoot:
    .claude/skills, .claude/agents, .claude/commands
    .cursor/skills, .cursor/commands
    .opencode/agents, .opencode/commands

  Examples:
    powershell -File scripts/link-ai-agents.ps1

    powershell -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')
#>
[CmdletBinding()]
param(
    [Parameter(HelpMessage = "Root where .claude, .cursor, .opencode are created. Default: directory containing scripts/.")]
    [string] $WorkspaceRoot = "",

    [Parameter(HelpMessage = "Folder containing skills, agents, commands (typically .../.ai-agents). Default: <toolkit>/.ai-agents.")]
    [string] $AssetsRoot = ""
)

$ErrorActionPreference = "Stop"

function Resolve-ExistingDirectory {
    param([Parameter(Mandatory = $true)][string] $PathInput)
    $full = if ([System.IO.Path]::IsPathRooted($PathInput)) {
        $PathInput
    }
    else {
        Join-Path -Path (Get-Location).Path -ChildPath $PathInput
    }
    if (-not (Test-Path -LiteralPath $full -PathType Container)) {
        throw "Missing or not a directory: $full"
    }
    return (Get-Item -LiteralPath $full).FullName
}

$toolkitHome = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($WorkspaceRoot)) {
    $WorkspaceRoot = $toolkitHome
}
if ([string]::IsNullOrWhiteSpace($AssetsRoot)) {
    $AssetsRoot = Join-Path $toolkitHome ".ai-agents"
}

$workspaceFull = Resolve-ExistingDirectory -PathInput $WorkspaceRoot
$assetsFull = Resolve-ExistingDirectory -PathInput $AssetsRoot

foreach ($name in @("skills", "agents", "commands")) {
    $p = Join-Path $assetsFull $name
    if (-not (Test-Path -LiteralPath $p -PathType Container)) {
        throw "Assets root must contain '$name' directory: $p"
    }
}

function Ensure-Junction {
    param(
        [Parameter(Mandatory = $true)][string] $LinkFullPath,
        [Parameter(Mandatory = $true)][string] $TargetFullPath
    )
    if (-not (Test-Path -LiteralPath $TargetFullPath -PathType Container)) {
        throw "Missing target directory: $TargetFullPath"
    }
    $parentDir = Split-Path -Parent $LinkFullPath
    if (-not (Test-Path -LiteralPath $parentDir)) {
        New-Item -ItemType Directory -Path $parentDir | Out-Null
    }
    if (Test-Path -LiteralPath $LinkFullPath) {
        $attrs = [System.IO.File]::GetAttributes($LinkFullPath)
        if (($attrs -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            # Junction/symlink: never use -Recurse here (it would delete the link target tree).
            cmd.exe /c "rmdir `"$LinkFullPath`""
        }
        else {
            Remove-Item -LiteralPath $LinkFullPath -Force -Recurse
        }
    }
    New-Item -ItemType Junction -Path $LinkFullPath -Target $TargetFullPath | Out-Null
}

Ensure-Junction (Join-Path $workspaceFull ".claude\skills") (Join-Path $assetsFull "skills")
Ensure-Junction (Join-Path $workspaceFull ".claude\agents") (Join-Path $assetsFull "agents")
Ensure-Junction (Join-Path $workspaceFull ".claude\commands") (Join-Path $assetsFull "commands")
Ensure-Junction (Join-Path $workspaceFull ".cursor\skills") (Join-Path $assetsFull "skills")
Ensure-Junction (Join-Path $workspaceFull ".cursor\commands") (Join-Path $assetsFull "commands")
Ensure-Junction (Join-Path $workspaceFull ".opencode\agents") (Join-Path $assetsFull "agents")
Ensure-Junction (Join-Path $workspaceFull ".opencode\commands") (Join-Path $assetsFull "commands")

Write-Host "Junctions created under $workspaceFull (.claude, .cursor, .opencode) -> $assetsFull"
