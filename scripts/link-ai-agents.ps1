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
    .agents/skills, .agents/commands (Codex discovery when supported; mirrors .ai-agents)
    .codex/agents/*.toml (Codex custom subagents, generated from .ai-agents/agents/*.md)

  Examples:
    powershell -File scripts/link-ai-agents.ps1

    powershell -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')

  Optional environment variables (when the matching parameter is omitted):
    LINK_WORKSPACE, LINK_ASSETS — same paths as -WorkspaceRoot / -AssetsRoot (useful from CI or wrappers).
  The script also adds generated discovery paths to <WorkspaceRoot>/.git/info/exclude when WorkspaceRoot is a
  Git repository. This keeps local links and generated Codex agent files out of Git without requiring root
  .gitignore rules in consumer repositories.
#>
[CmdletBinding()]
param(
    [Parameter(HelpMessage = "Root where .claude, .cursor, .opencode are created. Default: directory containing scripts/.")]
    [string] $WorkspaceRoot = "",

    [Parameter(HelpMessage = "Folder containing skills, agents, commands (typically .../.ai-agents). Default: <toolkit>/.ai-agents.")]
    [string] $AssetsRoot = ""
)

$ErrorActionPreference = "Stop"

# Coerce PathInfo / other objects from callers (e.g. -WorkspaceRoot $PWD) to plain strings.
$WorkspaceRoot = [string]$WorkspaceRoot
$AssetsRoot = [string]$AssetsRoot

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
    if (-not [string]::IsNullOrWhiteSpace($env:LINK_WORKSPACE)) {
        $WorkspaceRoot = $env:LINK_WORKSPACE
    }
    else {
        $WorkspaceRoot = $toolkitHome
    }
}
if ([string]::IsNullOrWhiteSpace($AssetsRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($env:LINK_ASSETS)) {
        $AssetsRoot = $env:LINK_ASSETS
    }
    else {
        $AssetsRoot = Join-Path $toolkitHome ".ai-agents"
    }
}

$workspaceFull = Resolve-ExistingDirectory -PathInput $WorkspaceRoot
$assetsFull = Resolve-ExistingDirectory -PathInput $AssetsRoot

foreach ($name in @("skills", "agents", "commands")) {
    $p = Join-Path $assetsFull $name
    if (-not (Test-Path -LiteralPath $p -PathType Container)) {
        throw "Assets root must contain '$name' directory: $p"
    }
}

function Get-FrontmatterField {
    param(
        [Parameter(Mandatory = $true)][string] $YamlBlock,
        [Parameter(Mandatory = $true)][string] $FieldName
    )
    if ($YamlBlock -match "(?ms)^${FieldName}:\s*>\-?\s*\r?\n(.*?)(?=^\S|\z)") {
        $folded = ($matches[1] -split "\r?\n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' }) -join ' '
        return $folded.Trim()
    }
    if ($YamlBlock -match "(?m)^${FieldName}:\s*(.+)$") {
        $value = $matches[1].Trim()
        if ($value -match '^["''](.+)["'']$') {
            return $matches[1]
        }
        return $value
    }
    return $null
}

function Test-ReadOnlyAgentTools {
    param([Parameter(Mandatory = $true)][string] $YamlBlock)
    if ($YamlBlock -notmatch '(?m)^tools:\s*$') {
        return $false
    }
    $mutating = @('Bash', 'Edit', 'Write', 'NotebookEdit', 'Task')
    foreach ($tool in $mutating) {
        if ($YamlBlock -match "(?m)^\s+${tool}:\s*true\s*$") {
            return $false
        }
    }
    return $true
}

function Escape-TomlString {
    param([Parameter(Mandatory = $true)][string] $Value)
    return $Value.Replace('\', '\\').Replace('"', '\"')
}

function Convert-AgentBodyForCodex {
    param([Parameter(Mandatory = $true)][string] $Body)
    $converted = $Body
    $converted = $converted.Replace('../skills/', '.ai-agents/skills/')
    $converted = $converted.Replace('../references/', '.ai-agents/references/')
    $converted = $converted.Replace('../stack-profiles/', '.ai-agents/stack-profiles/')
    $converted = $converted.Replace('../commands/', '.ai-agents/commands/')
    $converted = $converted.Replace('../agents/', '.ai-agents/agents/')
    return @"
Codex note: this file is generated from `.ai-agents/agents`. Resolve shared asset links from the workspace root (for example `.ai-agents/skills/...`), not from `.codex/agents`.

$converted
"@
}

function Sync-CodexAgents {
    param(
        [Parameter(Mandatory = $true)][string] $AssetsFull,
        [Parameter(Mandatory = $true)][string] $WorkspaceFull
    )
    $agentsSrc = Join-Path $AssetsFull 'agents'
    $agentsDest = Join-Path $WorkspaceFull '.codex\agents'
    $excludeNames = @('TEMPLATE.md', 'README.md', 'ROUTER.md')
    if (-not (Test-Path -LiteralPath $agentsDest)) {
        New-Item -ItemType Directory -Path $agentsDest -Force | Out-Null
    }
    $generatedNames = New-Object 'System.Collections.Generic.List[string]'
    Get-ChildItem -LiteralPath $agentsSrc -Filter '*.md' -File | ForEach-Object {
        if ($excludeNames -contains $_.Name) {
            return
        }
        $content = Get-Content -LiteralPath $_.FullName -Raw -Encoding UTF8
        if ($content -notmatch '(?s)\A---\r?\n(.*?)\r?\n---\r?\n(.*)\z') {
            Write-Warning "Skipping $($_.Name): missing YAML frontmatter."
            return
        }
        $yamlBlock = $matches[1]
        $body = Convert-AgentBodyForCodex -Body $matches[2].Trim()
        $name = Get-FrontmatterField -YamlBlock $yamlBlock -FieldName 'name'
        if ([string]::IsNullOrWhiteSpace($name)) {
            Write-Warning "Skipping $($_.Name): missing name in frontmatter."
            return
        }
        $description = Get-FrontmatterField -YamlBlock $yamlBlock -FieldName 'description'
        if ([string]::IsNullOrWhiteSpace($description)) {
            $description = $name
        }
        $sandboxLine = ''
        if (Test-ReadOnlyAgentTools -YamlBlock $yamlBlock) {
            $sandboxLine = "sandbox_mode = `"read-only`"`r`n"
        }
        $tomlPath = Join-Path $agentsDest ($name + '.toml')
        $header = @"
# Generated by scripts/link-ai-agents.ps1 from .ai-agents/agents/$($_.Name) - do not edit; re-run link script.

name = "$(Escape-TomlString $name)"
description = "$(Escape-TomlString $description)"
$($sandboxLine)developer_instructions = """

"@
        $tomlText = $header + $body + "`"`"`"`n"
        [System.IO.File]::WriteAllText($tomlPath, $tomlText, (New-Object System.Text.UTF8Encoding $false))
        [void]$generatedNames.Add($name)
    }
    Get-ChildItem -LiteralPath $agentsDest -Filter '*.toml' -File | ForEach-Object {
        $agentName = [System.IO.Path]::GetFileNameWithoutExtension($_.Name)
        if ($generatedNames -notcontains $agentName) {
            Remove-Item -LiteralPath $_.FullName -Force
        }
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
Ensure-Junction (Join-Path $workspaceFull ".agents\skills") (Join-Path $assetsFull "skills")
Ensure-Junction (Join-Path $workspaceFull ".agents\commands") (Join-Path $assetsFull "commands")
Sync-CodexAgents -AssetsFull $assetsFull -WorkspaceFull $workspaceFull

function Install-LocalGitExclude {
    param([Parameter(Mandatory = $true)][string] $WorkspaceFull)
    $gitDir = Join-Path $WorkspaceFull ".git"
    if (-not (Test-Path -LiteralPath $gitDir -PathType Container)) {
        Write-Warning "No .git directory at $WorkspaceFull; skipped local git exclude rules."
        return
    }
    $infoDir = Join-Path $gitDir "info"
    if (-not (Test-Path -LiteralPath $infoDir)) {
        New-Item -ItemType Directory -Path $infoDir | Out-Null
    }
    $excludePath = Join-Path $infoDir "exclude"
    if (-not (Test-Path -LiteralPath $excludePath)) {
        New-Item -ItemType File -Path $excludePath | Out-Null
    }
    $rules = @(
        "/.claude/skills/",
        "/.claude/agents/",
        "/.claude/commands/",
        "/.cursor/skills/",
        "/.cursor/commands/",
        "/.opencode/agents/",
        "/.opencode/commands/",
        "/.agents/skills/",
        "/.agents/commands/",
        "/.codex/agents/"
    )
    $existing = Get-Content -LiteralPath $excludePath -ErrorAction SilentlyContinue
    $toAdd = $rules | Where-Object { $existing -notcontains $_ }
    if ($toAdd.Count -eq 0) {
        Write-Host "Local git exclude rules already present at $excludePath"
        return
    }
    Add-Content -LiteralPath $excludePath -Value ""
    Add-Content -LiteralPath $excludePath -Value "# Generated vibe-agent discovery paths"
    Add-Content -LiteralPath $excludePath -Value $toAdd
    Write-Host "Installed local git exclude rules at $excludePath"
}

function Install-CommitAttributionHook {
    param(
        [Parameter(Mandatory = $true)][string] $AssetsFull,
        [Parameter(Mandatory = $true)][string] $WorkspaceFull
    )
    $gitDir = Join-Path $WorkspaceFull ".git"
    if (-not (Test-Path -LiteralPath $gitDir -PathType Container)) {
        Write-Warning "No .git directory at $WorkspaceFull; skipped prepare-commit-msg attribution hook."
        return
    }
    $hooksDir = Join-Path $gitDir "hooks"
    if (-not (Test-Path -LiteralPath $hooksDir)) {
        New-Item -ItemType Directory -Path $hooksDir | Out-Null
    }
    # Git runs hooks via its bundled sh even on Windows; write a POSIX shim with
    # a forward-slash assets path and LF line endings.
    $scriptPath = ($AssetsFull -replace '\\', '/') + "/hooks/strip-ai-attribution.sh"
    $shim = @(
        '#!/bin/sh',
        '# Installed by scripts/link-ai-agents.ps1 - strips AI/agent attribution from commit messages.',
        '# Source: .ai-agents/hooks/strip-ai-attribution.sh',
        ('exec sh "' + $scriptPath + '" "$1"')
    ) -join "`n"
    $hookPath = Join-Path $hooksDir "prepare-commit-msg"
    [System.IO.File]::WriteAllText($hookPath, $shim + "`n", (New-Object System.Text.UTF8Encoding $false))
    Write-Host "Installed git prepare-commit-msg attribution hook at $hookPath"
}

function Install-Runtime {
    <#
      Fetches the optional runtime binary that the wired hooks invoke by name.

      Skipped when the binary is already present, when LINK_SKIP_RUNTIME is set,
      and in CI, where a network download would make an unrelated outage look
      like a broken link script.

      Never fails the link run. The runtime is optional by design: without it
      every hook is a quiet no-op and the markdown assets work as before.
    #>
    $installer = Join-Path $PSScriptRoot 'install-runtime.ps1'

    if ($env:LINK_SKIP_RUNTIME) {
        Write-Host 'Runtime install skipped (LINK_SKIP_RUNTIME set).'
        return
    }
    if ($env:CI) {
        Write-Host 'Runtime install skipped (CI). Run scripts/install-runtime.ps1 to install it.'
        return
    }
    $existing = Get-Command vibe-agent -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "Runtime already installed: $($existing.Source)"
        return
    }
    if (-not (Test-Path -LiteralPath $installer)) {
        Write-Warning 'No install-runtime.ps1 next to this script; skipped runtime install.'
        return
    }

    Write-Host 'Installing the optional runtime binary...'
    try {
        & $installer
    } catch {
        Write-Warning "Runtime install did not complete: $($_.Exception.Message)"
        Write-Warning "This is not fatal; the toolkit works without it. Retry with: $installer"
    }
}

Install-LocalGitExclude -WorkspaceFull $workspaceFull
Install-CommitAttributionHook -AssetsFull $assetsFull -WorkspaceFull $workspaceFull
Install-Runtime

Write-Host "Links created under $workspaceFull (.claude, .cursor, .opencode, .agents) -> $assetsFull"
Write-Host "Codex custom agents synced to $workspaceFull\.codex\agents"
