package observability

import (
	"context"
	"runtime/debug"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// LogError records an error with a redacted message and stack in the JSON log file.
func LogError(l Logger, msg string, err error) {
	if l == nil || err == nil {
		return
	}
	l.Error(msg,
		"error", session.RedactText(err.Error()),
		"stack", session.RedactText(string(debug.Stack())),
	)
}

// LogErrorContext is LogError with a request context.
func LogErrorContext(ctx context.Context, l Logger, msg string, err error) {
	if l == nil || err == nil {
		return
	}
	l.ErrorContext(ctx, msg,
		"error", session.RedactText(err.Error()),
		"stack", session.RedactText(string(debug.Stack())),
	)
}
