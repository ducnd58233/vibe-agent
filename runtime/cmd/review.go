package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/reviewscan"
)

const reviewScanTimeout = 10 * time.Minute

func reviewCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("review needs a subcommand: scan or block")
	}
	switch args[0] {
	case "scan":
		return reviewScanCommand(args[1:])
	case "block":
		return reviewBlockCommand(args[1:])
	default:
		return fmt.Errorf("unknown review subcommand %q; try `vibe-agent review scan`", args[0])
	}
}

func reviewScanCommand(args []string) error {
	flags := newFlagSet("review scan")
	root := flags.String("root", ".", "workspace root: references are counted across all of it")
	changed := flags.Bool("changed", false, "report only files changed since the base, and mark changed blocks")
	base := flags.String("base", "", "git ref to diff against with --changed (default: upstream, then origin/HEAD, main, master)")
	asJSON := flags.Bool("json", false, "emit the report as JSON")
	minSeverity := flags.String("min-severity", string(reviewscan.SeverityLow), "drop findings below this: high, medium, low, or info")
	findingsOnly := flags.Bool("findings-only", false, "print findings without the block worklist")
	failOn := flags.String("fail-on", "", "exit non-zero when a finding at this severity or above is reported")
	workers := flags.Int("workers", 0, "parse workers (default: one per CPU)")
	paths, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}
	floor, err := reviewscan.ParseSeverity(*minSeverity)
	if err != nil {
		return err
	}
	var gate reviewscan.Severity
	if *failOn != "" {
		if gate, err = reviewscan.ParseSeverity(*failOn); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), reviewScanTimeout)
	defer cancel()
	report, err := reviewscan.Scan(ctx, *root, reviewscan.Options{
		Paths: paths, Changed: *changed, Base: *base, MinSeverity: floor, Workers: *workers,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else if err := reviewscan.WriteText(os.Stdout, report, *findingsOnly); err != nil {
		return err
	}
	if gate != "" {
		for _, finding := range report.Findings {
			if finding.Severity.AtLeast(gate) {
				return fmt.Errorf("review scan reported %s findings (--fail-on %s)", finding.Severity, gate)
			}
		}
	}
	return nil
}

func reviewBlockCommand(args []string) error {
	flags := newFlagSet("review block")
	root := flags.String("root", ".", "workspace root: references are counted across all of it")
	asJSON := flags.Bool("json", false, "emit the block as JSON")
	targets, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}
	if len(targets) != 1 {
		return fmt.Errorf("review block takes one <path>:<line> or <path>:<name>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), reviewScanTimeout)
	defer cancel()
	report, err := reviewscan.Scan(ctx, *root, reviewscan.Options{MinSeverity: reviewscan.SeverityInfo})
	if err != nil {
		return err
	}
	view, err := report.Block(targets[0])
	if err != nil {
		return err
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(view)
	}
	return reviewscan.WriteBlock(os.Stdout, view)
}
