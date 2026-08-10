<#
.SYNOPSIS
Install the vibe-agent toolkit into the user-level asset directories on Windows.

.DESCRIPTION
The PowerShell port of scripts/install-global.sh, and it earns its place rather
than duplicating for symmetry: Git Bash on Windows accepts `ln -s` and silently
copies, so the shell script installs 130-plus copies that go stale. PowerShell
creates real symbolic links when Developer Mode is on or the shell is elevated,
which keeps every asset live against the repository.

Both scripts write the same layout, the same `vibe-` prefix, and the same
manifest, so either can uninstall what the other installed.

The prefix is load-bearing. Claude Code resolves a skill-name collision toward
the personal level, so an unprefixed global install would make this toolkit
override a repository's own skills instead of being the fallback AGENTS.md says
it is. Prefixed, they cannot collide.

Permissions and hooks are deliberately not installed. Run link-ai-agents.ps1 in
a project to get those.

.PARAMETER DryRun
Print what would change and write nothing.

.PARAMETER Check
Report drift between installed copies and the toolkit, then exit.

.PARAMETER Uninstall
Remove exactly what the manifest records, and nothing else.

.PARAMETER Prefix
Namespace prefix. Default: vibe-

.EXAMPLE
powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1
#>
[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$Check,
    [switch]$Uninstall,
    [string]$Prefix = 'vibe-'
)

$ErrorActionPreference = 'Stop'

$Toolkit = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$Assets = Join-Path $Toolkit '.ai-agents'
if (-not (Test-Path (Join-Path $Assets 'skills'))) {
    throw "Not a toolkit checkout: $Assets\skills missing"
}

$HomeDir = if ($env:USERPROFILE) { $env:USERPROFILE } else { $HOME }
if (-not $HomeDir -or -not (Test-Path $HomeDir)) { throw 'Cannot resolve a home directory' }

$ClaudeHome = Join-Path $HomeDir '.claude'
$CursorHome = Join-Path $HomeDir '.cursor'
$CodexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HomeDir '.codex' }
$AgentsHome = Join-Path $HomeDir '.agents'
$ConfigRoot = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { Join-Path $HomeDir '.config' }
$OpencodeHome = Join-Path $ConfigRoot 'opencode'

$Manifest = Join-Path $ClaudeHome '.vibe-agent-global.manifest'
$ManifestTmp = "$Manifest.tmp"

$BeginMark = '<!-- vibe-agent:begin -->'
$EndMark = '<!-- vibe-agent:end -->'

# Forward slashes so the same text works when a POSIX-shell tool reads it.
$NativeToolkit = $Toolkit -replace '\\', '/'
$NativeAssets = $Assets -replace '\\', '/'

$script:Installed = 0
$script:Linked = 0
$script:Copied = 0

# Windows PowerShell 5.1 writes a BOM for -Encoding UTF8 and uses CRLF, and both
# break the shell port's reader: the BOM corrupts the first path and the CR makes
# every path miss by one character. The manifest is shared, so it is written as
# plain UTF-8 with LF and either script can uninstall what the other installed.
function Add-Manifest([string]$Path) {
    # One canonical form: drive letter with forward slashes. PowerShell and Git
    # Bash both resolve it, backslashes do not survive a shell here-document,
    # and the MSYS form (/c/Users/...) is meaningless to PowerShell.
    $canonical = $Path -replace '\\', '/'
    [System.IO.File]::AppendAllText($ManifestTmp, $canonical + "`n", (New-Object System.Text.UTF8Encoding $false))
}

function Test-IsLink([string]$Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    if (-not $item) { return $false }
    # A hard link shares the file's data, so it cannot drift either.
    return $item.LinkType -in @('SymbolicLink', 'Junction', 'HardLink')
}

