package session

import "strings"

const (
	HostStatusTimeout = "Host timed out while generating output."
	HostStatusError   = "Host exited with an error."
	HostStatusFail    = "Host failed to produce output."
	HostStatusEmpty   = "Host produced no output."
)

// EphemeralHostStatus reports composer placeholders that must not own token counts.
func EphemeralHostStatus(body string) bool {
	switch strings.TrimSpace(body) {
	case HostStatusTimeout, HostStatusError, HostStatusFail, HostStatusEmpty:
		return true
	default:
		return false
	}
}

// IsCommandInjectionUserText reports Claude slash-command expansion the Chat tab
// should treat as thinking context, not a second user turn.
func IsCommandInjectionUserText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "<command-message>") || strings.Contains(trimmed, "<command-name>") {
		return true
	}
	if strings.Contains(trimmed, "<context>") && strings.Contains(trimmed, "</context>") {
		return true
	}
	if strings.Contains(trimmed, "ARGUMENTS:") && strings.Contains(trimmed, "## ") {
		return true
	}
	return false
}
