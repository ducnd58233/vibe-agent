package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// Files asserts that named paths exist and hold something.
//
// Non-empty matters: an artifact node that produced a zero-byte SPEC.md has not
// produced a spec, and an existence-only check would pass it.
type Files struct{}

func (Files) Kind() string { return "files" }

func (f Files) Verify(_ context.Context, req Request) (Result, error) {
	if len(req.Paths) == 0 {
		return Result{}, errors.New("files verifier needs at least one path")
	}

	var missing, empty, present []string
	for _, path := range req.Paths {
		full := path
		if !filepath.IsAbs(full) {
			full = filepath.Join(req.WorkspaceRoot, path)
		}
		info, err := os.Stat(full)
		switch {
		case errors.Is(err, os.ErrNotExist):
			missing = append(missing, path)
		case err != nil:
			return Result{}, fmt.Errorf("stat %s: %w", path, err)
		case info.IsDir():
			missing = append(missing, path+" (is a directory)")
		case info.Size() == 0:
			empty = append(empty, path)
		default:
			present = append(present, path)
		}
	}

	passed := len(missing) == 0 && len(empty) == 0
	result := Result{
		Check: state.Check{
			Passed: passed,
			Source: state.SourceFileAssert,
			At:     time.Now().UTC(),
		},
	}

	var parts []string
	if len(present) > 0 {
		parts = append(parts, fmt.Sprintf("%d present", len(present)))
	}
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(empty) > 0 {
		parts = append(parts, "empty: "+strings.Join(empty, ", "))
	}
	result.Summary = strings.Join(parts, "; ")
	result.Detail = result.Summary
	return result, nil
}