# Try each link kind in turn and verify the result, because every one of them
# fails under conditions this script cannot detect in advance:
#
#   SymbolicLink  the best option, but Windows PowerShell 5.1 asks the kernel
#                 without SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE, so it
#                 demands elevation even when Developer Mode is on. PowerShell 7
#                 passes the flag and succeeds.
#   Junction      directories only, needs no elevation, and crosses volumes,
#                 which matters when the toolkit is on D: and the home on C:.
#                 This is what link-ai-agents.ps1 already relies on.
#   HardLink      files only, no elevation, but cannot cross a volume.
#
# A copy is the floor. It works everywhere and goes stale, so the caller is told
# how many entries ended up that way rather than being left to assume.
function Install-Entry([string]$Source, [string]$Destination) {
    if ($DryRun) { Write-Output "would install $Destination"; return }

    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Recurse -Force
    }
    $parent = Split-Path -Parent $Destination
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }

    $isDirectory = (Get-Item -LiteralPath $Source).PSIsContainer
    $kinds = if ($isDirectory) { @('SymbolicLink', 'Junction') } else { @('SymbolicLink', 'HardLink') }

    $ok = $false
    foreach ($kind in $kinds) {
        try {
            New-Item -ItemType $kind -Path $Destination -Target $Source -ErrorAction Stop | Out-Null
            $ok = ($kind -eq 'HardLink') -or (Test-IsLink $Destination)
        } catch {
            $ok = $false
        }
        if ($ok) { break }
        if (Test-Path -LiteralPath $Destination) { Remove-Item -LiteralPath $Destination -Recurse -Force }
    }

    if ($ok) {
        $script:Linked++
    } else {
        Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
        $script:Copied++
    }
    Add-Manifest $Destination
    $script:Installed++
}

# A subagent is identified by its frontmatter `name:`, not its filename, so
# namespacing one means rewriting the file. Relative links are made absolute at
# the same time: `../skills/x` resolved from ~/.claude/agents/ points at nothing.
function Get-AgentBody([string]$Source, [string]$NewName) {
    $lines = Get-Content -LiteralPath $Source -Encoding UTF8
    $out = New-Object System.Collections.Generic.List[string]
    $inFrontmatter = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        if ($i -eq 0 -and $line -eq '---') { $inFrontmatter = $true; $out.Add($line); continue }
        if ($inFrontmatter -and $line -match '^name:[ \t]') { $out.Add("name: $NewName"); continue }
        if ($inFrontmatter -and $line -eq '---') { $inFrontmatter = $false; $out.Add($line); continue }
        $out.Add([regex]::Replace($line, '\]\(\.\./([^)]+)\)', { param($m) "](" + $NativeAssets + "/" + $m.Groups[1].Value + ")" }))
    }
    return ($out -join "`n") + "`n"
}

function Install-Agent([string]$Source, [string]$Destination, [string]$NewName) {
    if ($DryRun) { Write-Output "would generate $Destination (name: $NewName)"; return }
    $parent = Split-Path -Parent $Destination
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    # No BOM: a BOM ahead of YAML frontmatter stops the loader parsing it.
    [System.IO.File]::WriteAllText($Destination, (Get-AgentBody $Source $NewName), (New-Object System.Text.UTF8Encoding $false))
    Add-Manifest $Destination
    $script:Installed++
    $script:Copied++
}

function Get-PointerText {
    @"
This machine has the vibe-agent toolkit installed at ``$NativeToolkit``.

Router: ``$NativeAssets/ROUTER.md``. Charter: ``$NativeToolkit/AGENTS.md``. Read those before applying a
toolkit default. A repository's own rules win over both.

Shared assets are installed under the ``$Prefix`` prefix.

"@
}

# A marked block keeps the user's own file intact: anything outside the markers
# is theirs and is never touched.
function Write-ManagedBlock([string]$Path) {
    if ($DryRun) { Write-Output "would write managed block into $Path"; return }
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    $block = "$BeginMark`n$(Get-PointerText)$EndMark"
    if ((Test-Path -LiteralPath $Path) -and (Select-String -LiteralPath $Path -SimpleMatch $BeginMark -Quiet)) {
        $kept = New-Object System.Collections.Generic.List[string]
        $skip = $false
        foreach ($line in (Get-Content -LiteralPath $Path -Encoding UTF8)) {
            if ($line -eq $BeginMark) { $kept.Add($block); $skip = $true; continue }
            if ($line -eq $EndMark) { $skip = $false; continue }
            if (-not $skip) { $kept.Add($line) }
        }
        [System.IO.File]::WriteAllText($Path, ($kept -join "`n") + "`n", (New-Object System.Text.UTF8Encoding $false))
    } else {
        $existing = if (Test-Path -LiteralPath $Path) { (Get-Content -Raw -LiteralPath $Path) + "`n" } else { '' }
        [System.IO.File]::WriteAllText($Path, $existing + $block + "`n", (New-Object System.Text.UTF8Encoding $false))
    }
    Add-Manifest $Path
}

# Cursor has no global AGENTS.md. Its user-level rules are .mdc files with
# frontmatter, and alwaysApply puts this one in front of every session.
function Write-CursorRule {
    $path = Join-Path $CursorHome "rules/${Prefix}toolkit.mdc"
    if ($DryRun) { Write-Output "would write $path"; return }
    $parent = Split-Path -Parent $path
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    $body = "---`ndescription: Points at the vibe-agent toolkit installed on this machine`nalwaysApply: true`n---`n`n" + (Get-PointerText)
    [System.IO.File]::WriteAllText($path, $body, (New-Object System.Text.UTF8Encoding $false))
    Add-Manifest $path
}

