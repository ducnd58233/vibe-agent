// Command vibe-agent is the outer-loop control plane for the vibe-agent
// toolkit. It owns run state, graph transitions, and verification evidence.
//
// It does not own the inner loop. Claude Code, Codex, Cursor, and opencode keep
// their own model and tool loops; this binary decides what happens between
// their turns.
//
// One file per command: run.go, checkpoint.go, graph.go, mcp.go, hook.go, and
// doctor.go. Shared flag and formatting helpers live in common.go.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `vibe-agent - outer-loop control plane

Usage:
  vibe-agent run start --slug <slug> --goal <text> [--graph <id>]
  vibe-agent run status --slug <slug> [--json]
  vibe-agent checkpoint --slug <slug> --check <name> --source <source> [--passed|--failed|--skipped]
  vibe-agent graph validate [--graph <id>]
  vibe-agent mcp serve
  vibe-agent hook <session-start|user-prompt-submit|stop|subagent-stop> [--client claude|cursor]
  vibe-agent doctor
  vibe-agent version

Run state is written to tmp/<slug>/manifest.json with an append-only log at
tmp/<slug>/events.ndjson, both under the workspace root and both gitignored.

Evidence sources: exit_code, file_assert, ci_api, human_event. There is no
source for model assertion, so nothing can mark its own work complete.

Global flags:
  --workspace <dir>   Workspace root (default: current directory)
  --toolkit <dir>     Toolkit root holding .ai-agents (default: workspace root)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "vibe-agent: %v\n", err)
		os.Exit(1)
	}
}

// run dispatches to one command. Each case lives in its own file, so this stays
// a routing table rather than a place logic accumulates.
func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "checkpoint":
		return checkpointCommand(args[1:])
	case "graph":
		return graphCommand(args[1:])
	case "mcp":
		return mcpCommand(args[1:])
	case "hook":
		return hookCommand(args[1:])
	case "doctor":
		return doctorCommand(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q; try `vibe-agent help`", args[0])
	}
}
