package slopaudit

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/deadcode"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/source"
)

type Options struct {
	Workers int
	// DeadCode adds whole-program reachability over a Go module inside the
	// target. Off by default: the rest of the audit is a file walk that runs on
	// any language, while this shells out to the Go toolchain and fetches a
	// pinned tool on first use. A default that slow and that specific would get
	// the whole command avoided.
	DeadCode bool
	// ModuleDir is where go.mod lives relative to the target, for DeadCode.
	ModuleDir string
}

func Audit(ctx context.Context, target string, options Options) domain.Report {
	workers := options.Workers
	if workers <= 0 {
		workers = app.DefaultWorkers
	}
	scanners := []app.Scanner{source.NewScanner(workers)}
	if options.DeadCode {
		scanners = append(scanners, deadcode.NewScanner(options.ModuleDir))
	}
	auditor := app.NewAuditor(scanners, nil)
	return auditor.Audit(ctx, target)
}
