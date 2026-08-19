//go:build !windows && !darwin

package hostpick

import (
	"context"
	"errors"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func pick(ctx context.Context, kind domain.PickKind) (string, error) {
	args := []string{"--file-selection"}
	if kind == domain.PickFolder {
		args = append(args, "--directory")
	}
	cmd, err := safexec.CommandContext(ctx, "zenity", args...)
	if err != nil {
		return "", domain.ErrPickUnavailable
	}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *safexec.ExitError
		if errors.As(err, &exitErr) {
			return "", domain.ErrPickCancelled
		}
		return "", domain.ErrPickUnavailable
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", domain.ErrPickCancelled
	}
	return path, nil
}
