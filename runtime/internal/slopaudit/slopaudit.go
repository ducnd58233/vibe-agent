package slopaudit

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/source"
)

type Options struct {
	Workers int
}

func Audit(ctx context.Context, target string, options Options) domain.Report {
	scanner := source.NewScanner(options.Workers)
	auditor := app.NewAuditor([]app.Scanner{scanner}, nil)
	return auditor.Audit(ctx, target)
}
