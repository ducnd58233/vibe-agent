package hostpick

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func TestPickSubprocessErrMapsCancelAndUnavailable(t *testing.T) {
	cancelErr := exitErr(t, 1)
	if !errors.Is(pickSubprocessErr(cancelErr, 1), domain.ErrPickCancelled) {
		t.Fatal("exit 1 should cancel")
	}
	failErr := exitErr(t, 5)
	if !errors.Is(pickSubprocessErr(failErr, 1), domain.ErrPickUnavailable) {
		t.Fatal("non-cancel exit should be unavailable")
	}
	if !errors.Is(pickSubprocessErr(errors.New("spawn failed"), 1), domain.ErrPickUnavailable) {
		t.Fatal("non-exit error should be unavailable")
	}
}

func exitErr(t *testing.T, code int) error {
	t.Helper()
	ctx := context.Background()
	var cmd *safexec.Cmd
	var err error
	if runtime.GOOS == "windows" {
		cmd, err = safexec.CommandContext(ctx, "cmd", "/c", "exit", strconv.Itoa(code))
	} else {
		cmd, err = safexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("exit %d", code))
	}
	if err != nil {
		t.Fatal(err)
	}
	_, err = cmd.Output()
	return err
}
