package hostpick

import (
	"errors"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

// pickSubprocessErr maps a failed pick subprocess to cancel or unavailable.
// Only the documented cancel exit code counts as a user dismissal.
func pickSubprocessErr(err error, cancelExitCode int) error {
	var exitErr *safexec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == cancelExitCode {
		return domain.ErrPickCancelled
	}
	return domain.ErrPickUnavailable
}
