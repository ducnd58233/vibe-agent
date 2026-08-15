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
	"errors"
	"fmt"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `vibe-agent - outer-loop control plane

Usage:
  vibe-agent run start --slug <slug> --goal <text> [--graph <id>]
  vibe-agent run status --slug <slug> [--json]
  vibe-agent run flag --slug <slug> (--set|--clear) <guard> [--note <text>]
  vibe-agent verify --slug <slug> [--dry-run]
  vibe-agent checkpoint --slug <slug> --check <name> --source <source> [--passed|--failed|--skipped]
  vibe-agent graph validate [--graph <id>]
  vibe-agent fetch <url|path> [--budget <tokens>] [--json] [--refresh]
  vibe-agent mcp serve
  vibe-agent hook <session-start|user-prompt-submit|pre-tool-use|post-tool-use|post-tool-use-failure|stop|subagent-stop> [--client claude|cursor]
  vibe-agent hook --events
  vibe-agent memory list [--status <status>]
  vibe-agent memory confirm --id <id>
  vibe-agent memory forget --id <id>
  vibe-agent doctor
  vibe-agent eval routing [--trials N] [--jobs N] [--runner codex|claude|cursor|opencode|all] [--only <text>]
  vibe-agent version

Run state is written to tmp/<slug>/manifest.json with an append-only log at
tmp/<slug>/events.ndjson, both under the workspace root and both gitignored.


"fetch" reads a URL or a file and prints the text without the markup, scripts,
and navigation that are most of a page. Documents are cached under
.agent-state/fetch/, so asking twice costs one request. Output is clipped to a
budget and says how many lines were left, rather than implying the page fit.

Evidence sources: exit_code, file_assert, ci_api, human_event. There is no
source for model assertion, so nothing can mark its own work complete.

A verifier node's check comes from "verify", which runs what vibe-checks.yaml at
the workspace root declares for that check. "checkpoint" cannot write it: a
--source argument names a kind of evidence without producing any. The one
exception is a check the plan declares "verifier: human", which is a tracked
diff rather than a flag.

A check the plan does not declare is skipped, with the plan named as the reason.
A skipped check satisfies a guard only where the graph sets acceptsSkipped, and
"run flag" writes a scope flag only while the run sits at a human gate.

Hooks call this binary by name, so they run whatever is on PATH rather than the
source it was built from. Run "make install" after changing the runtime, and
"doctor" to confirm the binary on PATH handles every hook the host configs
register.

Hooks mostly inform. Two of them refuse:

  pre-tool-use  exits 2 on a push to a protected branch or a pull request merge
                while no active run has recorded merge_approved, and on any
                write to a run's own manifest or event log.
  stop          returns decision "block" while a run sits mid-graph with nothing
                recorded, once per turn, and never for a run awaiting a human.

Only confirmed memories are retrieved into a session. Use "memory list" to see
what is stored and "memory confirm" to vouch for one yourself.

"eval routing" asks a model to route each intent in
.ai-agents/references/routing-evals.md using only the router tables, then scores
the answers against the asset each row names. It calls a model, so it is not
part of "doctor" and not part of CI: run it locally or nightly. It reports
pass^k, the rate of passing every trial, and exits non-zero only with --require.
The default runner is "codex". Runner presets include:

  codex     codex exec --ephemeral --sandbox read-only --json -
  claude    claude -p
  cursor    cursor-agent --print --mode ask --trust
  opencode  opencode run
  all       all presets above

Pass --runner more than once, or comma-separate names, to compare hosts:
  vibe-agent eval routing --runner codex --runner claude

Global flags:
  --workspace <dir>   Workspace root (default: current directory)
  --toolkit <dir>     Toolkit root holding .ai-agents (default: workspace root)
`

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}

	// Exit 2 is the only status a host treats as a hard block, and stderr is
	// what it hands back to the model. Printed without the program prefix,
	// because the text is an instruction to the agent rather than a report of a
	// tool failure.
	var blocked *harness.BlockError
	if errors.As(err, &blocked) {
		fmt.Fprintln(os.Stderr, blocked.Reason)
		os.Exit(2)
	}

	fmt.Fprintf(os.Stderr, "vibe-agent: %v\n", err)
	os.Exit(1)
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
	case "verify":
		return verifyCommand(args[1:])
	case "graph":
		return graphCommand(args[1:])
	case "fetch":
		return fetchCommand(args[1:])
	case "mcp":
		return mcpCommand(args[1:])
	case "hook":
		return hookCommand(args[1:])
	case "memory":
		return memoryCommand(args[1:])
	case "doctor":
		return doctorCommand(args[1:])
	case "eval":
		return evalCommand(args[1:])
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
