package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch"
)

// fetchBudget is how much of a session a fetched document may take by default.
//
// Four thousand tokens is a long article's worth of prose. Past that a page is
// usually being read for one section, and the clip plus the handle is the better
// shape than the whole thing.
const fetchBudget = 4000

// commandTimeout bounds the whole command: the request, and the probe for a text
// route that runs when the request comes back with something that is not the
// page. A command that hangs is worse than one that reports a timeout, because
// the session waits on it.
const commandTimeout = 60 * time.Second

// fetchCommand retrieves a URL or a file and prints the text an agent needs.
//
// The saving comes from two places and both are reported rather than claimed.
// Extraction drops the markup, scripts, and navigation that are the bulk of a
// page. The cache means the second ask, in this session or a later one, costs no
// request and no re-extraction.
//
// What it prints is a clip plus a handle: the document up to the budget, then
// how much was left and where the whole of it sits. An agent told 400 lines
// remain can ask for them; one told nothing assumes it read the page.
func fetchCommand(args []string) error {
	flags := newFlagSet("fetch")
	paths := addRootFlags(flags)
	budget := flags.Int("budget", fetchBudget, "approximate token budget for the printed text")
	refresh := flags.Bool("refresh", false, "ignore any cached copy and ask the source again")
	asJSON := flags.Bool("json", false, "emit the whole document as JSON, ignoring the budget")
	// Go's flag package stops at the first operand, so `fetch <url> --budget 300`
	// would read the flags as two more operands. Parsing what precedes the
	// operand and then what follows it accepts either order, which matters
	// because the natural way to write this puts the URL first.
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return fmt.Errorf("fetch needs a URL or a file path")
	}
	source := rest[0]
	if err := flags.Parse(rest[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("fetch takes one source, got %d: %s",
			len(rest), strings.Join(rest, " "))
	}

	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	cacheRoot := fetchCacheRoot(workspaceRoot, paths.InsideAWorkspace())

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	doc, cached, err := fetch.Get(ctx, cacheRoot, source, fetch.Options{Refresh: *refresh})
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(doc)
	}

	// An asset is a path, not a page. Printing the budget line and the saving
	// percentage under it would be measuring the wrong thing: nothing was
	// extracted and nothing was saved by not extracting it.
	if doc.Status == fetch.StatusAsset {
		fmt.Println(doc.Text)
		return nil
	}

	if doc.Title != "" {
		fmt.Printf("# %s\n\n", doc.Title)
	}
	// Said on stderr and stated plainly, because the alternative is an agent
	// answering from a blank page while the summary line congratulates it on a
	// 99% saving. A bot check never reaches here: that is refused outright.
	if doc.Status != fetch.StatusOK {
		problem := "parsed cleanly and carried no readable text"
		if doc.Status == fetch.StatusThin {
			problem = "returned far more markup and script than text, which is the shape " +
				"of a page shell rather than a page"
		}
		fmt.Fprintf(os.Stderr, "vibe-agent: %s %s. Do not answer from this document. %s.\n",
			source, problem, fetch.Advise(ctx, source,
				"It is almost certainly built with JavaScript, which no AI crawler runs"))
	}
	clipped, omitted := fetch.Clip(doc.Text, *budget)
	fmt.Println(clipped)

	if omitted > 0 {
		fmt.Printf("\n[%d more lines. Re-run with --budget to raise the limit, or --json for all of it.]\n",
			omitted)
	}
	fmt.Println(summary(doc, cached, fetch.CacheDir(cacheRoot)))
	return nil
}

// summary states what this cost and what it saved, in the terms the numbers
// actually support.
//
// Both returns carry a sensitive-data-guard opt-out. The guard pairs a call that
// formats output with an argument named like a credential, and "tokens" here is
// the model's unit of text. Every value interpolated is an integer or a
// directory path, and the marker has to sit on the flagged line itself.
func summary(doc fetch.Document, cached bool, cacheDir string) string {
	origin := "fetched"
	if cached {
		origin = "from cache, no request made"
	}
	extracted := fetch.EstimateTokens(doc.Text)
	if doc.OriginalBytes == 0 {
		return fmt.Sprintf("\n[%s; ~%d tokens; cached in %s]", origin, extracted, cacheDir) // sensitive-data-guard: allow - model tokens, not credentials
	}
	before := (doc.OriginalBytes + fetch.CharsPerToken - 1) / fetch.CharsPerToken
	saved := 100 - (extracted * 100 / max(before, 1))
	return fmt.Sprintf("\n[%s; ~%d tokens from ~%d raw, %d%% smaller; cached in %s]", // sensitive-data-guard: allow - model tokens, not credentials
		origin, extracted, before, saved, cacheDir)
}

// fetchCacheRoot picks the directory the fetch cache hangs off.
//
// Inside a workspace that is the workspace, where .agent-state is gitignored
// and a consumer expects to find it. Outside one it used to be the current
// working directory, which meant a fetch from a notes folder created an
// .agent-state there - state in a place that is not a workspace, and a cache
// nothing would ever hit again, because the next fetch from a different folder
// started a fresh one. The token budget the feature exists to protect was being
// spent re-fetching pages already on disk somewhere else.
//
// The platform cache directory is one place, so it persists and is reused. A
// machine that cannot name one falls back to the old behaviour rather than
// failing a fetch over where to put a cache.
func fetchCacheRoot(workspaceRoot string, insideWorkspace bool) string {
	if insideWorkspace {
		return workspaceRoot
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return workspaceRoot
	}
	return filepath.Join(dir, "vibe-agent")
}
