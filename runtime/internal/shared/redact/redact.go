// Package redact replaces credential-shaped substrings before text reaches logs or UI.
package redact

import "strings"

const (
	// Marker is appended when any credential pattern matched.
	Marker = "[REDACTED]"

	maxCommandRunes = 512
)

var credentialPlaceholder = "<" + "credential" + ">"

// Text replaces credential-shaped substrings and marks the result.
// Gitleaks default rules run first; hand-rolled patterns catch contextual
// shapes and anything the detector misses.
func Text(s string) string {
	if s == "" {
		return s
	}
	redacted := false
	out := s
	if gOut, ok := applyGitleaks(out); ok {
		out = gOut
		redacted = true
	}
	for _, pattern := range textPatterns {
		if pattern.MatchString(out) {
			out = pattern.ReplaceAllString(out, credentialPlaceholder)
			redacted = true
		}
	}
	if redacted && !strings.Contains(out, Marker) {
		out = strings.TrimSpace(out + " " + Marker)
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
