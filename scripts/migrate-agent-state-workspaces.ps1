# Discover workspaces that already have .agent-state under the given roots and
# run `vibe-agent migrate state` on each. Never scans or writes under G:.
# Skips the vibe-agent toolkit checkout itself.
#
# Usage:
#   powershell -File scripts/migrate-agent-state-workspaces.ps1 -DryRun D:\projects,D:\research
#   powershell -File scripts/migrate-agent-state-workspaces.ps1 D:\projects,D:\competitions,D:\research
#   powershell -File scripts/migrate-agent-state-workspaces.ps1 -Toolkit D:\projects\vibe-agent D:\projects

[CmdletBinding()]
param(
    [switch]$DryRun,
    [string]$Toolkit = "",
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Roots
)

$ErrorActionPreference = "Stop"

function Die([string]$Message) {
    Write-Error "migrate-agent-state-workspaces: $Message"
    exit 1
}

if (-not $Toolkit) {
    $Toolkit = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}
$Toolkit = (Resolve-Path $Toolkit).Path

if (-not $Roots -or $Roots.Count -eq 0) {
    Die "pass one or more roots (for example D:\projects D:\research)"
}

# RemainingArguments may arrive as one comma-joined string or many args.
$rootList = @()
foreach ($r in $Roots) {
    foreach ($part in ($r -split ",")) {
        $trimmed = $part.Trim()
        if ($trimmed) { $rootList += $trimmed }
    }
}

if (-not (Get-Command vibe-agent -ErrorAction SilentlyContinue)) {
    Die "vibe-agent not on PATH; run scripts/install-runtime.ps1 first"
}

function Test-ForbiddenPath([string]$Path) {
    try {
        $full = (Resolve-Path $Path).Path
    } catch {
        return $true
    }
    if ($full -match '^[gG]:') { return $true }
    if ($full -eq $Toolkit) { return $true }
    return $false
}

$targets = New-Object System.Collections.Generic.List[string]
$seen = @{}

foreach ($root in $rootList) {
    if (Test-ForbiddenPath $root) {
        Write-Host "skip root (forbidden): $root"
        continue
    }
    if (-not (Test-Path -LiteralPath $root -PathType Container)) {
        Write-Host "skip missing root: $root"
        continue
    }
    Get-ChildItem -LiteralPath $root -Directory -Filter ".agent-state" -Recurse -Depth 5 -ErrorAction SilentlyContinue |
        ForEach-Object {
            $ws = $_.Parent.FullName
            if ($seen.ContainsKey($ws)) { return }
            if (Test-ForbiddenPath $ws) {
                Write-Host "skip (forbidden or toolkit): $ws"
                return
            }
            $seen[$ws] = $true
            $targets.Add($ws) | Out-Null
        }
}

if ($targets.Count -eq 0) {
    Write-Host "no .agent-state workspaces found under: $($rootList -join ' ')"
    exit 0
}

Write-Host "toolkit: $Toolkit"
Write-Host "targets: $($targets.Count)"
$failed = 0
foreach ($ws in $targets) {
    if ($DryRun) {
        Write-Host "dry-run: would migrate $ws"
        continue
    }
    Write-Host "=== migrate state: $ws ==="
    & vibe-agent migrate state --workspace $ws --toolkit $Toolkit
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $ws" -ForegroundColor Red
        $failed++
    }
}
if ($failed -ne 0) {
    Die "$failed workspace(s) failed migrate state"
}
