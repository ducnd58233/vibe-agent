package session

import "github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"

const redactedMarker = redact.Marker

// RedactText replaces credential-shaped substrings and marks the result.
func RedactText(text string) string {
	return redact.Text(text)
}

// TruncateCommand bounds tool command text before redaction.
func TruncateCommand(command string) string {
	return redact.TruncateCommand(command)
}
