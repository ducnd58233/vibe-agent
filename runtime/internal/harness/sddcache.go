package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// The WebFetch cache pair stayed in Python when the other eight hooks moved
// into this binary, because it needs a process of its own. What did not survive
// the move is the assumption underneath its wiring: each host config named the
// script by a relative path, which resolves against whatever directory that
// host runs a hook in.
//
// Only Claude Code documents that directory, through ${CLAUDE_PROJECT_DIR}.
// Cursor and Codex publish nothing, so on those hosts the interpreter was being
// handed a path resolved from an unknown starting point. The failure is quiet:
// python reports a missing file, the host logs it out of sight, and the cache
// silently never runs.
//
// Calling the script from here fixes it for every host at once. This binary
// discovers its own workspace and toolkit root, so it can name the script
// absolutely, and it can hand the script the project root the script itself
// wants: sdd-cache-pre.py reads CLAUDE_PROJECT_DIR and falls back to its own
// cwd, which had the same defect one layer down.
//
// The delegation is deliberately thin. It forwards the payload unread, relays
// the script's own output, and translates a refusal into this package's
// BlockError so the same cache hit reaches the model correctly on all four
// hosts instead of only on Claude.

// sddCacheTool is the tool whose calls the cache wraps.
const sddCacheTool = "WebFetch"

// sddCacheTimeout bounds a revalidation request. The script makes one HTTP HEAD
// call, so this is generous rather than tight; the point is that a hung network
// cannot hold a tool call open indefinitely.
const sddCacheTimeout = 20 * time.Second

// sddCacheBlockExit is the status the script uses to serve cached content
// instead of letting the fetch proceed. It is Claude Code's convention, which
// the script was written against.
const sddCacheBlockExit = 2

// pythonCandidates are the interpreter names to try, in order.
//
// Two, because "python3" is absent on many Windows installations while
// "python" is the launcher shim, and the reverse is true on distributions that
// ship only python3. Trying both keeps the cache working without asking a
// person to configure an interpreter path.
var pythonCandidates = []string{"python3", "python"}

// sddCache runs one half of the WebFetch cache and returns a refusal if the
// script served cached content.
//
// A missing interpreter, a missing script, or an unreadable payload all return
// nil. The cache is an optimisation, and a hook that fails a session because an
// optional accelerator is not installed is worse than one that quietly does not
// accelerate.
func sddCache(req Request, body payload, script string) *BlockError {
	if body.ToolName != sddCacheTool {
		return nil
	}
	path := filepath.Join(req.ToolkitRoot, ".ai-agents", "hooks", script)
	if _, err := os.Stat(filepath.Clean(path)); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), sddCacheTimeout)
	defer cancel()

	cmd, err := findPython(ctx, path)
	if err != nil {
		return nil
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(body.raw)
	// Captured and discarded, deliberately. The script's contract is stderr
	// plus an exit status, and both scripts hold to it. Forwarding stdout as
	// well looked harmless and was a corruption path: this writer is the one
	// deliverBlock writes the host's JSON to, so anything the script printed
	// would arrive glued to the front of it and every host would fail to parse
	// the reply. Discarding it keeps one writer with one format.
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// The script resolves the cache directory from this, and falls back to its
	// own working directory without it. Passing the root this binary already
	// discovered is what stops the fallback from being reached.
	cmd.Env = append(os.Environ(),
		"CLAUDE_PROJECT_DIR="+req.WorkspaceRoot,
		workspace.EnvSDDCacheDir+"="+workspace.SDDCacheDir(req.WorkspaceRoot),
	)

	runErr := cmd.Run()

	var exit *safexec.ExitError
	if errors.As(runErr, &exit) && exit.ExitCode() == sddCacheBlockExit {
		// The script's refusal text is its entire payload: it prints the cached
		// page to stderr and expects the host to hand that back to the model.
		// Routing it through BlockError means every host's envelope carries it,
		// rather than only the one whose convention the script was written for.
		return &BlockError{Reason: stderr.String()}
	}
	return nil
}

// findPython builds the command with the first interpreter that resolves.
func findPython(ctx context.Context, script string) (*safexec.Cmd, error) {
	for _, name := range pythonCandidates {
		cmd, err := safexec.CommandContext(ctx, name, script)
		if err == nil {
			return cmd, nil
		}
	}
	return nil, fmt.Errorf("no python interpreter on PATH (tried %v)", pythonCandidates)
}
