package observability

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
)

// LogError records an error with a redacted message and stack in the JSON log file.
func LogError(l Logger, msg string, err error) {
	if l == nil || err == nil {
		return
	}
	l.Error(msg, errorLogAttrs(err)...)
}

func errorLogAttrs(err error) []any {
	return []any{
		"error", redact.Text(err.Error()),
		"stack", redact.Text(string(debug.Stack())),
	}
}

// LogPanicRecovered records a panic without logging the panic value (may hold secrets).
func LogPanicRecovered(ctx context.Context, l Logger, recovered any) {
	if l == nil || recovered == nil {
		return
	}
	l.ErrorContext(ctx, "panic recovered",
		"panic_type", fmt.Sprintf("%T", recovered),
		"stack", redact.Text(string(debug.Stack())),
	)
}
