package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

func openServiceLogger(service string) (*slog.Logger, io.Closer, error) {
	level := os.Getenv(observability.EnvLogLevel)
	if level == "" {
		level = observability.DefaultLogLevel
	}
	return observability.NewLogger(observability.Options{
		Service: service,
		Level:   level,
		Dir:     "", // ResolveLogDir inside NewLogger when empty
	})
}

func closeLogger(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
