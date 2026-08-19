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
    .agents/skills (Codex skills plus command adapters), .agents/commands (mirror of .ai-agents)
    .codex/agents/*.toml (Codex custom subagents, generated from .ai-agents/agents/*.md)
    minimal .claude/settings.json, .cursor/hooks.json, .codex/hooks.json only when absent

  Examples:
    powershell -File scripts/link-ai-agents.ps1

    powershell -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')

  Optional environment variables (when the matching parameter is omitted):
    LINK_WORKSPACE, LINK_ASSETS - same paths as -WorkspaceRoot / -AssetsRoot (useful from CI or wrappers).
  The script also adds generated discovery paths to <WorkspaceRoot>/.git/info/exclude when WorkspaceRoot is a
  Git repository. This keeps local links and generated Codex agent and prompt files out of Git without requiring root
  .gitignore rules in consumer repositories. Codex command prompts are generated as skills because Codex CLI
  removed custom /prompts support in 0.117.0. Use $<name> in a linked workspace, or $vibe-<name> after global install.
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
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [Parameter(Mandatory = $true)][string] $AssetsReference
    )
    $converted = $Body
    $converted = $converted.Replace('../skills/', "$AssetsReference/skills/")
    $converted = $converted.Replace('../references/', "$AssetsReference/references/")
    $converted = $converted.Replace('../stack-profiles/', "$AssetsReference/stack-profiles/")
    $converted = $converted.Replace('../commands/', "$AssetsReference/commands/")
    $converted = $converted.Replace('../agents/', "$AssetsReference/agents/")
    return @"
Codex note: this file is generated from $AssetsReference/agents. Resolve shared asset links from that assets root, not from `.codex/agents`.

$converted
"@
}

function Sync-CodexAgents {
    param(
        [Parameter(Mandatory = $true)][string] $AssetsFull,
        [Parameter(Mandatory = $true)][string] $WorkspaceFull,
        [Parameter(Mandatory = $true)][string] $AssetsReference
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
        $body = Convert-AgentBodyForCodex -Body $matches[2].Trim() -AssetsReference $AssetsReference
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

function Remove-PathSafely {
    param([Parameter(Mandatory = $true)][string] $Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    $attrs = [System.IO.File]::GetAttributes($Path)
    if (($attrs -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        cmd.exe /c "rmdir `"$Path`"" | Out-Null
    }
    else {
        Remove-Item -LiteralPath $Path -Force -Recurse
    }
}

function Convert-CommandBodyForCodexSkill {
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [Parameter(Mandatory = $true)][string] $AssetsReference
    )
    $converted = $Body
    $converted = $converted.Replace('../skills/', "$AssetsReference/skills/")
    $converted = $converted.Replace('../references/', "$AssetsReference/references/")
    $converted = $converted.Replace('../stack-profiles/', "$AssetsReference/stack-profiles/")
    $converted = $converted.Replace('../commands/', "$AssetsReference/commands/")
    $converted = $converted.Replace('../agents/', "$AssetsReference/agents/")
    return $converted
}

function Write-CodexCommandSkill {
    param(
        [Parameter(Mandatory = $true)][string] $CommandFile,
        [Parameter(Mandatory = $true)][string] $SkillRoot,
        [Parameter(Mandatory = $true)][string] $SkillName,
        [Parameter(Mandatory = $true)][string] $AssetsReference
    )
    $content = Get-Content -LiteralPath $CommandFile -Raw -Encoding UTF8
    if ($content -notmatch '(?s)\A---\r?\n(.*?)\r?\n---\r?\n(.*)\z') {
        Write-Warning "Skipping $([System.IO.Path]::GetFileName($CommandFile)): missing YAML frontmatter."
        return
    }
    $yamlBlock = $matches[1]
    $description = Get-FrontmatterField -YamlBlock $yamlBlock -FieldName 'description'
    if ([string]::IsNullOrWhiteSpace($description)) {
        $description = "Run the vibe-agent $SkillName command"
    }
    $body = Convert-CommandBodyForCodexSkill -Body $matches[2].Trim() -AssetsReference $AssetsReference
    $skillDir = Join-Path $SkillRoot $SkillName
    if (-not (Test-Path -LiteralPath $skillDir)) {
        New-Item -ItemType Directory -Path $skillDir -Force | Out-Null
    }
    $skillPath = Join-Path $skillDir 'SKILL.md'
    $skillMention = '$' + $SkillName
    $text = @"
---
name: $SkillName
description: >-
  Codex-compatible command adapter. Use only when the user explicitly mentions
  $skillMention or asks to run this vibe-agent command: $description
disable-model-invocation: true
---

# $SkillName

This is the Codex-compatible form of $AssetsReference/commands/$([System.IO.Path]::GetFileName($CommandFile)).
Codex CLI removed custom `/prompts` support in 0.117.0, so command prompts are exposed as explicit skills.

Treat any text after $skillMention as the command arguments, then follow the command prompt below.

<command_prompt>
$body
</command_prompt>
"@
    [System.IO.File]::WriteAllText($skillPath, $text, (New-Object System.Text.UTF8Encoding $false))
}

function Sync-CodexCommandSkills {
    param(
        [Parameter(Mandatory = $true)][string] $AssetsFull,
        [Parameter(Mandatory = $true)][string] $SkillRoot,
        [string] $NamePrefix = '',
        [Parameter(Mandatory = $true)][string] $AssetsReference,
        [switch] $IncludeCanonicalSkills
    )
    Remove-PathSafely -Path $SkillRoot
    New-Item -ItemType Directory -Path $SkillRoot -Force | Out-Null

    if ($IncludeCanonicalSkills) {
        Get-ChildItem -LiteralPath (Join-Path $AssetsFull 'skills') -Directory | ForEach-Object {
            Ensure-Junction (Join-Path $SkillRoot $_.Name) $_.FullName
        }
    }

    $excludeNames = @('TEMPLATE.md', 'README.md', 'ROUTER.md')
    Get-ChildItem -LiteralPath (Join-Path $AssetsFull 'commands') -Filter '*.md' -File | ForEach-Object {
        if ($excludeNames -contains $_.Name) {
            return
        }
        $commandName = [System.IO.Path]::GetFileNameWithoutExtension($_.Name)
        Write-CodexCommandSkill -CommandFile $_.FullName -SkillRoot $SkillRoot -SkillName ($NamePrefix + $commandName) -AssetsReference $AssetsReference
    }
}


function Get-CodexHome {
    if (-not [string]::IsNullOrWhiteSpace($env:CODEX_HOME)) {
        return $env:CODEX_HOME
    }
    $homeDir = if ($env:USERPROFILE) { $env:USERPROFILE } else { $HOME }
    if ([string]::IsNullOrWhiteSpace($homeDir)) {
        throw 'Cannot resolve CODEX_HOME or a home directory.'
    }
    return (Join-Path $homeDir '.codex')
}

function Remove-CodexPromptCopies {
    param(
        [Parameter(Mandatory = $true)][string] $WorkspaceFull
    )
    $workspacePrompts = Join-Path $WorkspaceFull '.codex\prompts'
    if (Test-Path -LiteralPath $workspacePrompts) {
        Remove-PathSafely -Path $workspacePrompts
    }
    $codexHome = Get-CodexHome
    $manifest = Join-Path $codexHome '.vibe-agent-prompts.manifest'
    if (Test-Path -LiteralPath $manifest) {
        Get-Content -LiteralPath $manifest -ErrorAction SilentlyContinue | ForEach-Object {
            if (-not [string]::IsNullOrWhiteSpace($_) -and (Test-Path -LiteralPath $_)) {
                Remove-Item -LiteralPath $_ -Force
            }
        }
        Remove-Item -LiteralPath $manifest -Force
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
Ensure-Junction (Join-Path $workspaceFull ".agents\commands") (Join-Path $assetsFull "commands")
$workspaceAssetsReference = $assetsFull -replace '\\', '/'
Sync-CodexCommandSkills -AssetsFull $assetsFull -SkillRoot (Join-Path $workspaceFull ".agents\skills") -NamePrefix '' -AssetsReference $workspaceAssetsReference -IncludeCanonicalSkills
Remove-CodexPromptCopies -WorkspaceFull $workspaceFull
Sync-CodexAgents -AssetsFull $assetsFull -WorkspaceFull $workspaceFull -AssetsReference $workspaceAssetsReference

function Get-ToolkitRoot {
    param([Parameter(Mandatory = $true)][string] $AssetsFull)
    return (Split-Path -Parent $AssetsFull)
}

function Join-HookCommand {
    param(
        [Parameter(Mandatory = $true)][string] $Event,
        [Parameter(Mandatory = $true)][string] $Client
    )
    $toolkitForCommand = Get-ToolkitRoot -AssetsFull $assetsFull
    return "vibe-agent hook $Event --workspace `"$workspaceFull`" --toolkit `"$toolkitForCommand`" --client $Client"
}

function Join-PythonHookCommand {
    param([Parameter(Mandatory = $true)][string] $ScriptName)
    $script = Join-Path $assetsFull "hooks\$ScriptName"
    return "python3 `"$script`""
}

function Write-JsonFile {
    param(
        [Parameter(Mandatory = $true)][string] $Path,
        [Parameter(Mandatory = $true)]$Value
    )
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    $json = $Value | ConvertTo-Json -Depth 12
    [System.IO.File]::WriteAllText($Path, $json + "`n", (New-Object System.Text.UTF8Encoding $false))
}

function CommandHook {
    param([Parameter(Mandatory = $true)][string] $Command)
    return [ordered]@{ type = 'command'; command = $Command }
}

function Install-WorkspaceHookConfigs {
    <#
      A consumer workspace usually has no .claude/settings.json, .cursor/hooks.json,
      or .codex/hooks.json. Directory links alone make commands visible, but hooks
      still have no entrypoint. Create minimal hook configs only when the file is
      absent, so an existing repository policy is never overwritten by this script.
    #>
    $claudePath = Join-Path $workspaceFull '.claude\settings.json'
    if (-not (Test-Path -LiteralPath $claudePath)) {
        $claude = [ordered]@{
            hooks = [ordered]@{
                PreToolUse = @(
                    [ordered]@{ matcher = 'WebFetch'; hooks = @((CommandHook (Join-PythonHookCommand 'sdd-cache-pre.py'))) },
                    [ordered]@{ matcher = 'Bash|Edit|Write|NotebookEdit'; hooks = @((CommandHook (Join-HookCommand 'pre-tool-use' 'claude'))) }
                )
                PostToolUseFailure = @(
                    [ordered]@{ matcher = 'Bash|Edit|Write|NotebookEdit'; hooks = @((CommandHook (Join-HookCommand 'post-tool-use-failure' 'claude'))) }
                )
                PostToolUse = @(
                    [ordered]@{ matcher = 'Bash|Edit|Write|NotebookEdit'; hooks = @((CommandHook (Join-HookCommand 'post-tool-use' 'claude'))) },
                    [ordered]@{ matcher = 'WebFetch'; hooks = @((CommandHook (Join-PythonHookCommand 'sdd-cache-post.py'))) }
                )
                Stop = @([ordered]@{ hooks = @((CommandHook (Join-HookCommand 'stop' 'claude'))) })
                SessionStart = @([ordered]@{ hooks = @((CommandHook (Join-HookCommand 'session-start' 'claude'))) })
                UserPromptSubmit = @([ordered]@{ hooks = @((CommandHook (Join-HookCommand 'user-prompt-submit' 'claude'))) })
                SubagentStop = @([ordered]@{ hooks = @((CommandHook (Join-HookCommand 'subagent-stop' 'claude'))) })
            }
            disableAllHooks = $false
        }
        Write-JsonFile -Path $claudePath -Value $claude
        Write-Host "Installed minimal Claude hook config at $claudePath"
    }

    $cursorPath = Join-Path $workspaceFull '.cursor\hooks.json'
    if (-not (Test-Path -LiteralPath $cursorPath)) {
        $cursor = [ordered]@{
            version = 1
            hooks = [ordered]@{
                sessionStart = @([ordered]@{ command = (Join-HookCommand 'session-start' 'cursor') })
                preToolUse = @(
                    [ordered]@{ matcher = 'WebFetch'; command = (Join-PythonHookCommand 'sdd-cache-pre.py') },
                    [ordered]@{ matcher = 'Write|Delete'; command = (Join-HookCommand 'pre-tool-use' 'cursor') }
                )
                beforeShellExecution = @([ordered]@{ command = (Join-HookCommand 'pre-tool-use' 'cursor') })
                postToolUse = @(
                    [ordered]@{ matcher = 'WebFetch'; command = (Join-PythonHookCommand 'sdd-cache-post.py') },
                    [ordered]@{ matcher = 'Shell|Write|Delete'; command = (Join-HookCommand 'post-tool-use' 'cursor') }
                )
                postToolUseFailure = @([ordered]@{ matcher = 'Shell|Write|Delete'; command = (Join-HookCommand 'post-tool-use-failure' 'cursor') })
                subagentStop = @([ordered]@{ command = (Join-HookCommand 'subagent-stop' 'cursor') })
                stop = @([ordered]@{ command = (Join-HookCommand 'stop' 'cursor') })
            }
        }
        Write-JsonFile -Path $cursorPath -Value $cursor
        Write-Host "Installed minimal Cursor hook config at $cursorPath"
    }

    $codexPath = Join-Path $workspaceFull '.codex\hooks.json'
    if (-not (Test-Path -LiteralPath $codexPath)) {
        $codexHooks = [ordered]@{}
        foreach ($pair in @(
            @('SessionStart', 'session-start'),
            @('UserPromptSubmit', 'user-prompt-submit'),
            @('PreToolUse', 'pre-tool-use'),
            @('PostToolUse', 'post-tool-use'),
            @('Stop', 'stop'),
            @('SubagentStop', 'subagent-stop')
        )) {
            $codexHooks[$pair[0]] = @([ordered]@{ hooks = @((CommandHook (Join-HookCommand $pair[1] 'codex'))) })
        }
        Write-JsonFile -Path $codexPath -Value ([ordered]@{ hooks = $codexHooks })
        Write-Host "Installed minimal Codex hook config at $codexPath"
    }
}

function Install-LocalGitExclude {
    param([Parameter(Mandatory = $true)][string] $WorkspaceFull)
    $excludePath = Resolve-GitPath -WorkspaceFull $WorkspaceFull -RelativePath "info/exclude"
    if ([string]::IsNullOrWhiteSpace($excludePath)) {
        Write-Warning "No .git directory at $WorkspaceFull; skipped local git exclude rules."
        return
    }
    $infoDir = Split-Path -Parent $excludePath
    if (-not (Test-Path -LiteralPath $infoDir)) {
        New-Item -ItemType Directory -Path $infoDir | Out-Null
    }
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
    $hooksDir = Resolve-GitPath -WorkspaceFull $WorkspaceFull -RelativePath "hooks"
    if ([string]::IsNullOrWhiteSpace($hooksDir)) {
        Write-Warning "No .git directory at $WorkspaceFull; skipped prepare-commit-msg attribution hook."
        return
    }
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

function Resolve-GitPath {
    param(
        [Parameter(Mandatory = $true)][string] $WorkspaceFull,
        [Parameter(Mandatory = $true)][string] $RelativePath
    )
    try {
        $path = (& git -C $WorkspaceFull rev-parse --git-path $RelativePath 2>$null | Select-Object -First 1)
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($path)) {
            return ''
        }
        if ([System.IO.Path]::IsPathRooted($path)) {
            return $path
        }
        return (Join-Path $WorkspaceFull $path)
    } catch {
        return ''
    }
}

function Get-RuntimeVersion {
    # Resolved fresh each call: the point of reporting it is that the binary on
    # PATH may have just been replaced.
    $existing = Get-Command vibe-agent -ErrorAction SilentlyContinue
    if (-not $existing) { return '' }
    try {
        return (& $existing.Source version 2>$null | Select-Object -First 1).Trim()
    } catch {
        return 'unknown'
    }
}

function Install-Runtime {
    <#
      Fetches the optional runtime binary that the wired hooks invoke by name.

      Always fetches, even when a binary is already present. It used to skip in
      that case, which meant a consumer who installed once never got another
      update: the hooks kept calling a binary that fell further behind the
      configs registering them. That failure is invisible from the outside,
      because a stale binary answers the events it knows and refuses the rest.

      Skipped only when LINK_SKIP_RUNTIME is set, and in CI, where a network
      download would make an unrelated outage look like a broken link script.

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
    if (-not (Test-Path -LiteralPath $installer)) {
        Write-Warning 'No install-runtime.ps1 next to this script; skipped runtime install.'
        return
    }

    $before = Get-RuntimeVersion
    if ($before) {
        Write-Host "Refreshing the runtime binary (installed: $before)..."
    } else {
        Write-Host 'Installing the optional runtime binary...'
    }

    try {
        & $installer
    } catch {
        Write-Warning "Runtime install did not complete: $($_.Exception.Message)"
        Write-Warning "This is not fatal; the toolkit works without it. Retry with: $installer"
        return
    }

    # Report the change rather than only the fact of a download, so an unchanged
    # version is visible as unchanged instead of looking like work was done.
    if ($before) {
        $after = Get-RuntimeVersion
        if ($after -eq $before) {
            Write-Host "Runtime already current: $after"
        } elseif ($after) {
            Write-Host "Runtime updated: $before -> $after"
        }
    }
}

function Emit-PluginManifests {
    param([Parameter(Mandatory = $true)][string] $WorkspaceFull)
    $pluginName = 'vibe-agent'
    $pluginDesc = 'Domain-agnostic agent workflows: skills, commands, hooks, and delivery graphs.'

    # Claude Code plugin
    $claudePluginDir = Join-Path $WorkspaceFull '.claude-plugin'
    if (-not (Test-Path -LiteralPath $claudePluginDir)) {
        New-Item -ItemType Directory -Path $claudePluginDir -Force | Out-Null
    }
    Write-JsonFile -Path (Join-Path $claudePluginDir 'plugin.json') -Value ([ordered]@{
        name = $pluginName
        description = $pluginDesc
    })
    Write-JsonFile -Path (Join-Path $claudePluginDir 'marketplace.json') -Value ([ordered]@{
        name = $pluginName
        owner = [ordered]@{ name = $pluginName }
        plugins = @([ordered]@{
            name = $pluginName
            source = './'
            description = 'Skills, slash commands, hooks, and goal-delivery graphs. Canonical assets live under .ai-agents/.'
        })
    })

    # Codex plugin
    $codexPluginDir = Join-Path $WorkspaceFull '.codex-plugin'
    if (-not (Test-Path -LiteralPath $codexPluginDir)) {
        New-Item -ItemType Directory -Path $codexPluginDir -Force | Out-Null
    }
    Write-JsonFile -Path (Join-Path $codexPluginDir 'plugin.json') -Value ([ordered]@{
        name = $pluginName
        description = 'Domain-agnostic agent workflows: skills and hooks for Codex.'
    })

    # Cursor plugin (host-specific)
    $cursorPluginDir = Join-Path $WorkspaceFull '.cursor-plugin'
    if (-not (Test-Path -LiteralPath $cursorPluginDir)) {
        New-Item -ItemType Directory -Path $cursorPluginDir -Force | Out-Null
    }
    Write-JsonFile -Path (Join-Path $cursorPluginDir 'plugin.json') -Value ([ordered]@{
        '$schema' = 'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json'
        name = $pluginName
        description = 'Domain-agnostic agent workflows for Cursor Agent Plugins.'
    })

    # Agent Plugins 1.0.0 (root, portable)
    Write-JsonFile -Path (Join-Path $WorkspaceFull 'plugin.json') -Value ([ordered]@{
        '$schema' = 'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json'
        name = $pluginName
        description = $pluginDesc
    })

    Write-Host "Plugin manifests emitted under $WorkspaceFull"
}

Install-LocalGitExclude -WorkspaceFull $workspaceFull
Install-CommitAttributionHook -AssetsFull $assetsFull -WorkspaceFull $workspaceFull
Install-WorkspaceHookConfigs
Emit-PluginManifests -WorkspaceFull $workspaceFull
Install-Runtime

Write-Host "Links created under $workspaceFull (.claude, .cursor, .opencode, .agents) -> $assetsFull"
Write-Host "Codex custom agents synced to $workspaceFull\.codex\agents"
Write-Host "Codex command skills synced to $workspaceFull\.agents\skills as <name>"
Write-Host "Codex command form in a linked workspace: `$<name> (custom /prompts and top-level /vibe-* are not available in Codex CLI 0.147.0)"
