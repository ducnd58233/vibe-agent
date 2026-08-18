package session

import (
	"regexp"
	"strings"
)

const (
	redactedMarker  = "[REDACTED]"
	maxCommandRunes = 512
)

// credentialPlaceholder is written over matched secret shapes in logs.
var credentialPlaceholder = "<" + "credential" + ">"

// credentialPatterns mirror the narrow live-credential shapes in harness/gate.go
// and memory/policy.go. Session logs are rendered in the UI, so the same shapes
// are replaced at append time.
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*\S{16,}`),
}

// RedactText replaces credential-shaped substrings and marks the result.
func RedactText(text string) string {
	if text == "" {
		return text
	}
	redacted := false
	out := text
	for _, pattern := range credentialPatterns {
		if pattern.MatchString(out) {
			out = pattern.ReplaceAllString(out, credentialPlaceholder)
			redacted = true
		}
	}
	if redacted && !strings.Contains(out, redactedMarker) {
		out = strings.TrimSpace(out + " " + redactedMarker)
	}
	return out
}

// TruncateCommand bounds tool command text before redaction.
func TruncateCommand(command string) string {
	runes := []rune(strings.TrimSpace(command))
	if len(runes) <= maxCommandRunes {
		return string(runes)
	}
	return string(runes[:maxCommandRunes]) + "..."
}
