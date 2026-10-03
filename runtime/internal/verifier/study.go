package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Study reads a learner's study record and says whether study continues.
//
// Its check is true while any topic is not yet learned, which is the same
// polarity as tasks_remaining and for the same reason: "something is left" is
// the state a loop wants to hold, so every doubtful case has to land there. A
// missing record, an unreadable table, and an empty table all leave the check
// passed, so a mistake can only keep a learner studying, never end the study
// early. The study ends only when there is at least one topic and every topic
// says it is learned.
type Study struct{}

func (Study) Kind() string { return "study" }

// StudyRecordPath is where the study record lives for a run.
func StudyRecordPath(workspaceRoot string, run *state.Run) string {
	if run.Date == "" || run.Version < 1 {
		return ""
	}
	return filepath.Join(sharedworkspace.DocsDirAt(workspaceRoot, run.Date, run.Slug, run.Version),
		"STUDY-"+run.Date+".md")
}

func (Study) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, errors.New("study verifier needs a slug")
	}
	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	run, err := state.Load(manifest)
	if err != nil {
		return Result{}, fmt.Errorf("read run state: %w", err)
	}
	now := time.Now().UTC()

	path := StudyRecordPath(req.WorkspaceRoot, run)
	if path == "" {
		return studyContinues(relativeTo(req.WorkspaceRoot, manifest), "the run has no docs location yet", now), nil
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return studyContinues(relativeTo(req.WorkspaceRoot, path), "no study record yet", now), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relativeTo(req.WorkspaceRoot, path), err)
	}

	total, learned := countTopics(string(raw))
	ref := relativeTo(req.WorkspaceRoot, path)
	switch {
	case total == 0:
		return studyContinues(ref, "the topics table has no topics", now), nil
	case learned < total:
		return studyContinues(ref, fmt.Sprintf("%d of %d topics learned", learned, total), now), nil
	}
	// Every topic is learned: the study is complete, so "topics remaining" is
	// false. This is a recorded fail, and it is the exit from the loop.
	return Result{
		Check:   state.Check{Passed: false, Source: state.SourceFileAssert, Ref: ref, At: now},
		Summary: fmt.Sprintf("all %d topics learned", total),
	}, nil
}

func studyContinues(ref, why string, at time.Time) Result {
	return Result{
		Check:   state.Check{Passed: true, Source: state.SourceFileAssert, Ref: ref, At: at},
		Summary: why + "; study continues",
	}
}

// countTopics reads the markdown table whose header has a "Can do" column and a
// "Status" column, and counts its rows and how many say learned.
func countTopics(body string) (total, learned int) {
	statusCol := -1
	inTable := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if inTable {
				return total, learned
			}
			continue
		}
		cells := tableCells(trimmed)
		if !inTable {
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "can do") && strings.Contains(lower, "status") {
				for i, c := range cells {
					if strings.EqualFold(c, "status") {
						statusCol = i
					}
				}
				inTable = statusCol >= 0
			}
			continue
		}
		if isSeparatorRow(cells) || statusCol >= len(cells) {
			continue
		}
		total++
		if strings.EqualFold(cells[statusCol], "learned") {
			learned++
		}
	}
	return total, learned
}

func tableCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