if ($Uninstall) {
    if (-not (Test-Path -LiteralPath $Manifest)) {
        Write-Output "Nothing to uninstall: no manifest at $Manifest"; exit 0
    }
    $removed = 0
    foreach ($entry in (Get-Content -LiteralPath $Manifest -Encoding UTF8)) {
        if ([string]::IsNullOrWhiteSpace($entry)) { continue }
        if ($entry -match '(AGENTS|CLAUDE)\.md$') {
            if (-not (Test-Path -LiteralPath $entry)) { continue }
            $kept = New-Object System.Collections.Generic.List[string]
            $skip = $false
            foreach ($line in (Get-Content -LiteralPath $entry -Encoding UTF8)) {
                if ($line -eq $BeginMark) { $skip = $true; continue }
                if ($line -eq $EndMark) { $skip = $false; continue }
                if (-not $skip) { $kept.Add($line) }
            }
            [System.IO.File]::WriteAllText($entry, ($kept -join "`n") + "`n", (New-Object System.Text.UTF8Encoding $false))
            $removed++
        } elseif (Test-Path -LiteralPath $entry) {
            Remove-Item -LiteralPath $entry -Recurse -Force
            $removed++
        }
    }
    Remove-Item -LiteralPath $Manifest -Force
    Write-Output "Removed $removed installed entries. Nothing outside the manifest was touched."
    exit 0
}

if ($Check) {
    if (-not (Test-Path -LiteralPath $Manifest)) {
        Write-Output "Not installed: no manifest at $Manifest"; exit 0
    }
    $drifted = 0
    foreach ($entry in (Get-Content -LiteralPath $Manifest -Encoding UTF8)) {
        if ([string]::IsNullOrWhiteSpace($entry)) { continue }
        if ($entry -match '(AGENTS|CLAUDE)\.md$' -or $entry -match '\.mdc$') { continue }
        if (Test-IsLink $entry) { continue }          # a link cannot drift
        if (-not (Test-Path -LiteralPath $entry)) {
            Write-Output "MISSING  $entry"; $drifted++; continue
        }

        $stem = [System.IO.Path]::GetFileNameWithoutExtension($entry)
        $plain = if ($stem.StartsWith($Prefix)) { $stem.Substring($Prefix.Length) } else { $stem }

        $skillDir = Join-Path $Assets "skills/$plain"
        $commandFile = Join-Path $Assets "commands/$plain.md"
        $agentFile = Join-Path $Assets "agents/$plain.md"

        if (Test-Path -LiteralPath $skillDir -PathType Container) {
            $a = Get-ChildItem -Recurse -File -LiteralPath $skillDir | Sort-Object Name |
                 ForEach-Object { (Get-FileHash $_.FullName -Algorithm MD5).Hash }
            $b = Get-ChildItem -Recurse -File -LiteralPath $entry | Sort-Object Name |
                 ForEach-Object { (Get-FileHash $_.FullName -Algorithm MD5).Hash }
            if (($a -join ',') -ne ($b -join ',')) { Write-Output "DRIFTED  $entry"; $drifted++ }
        } elseif (Test-Path -LiteralPath $commandFile) {
            if ((Get-FileHash $commandFile -Algorithm MD5).Hash -ne (Get-FileHash $entry -Algorithm MD5).Hash) {
                Write-Output "DRIFTED  $entry"; $drifted++
            }
        } elseif (Test-Path -LiteralPath $agentFile) {
            # Compare against what a fresh install would produce, so the
            # intended rewrite is never reported as drift.
            $expected = Get-AgentBody $agentFile "$Prefix$plain"
            $actual = [System.IO.File]::ReadAllText($entry)
            if ($expected -ne $actual) { Write-Output "DRIFTED  $entry"; $drifted++ }
        }
    }
    if ($drifted -eq 0) { Write-Output 'install-global --check: OK (no drift)'; exit 0 }
    Write-Output ''
    Write-Output "$drifted entry(ies) differ from the toolkit. Re-run: install-global.ps1"
    exit 1
}

if (-not (Test-Path -LiteralPath $ClaudeHome)) {
    New-Item -ItemType Directory -Path $ClaudeHome -Force | Out-Null
}
[System.IO.File]::WriteAllText($ManifestTmp, '', (New-Object System.Text.UTF8Encoding $false))

