package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

const (
	slopAuditTimeout = 2 * time.Minute
	unsetFailOn      = -1
)

func slopCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("slop needs a subcommand: audit")
	}
	switch args[0] {
	case "audit":
		return slopAuditCommand(args[1:])
	default:
		return fmt.Errorf("unknown slop subcommand %q; try `vibe-agent slop audit`", args[0])
	}
}

func slopAuditCommand(args []string) error {
	flags := newFlagSet("slop audit")
	workers := flags.Int("workers", app.DefaultWorkers, "number of Go source scan workers")
	failOn := flags.Int("fail-on", unsetFailOn, "exit non-zero when score is greater than this value")
	asJSON := flags.Bool("json", false, "emit the report as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	target := "."
	if len(rest) > 0 {
		target = rest[0]
		if err := flags.Parse(rest[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("slop audit takes one path, got %d: %s", len(rest), strings.Join(rest, " "))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), slopAuditTimeout)
	defer cancel()

	report := slopaudit.Audit(ctx, target, slopaudit.Options{Workers: *workers})
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else {
		printSlopReport(report)
	}

	if *failOn != unsetFailOn && report.Score > *failOn {
		return fmt.Errorf("slop score %d exceeds --fail-on %d", report.Score, *failOn)
	}
	return nil
}

func printSlopReport(report domain.Report) {
	fmt.Printf("slop audit: %s\n", report.Target)
	fmt.Printf("  status   %s\n", report.Status)
	fmt.Printf("  score    %d/%d\n", report.Score, domain.MaxScore)
	fmt.Printf("  findings %d\n", len(report.Findings))
	fmt.Printf("  files    %d\n", report.Summary.FilesScanned)
	fmt.Printf("  lines    %d\n", report.Summary.LinesScanned)
	fmt.Printf("  tree     %d parsed, %d failed\n", report.Summary.TreeSitterParsed, report.Summary.TreeSitterFailures)
	fmt.Printf("  parser   %s\n", report.Summary.Parser)
	fmt.Printf("  scoring  %s\n", report.Summary.Scoring)
	if len(report.Findings) > 0 {
		fmt.Println()
		for _, finding := range report.Findings {
			fmt.Printf("  %s:%d  %s  %s  %s\n", finding.Path, finding.Line, finding.Severity, finding.Rule, finding.Message)
		}
	}
	if len(report.Adapters) > 0 {
		fmt.Println()
		fmt.Println("adapters:")
		for _, adapter := range report.Adapters {
			reason := ""
			if adapter.Reason != "" {
				reason = " - " + adapter.Reason
			}
			fmt.Printf("  %s  %s%s\n", adapter.Name, adapter.Status, reason)
		}
	}
}
