package slopaudit

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
	goscanner "github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/golang"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/tool"
)

type Options struct {
	Workers int
}

func Audit(ctx context.Context, target string, options Options) domain.Report {
	scanner := goscanner.NewScanner(options.Workers)
	adapters := tool.NewAdapters(tool.OSExecutor{})
	auditor := app.NewAuditor([]app.Scanner{scanner}, adaptersToPorts(adapters))
	return auditor.Audit(ctx, target)
}

func adaptersToPorts(adapters []tool.Adapter) []app.Adapter {
	ports := make([]app.Adapter, 0, len(adapters))
	for _, adapter := range adapters {
		ports = append(ports, adapter)
	}
	return ports
}
