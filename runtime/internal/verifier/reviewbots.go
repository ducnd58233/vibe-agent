package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// reviewBotNames matches goal.md's own "External PR reviews" section:
// CodeRabbit, Cursor auto-review (Bugbot). A check-run name is a bot's if it
// contains one of these, case-insensitively — vendors do not standardize exact
// names across repos, and this repo's own CI shows "CodeRabbit" verbatim.
var reviewBotNames = []string{"coderabbit", "cursor", "bugbot"}

// prCheck is the shape `gh pr checks --json name,bucket` prints. bucket is
// gh's own categorization (pass, fail, pending, skipping, cancel), not a
// vibe-agent concept.
type prCheck struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket"`
}

// nonBlockingBuckets are the gh buckets that do not hold a merge back. A check
// a bot explicitly skipped (this repo's CodeRabbit does, on some base
// branches) is not evidence of an unreviewed change — it is evidence the bot
// chose not to run, which is a fact about the bot, not about the diff.
var nonBlockingBuckets = map[string]bool{"pass": true, "skipping": true}

// ReviewBots reports whether every configured external review bot's
// check-run has reached a non-blocking verdict.
//
// Built for the auto path (docs/auto-ship-reviews), where `reviews` stays
// verifier: human by default: no external reviewer exposed a queryable status
// per-check-run *content* until this, so a person read the comments. gh pr
// checks already exposes the bucket a bot's check-run landed in, which is
// enough to decide "blocking or not" without reading a single comment body.
//
// No bot check-run at all is a pass, not a failure: there is nothing to wait
// on, the same as a repo with no bots configured today has only ever had a
// human read the diff on /goal.
type ReviewBots struct{}

func (ReviewBots) Kind() string { return "reviewbots" }

func (r ReviewBots) Verify(ctx context.Context, req Request) (Result, error) {
	if req.Command == "" {
		return Result{}, errors.New("reviewbots verifier needs a command (gh pr checks --json name,bucket)")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var captured bytes.Buffer
	cmd, startErr := safexec.CommandContext(ctx, req.Command, req.Args...)
	if startErr == nil {
		cmd.Dir = req.WorkspaceRoot
		cmd.Stdout = &captured
		cmd.Stderr = &captured
	}
	runErr := startErr
	if cmd != nil {
		runErr = cmd.Run()
	}
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)

	result := Result{Check: state.Check{Source: state.SourceCIAPI, At: time.Now().UTC()}}

	if timedOut {
		result.Summary = "gh pr checks timed out waiting on review bot status"
		result.Detail = captured.String()
		return result, nil
	}

	var checks []prCheck
	if jsonErr := json.Unmarshal(captured.Bytes(), &checks); jsonErr != nil {
		// gh can exit non-zero for a reason unrelated to reviews (a build check
		// still pending) but should still print valid JSON with --json set. If it
		// did not, this is unproven, not a lie in either direction: not passed.
		result.Summary = fmt.Sprintf("gh pr checks did not produce parseable JSON (exit err: %v): %s", runErr, jsonErr)
		result.Detail = captured.String()
		return result, nil
	}

	passed, summary := decide(checks)
	result.Check.Passed = passed
	result.Summary = summary
	result.Detail = captured.String()
	return result, nil
}

// decide is the pure part of ReviewBots.Verify: given the check-runs gh
// reported, is every review bot's run non-blocking. Separated so it can be
// tested against synthetic input without shelling out to gh or needing a real
// pull request.
func decide(checks []prCheck) (passed bool, summary string) {
	var bots []prCheck
	for _, c := range checks {
		lower := strings.ToLower(c.Name)
		for _, name := range reviewBotNames {
			if strings.Contains(lower, name) {
				bots = append(bots, c)
				break
			}
		}
	}

	if len(bots) == 0 {
		return true, "no configured review bot check-run found; nothing to wait on"
	}

	var blocking []string
	for _, b := range bots {
		if !nonBlockingBuckets[b.Bucket] {
			blocking = append(blocking, fmt.Sprintf("%s (%s)", b.Name, b.Bucket))
		}
	}

	if len(blocking) > 0 {
		return false, fmt.Sprintf("%d review bot check(s) not yet non-blocking: %s", len(blocking), strings.Join(blocking, ", "))
	}
	return true, fmt.Sprintf("%d review bot check(s) all non-blocking", len(bots))
}
