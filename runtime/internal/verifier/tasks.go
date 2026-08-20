package verifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

// Tasks answers whether a slug still has work planned.
//
// It replaces the one human verifier in the delivery plan that was human only
// because nothing had given it a machine-readable input. A person reading
// TASKS.md against the agreed scope is a judgement call; "does any task have a
// status other than done or canceled" is a fact about a file, which is what
// file_assert means.
//
// A missing task list is an error rather than "nothing remains". Treating an
// absent file as an empty one would end a run on a file somebody forgot to
// write, which is the expensive direction to be wrong in.
type Tasks struct{}

func (Tasks) Kind() string { return "tasks" }

func (Tasks) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, fmt.Errorf("tasks verifier needs a slug to find the task list")
	}

	path := tasks.Path(req.WorkspaceRoot, req.Slug)
	file, err := tasks.Load(req.WorkspaceRoot, req.Slug)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relativeTo(req.WorkspaceRoot, path), err)
	}

	remaining := file.Remaining()
	result := Result{
		Check: state.Check{
			Passed: len(remaining) > 0,
			Source: state.SourceFileAssert,
			Ref:    relativeTo(req.WorkspaceRoot, path),
			At:     time.Now().UTC(),
		},
	}

	if len(remaining) == 0 {
		result.Summary = fmt.Sprintf("no tasks remain of %d", len(file.Tasks))
		return result, nil
	}

	names := make([]string, 0, len(remaining))
	for _, task := range remaining {
		names = append(names, fmt.Sprintf("%s (%s)", task.ID, task.Status))
	}
	result.Summary = fmt.Sprintf("%d of %d tasks remain", len(remaining), len(file.Tasks))
	result.Detail = strings.Join(names, ", ")
	return result, nil
}
