#!/usr/bin/env pwsh
# Strip AI/agent attribution from a commit message, in place.
#
# PowerShell equivalent of strip-ai-attribution.sh. The git prepare-commit-msg
# hook installed by scripts/link-ai-agents.* calls the .sh version (git runs
# hooks via sh on every platform). This .ps1 is the parallel implementation for
# PowerShell-driven environments and manual runs. Works on Windows PowerShell
# 5.1 and PowerShell 7+.
#
# Usage: pwsh -File strip-ai-attribution.ps1 <commit-msg-file>
#
# Removes AI co-author trailers (keeps human co-authors), "Generated with ..."
# lines, and robot-emoji attribution. Non-blocking: any error exits 0.

param([string] $Path)

try {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        exit 0
    }

    $agent = '(?i)claude|anthropic|cursor|codex|openai|chatgpt|gpt-?[0-9]|copilot|opencode|noreply@anthropic\.com|ai assistant|bot'
    $robot = [System.Char]::ConvertFromUtf32(0x1F916)

    $lines = [System.IO.File]::ReadAllLines($Path)
    $kept = New-Object 'System.Collections.Generic.List[string]'

    foreach ($line in $lines) {
        $drop = $false
        $hasAgent = $line -match $agent
        $hasRobot = $line.Contains($robot)

        # Co-authored-by trailer attributed to an AI assistant (human co-authors stay).
        if ($line -match '(?i)^\s*co-authored-by\s*:' -and ($hasAgent -or $line -match '(?i)\bbot\b')) {
            $drop = $true
        }
        # "generated/created/written/authored with ..." attribution, guarded by an
        # agent signature or the robot emoji so plain prose is safe.
        if ($line -match '(?i)(generated|created|written|authored)\s+with' -and ($hasAgent -or $hasRobot)) {
            $drop = $true
        }
        # Bare robot-emoji attribution line that also names an agent.
        if ($hasRobot -and $hasAgent) {
            $drop = $true
        }

        if (-not $drop) { $kept.Add($line) }
    }

    while ($kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($kept[$kept.Count - 1])) {
        $kept.RemoveAt($kept.Count - 1)
    }

    $out = if ($kept.Count -gt 0) { ($kept -join "`n") + "`n" } else { "" }
    [System.IO.File]::WriteAllText($Path, $out, (New-Object System.Text.UTF8Encoding $false))
}
catch {
    exit 0
}
exit 0
