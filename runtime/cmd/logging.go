package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

func openServiceLogger(service string) (*slog.Logger, io.Closer, error) {
	level := os.Getenv("VIBE_LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	dir, err := observability.ResolveLogDir()
	if err != nil {
		return nil, nil, err
	}
	return observability.NewLogger(observability.Options{
		Service: service,
		Level:   level,
		Dir:     dir,
	})
}

func closeLogger(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
