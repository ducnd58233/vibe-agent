package checkpoint

import (
	"fmt"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// DuplicateReplayCLI is printed when a CLI checkpoint or verify replay is detected.
func DuplicateReplayCLI() string {
	return "Already recorded. This exact evidence was the last checkpoint, so nothing advanced."
}

// DuplicateReplayNote is returned in MCP tool output when a checkpoint replay is detected.
func DuplicateReplayNote() string {
	return "This exact evidence was the last checkpoint recorded, so nothing advanced."
}

// FormatFailureClasses joins the allowed failure classes for validation errors.
func FormatFailureClasses() string {
	classes := state.FailureClasses()
	parts := make([]string, len(classes))
	for i, class := range classes {
		parts[i] = string(class)
	}
	return strings.Join(parts, ", ")
}

// ValidateBlockerOutcome returns an error when blocker and class are inconsistent.
func ValidateBlockerOutcome(blocker, class string) error {
	if class != "" && blocker == "" {
		return fmt.Errorf("--class describes a failure, so it needs --blocker")
	}
	if blocker == "" {
		return nil
	}
	if class == "" {
		return fmt.Errorf("--blocker needs --class; use one of %s", FormatFailureClasses())
	}
	if !FailureClassOK(class) {
		return fmt.Errorf("--class %q is not a failure class; use one of %s", class, FormatFailureClasses())
	}
	return nil
}

// FailureClassOK reports whether name is a known failure class.
func FailureClassOK(name string) bool {
	want := state.FailureClass(name)
	for _, class := range state.FailureClasses() {
		if class == want {
			return true
		}
	}
	return false
}