# Skills. These two directories reach all four tools: Claude Code reads the
# first, Codex reads only ~/.agents/skills, Cursor reads ~/.agents/skills, and
# opencode reads both.
foreach ($dir in (Get-ChildItem -Directory -LiteralPath (Join-Path $Assets 'skills'))) {
    if (-not (Test-Path -LiteralPath (Join-Path $dir.FullName 'SKILL.md'))) { continue }
    Install-Entry $dir.FullName (Join-Path $ClaudeHome "skills/$Prefix$($dir.Name)")
    Install-Entry $dir.FullName (Join-Path $AgentsHome "skills/$Prefix$($dir.Name)")
}

# Commands. No shared convention exists, so each tool gets its own directory.
foreach ($file in (Get-ChildItem -File -Filter *.md -LiteralPath (Join-Path $Assets 'commands'))) {
    $name = [System.IO.Path]::GetFileNameWithoutExtension($file.Name)
    if ($name -in @('ROUTER', 'TEMPLATE')) { continue }
    Install-Entry $file.FullName (Join-Path $ClaudeHome "commands/$Prefix$name.md")
    Install-Entry $file.FullName (Join-Path $CursorHome "commands/$Prefix$name.md")
    Install-Entry $file.FullName (Join-Path $OpencodeHome "commands/$Prefix$name.md")
    Install-Entry $file.FullName (Join-Path $CodexHome "prompts/$Prefix$name.md")
}

foreach ($file in (Get-ChildItem -File -Filter *.md -LiteralPath (Join-Path $Assets 'agents'))) {
    $name = [System.IO.Path]::GetFileNameWithoutExtension($file.Name)
    if ($name -in @('ROUTER', 'TEMPLATE', 'README')) { continue }
    Install-Agent $file.FullName (Join-Path $ClaudeHome "agents/$Prefix$name.md") "$Prefix$name"
    Install-Agent $file.FullName (Join-Path $CursorHome "agents/$Prefix$name.md") "$Prefix$name"
    Install-Agent $file.FullName (Join-Path $OpencodeHome "agents/$Prefix$name.md") "$Prefix$name"
}

# The control plane. `vibe-agent doctor` and the delivery commands need the
# workflow graphs and hook wiring, and both live under .ai-agents. Without this
# a repository that has not vendored the toolkit fails doctor on a missing
# .ai-agents/graphs even though nothing about it is repository-specific. The
# runtime probes ~/.vibe-agent last, so a repository that ships its own assets
# is never shadowed. Kept in step with GlobalToolkitDir in runtime/cmd/common.go.
Install-Entry $Assets (Join-Path $HomeDir '.vibe-agent/.ai-agents')

Write-ManagedBlock (Join-Path $CodexHome 'AGENTS.md')
Write-ManagedBlock (Join-Path $OpencodeHome 'AGENTS.md')
Write-ManagedBlock (Join-Path $ClaudeHome 'CLAUDE.md')
Write-CursorRule

if ($DryRun) {
    Remove-Item -LiteralPath $ManifestTmp -Force -ErrorAction SilentlyContinue
    Write-Output ''
    Write-Output 'Dry run. Nothing was written.'
    exit 0
}

Move-Item -LiteralPath $ManifestTmp -Destination $Manifest -Force

Write-Output ''
Write-Output "Installed $script:Installed entries: $script:Linked symlinked, $script:Copied copied."
Write-Output "  skills      $ClaudeHome\skills, $AgentsHome\skills   (all four tools)"
Write-Output "  commands    claude, cursor, opencode, codex          (as /$Prefix<name>)"
Write-Output "  subagents   generated with name: $Prefix<name>"
Write-Output "  rules       marked block in each global instructions file, plus a Cursor .mdc"
Write-Output "  manifest    $Manifest"

if ($script:Copied -gt 0) {
    Write-Output ''
    Write-Output "$script:Copied entries are copies rather than links, so they go stale when the"
    Write-Output 'toolkit changes. Re-run after editing an asset, and use -Check for drift.'
    if ($script:Linked -eq 0) {
        Write-Output ''
        Write-Output 'Nothing could be linked at all. Windows PowerShell 5.1 asks for elevation'
        Write-Output 'even with Developer Mode on; PowerShell 7 (pwsh) does not. Install pwsh or'
        Write-Output 'run this elevated to get live links.'
    }
}

Write-Output ''
Write-Output 'Permissions and hooks were not installed. To apply this repo''s policy to a'
Write-Output 'project, run link-ai-agents.ps1 in that project instead.'
