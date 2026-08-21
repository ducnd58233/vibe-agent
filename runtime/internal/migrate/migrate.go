// Package migrate moves legacy flat docs/tmp trees into the versioned layout.
package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Plan is one directory move the migrator will perform.
type Plan struct {
	Slug       string
	Date       string
	Kind       string // "docs" or "tmp"
	From       string
	To         string
	Renames    []Rename
	SkipReason string
}

// Rename is one basename change under a moved directory.
type Rename struct {
	From string
	To   string
}

// Options controls a migrate pass.
type Options struct {
	DryRun bool
	Now    time.Time
}

var markdownStems = []string{
	"SPEC", "PLAN", "TASKS", "RESEARCH", "INVESTIGATION", "RECORD", "ADR",
}

// PlanWorkspace lists every flat docs/tmp slug that should move.
func PlanWorkspace(root string, now time.Time) ([]Plan, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var plans []Plan
	for _, kind := range []string{workspace.DocsDirName, workspace.RunsDirName} {
		base := filepath.Join(root, kind)
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if validate.Date(name) {
				continue // already versioned tree root
			}
			if !validate.Slug(name) {
				continue
			}
			from := filepath.Join(base, name)
			date, err := chooseDate(root, name, from, now)
			if err != nil {
				return nil, err
			}
			to := filepath.Join(base, date, name, "1")
			plan := Plan{
				Slug: name,
				Date: date,
				Kind: kind,
				From: from,
				To:   to,
			}
			if _, err := os.Stat(to); err == nil {
				plan.SkipReason = "target already exists"
				plans = append(plans, plan)
				continue
			}
			renames, err := plannedRenames(from, date)
			if err != nil {
				return nil, err
			}
			plan.Renames = renames
			plans = append(plans, plan)
		}
	}
	return plans, nil
}

// Apply executes plans. Dry-run returns the same plans without writing.
func Apply(root string, plans []Plan, opts Options) error {
	for _, plan := range plans {
		if plan.SkipReason != "" {
			continue
		}
		if opts.DryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(plan.To), 0o750); err != nil {
			return fmt.Errorf("mkdir parent for %s: %w", plan.To, err)
		}
		if err := os.Rename(plan.From, plan.To); err != nil {
			return fmt.Errorf("move %s -> %s: %w", plan.From, plan.To, err)
		}
		for _, rename := range plan.Renames {
			from := filepath.Join(plan.To, rename.From)
			to := filepath.Join(plan.To, rename.To)
			if err := os.Rename(from, to); err != nil {
				return fmt.Errorf("rename %s -> %s: %w", from, to, err)
			}
		}
		if err := ensureIndex(root, plan); err != nil {
			return err
		}
	}
	return nil
}

func ensureIndex(root string, plan Plan) error {
	entry, err := runpath.LoadIndex(root, plan.Slug)
	if err == nil && entry.Version >= 1 {
		// Keep the higher version if something newer already points elsewhere.
		if entry.Version > 1 || entry.Date > plan.Date {
			return nil
		}
	}
	return runpath.SaveIndex(root, runpath.Entry{
		SchemaVersion: 1,
		Slug:          plan.Slug,
		Date:          plan.Date,
		Version:       1,
	})
}

func chooseDate(root, slug, dir string, now time.Time) (string, error) {
	manifest := filepath.Join(root, workspace.RunsDirName, slug, "manifest.json")
	if raw, err := os.ReadFile(filepath.Clean(manifest)); err == nil {
		var body struct {
			CreatedAt time.Time `json:"createdAt"`
			Date      string    `json:"date"`
		}
		if json.Unmarshal(raw, &body) == nil {
			if validate.Date(body.Date) {
				return body.Date, nil
			}
			if !body.CreatedAt.IsZero() {
				return body.CreatedAt.UTC().Format("2006-01-02"), nil
			}
		}
	}
	info, err := os.Stat(dir)
	if err == nil {
		return info.ModTime().UTC().Format("2006-01-02"), nil
	}
	return now.UTC().Format("2006-01-02"), nil
}

func plannedRenames(dir, date string) ([]Rename, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Rename
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "-"+date+".md") || strings.HasSuffix(name, "-"+date+".json") {
			continue
		}
		switch name {
		case "tasks.json":
			to := "tasks-" + date + ".json"
			out = append(out, Rename{From: name, To: to})
			seen[to] = true
			continue
		}
		for _, stem := range markdownStems {
			if name == stem+".md" {
				to := stem + "-" + date + ".md"
				out = append(out, Rename{From: name, To: to})
				seen[to] = true
			}
		}
	}
	_ = seen
	return out, nil
}

// FormatPlan renders a human-readable dry-run listing.
func FormatPlan(plans []Plan) string {
	var b strings.Builder
	if len(plans) == 0 {
		return "migrate: nothing to do\n"
	}
	for _, plan := range plans {
		if plan.SkipReason != "" {
			fmt.Fprintf(&b, "skip  %s %s (%s)\n", plan.Kind, plan.Slug, plan.SkipReason)
			continue
		}
		fmt.Fprintf(&b, "move  %s -> %s\n", filepath.ToSlash(plan.From), filepath.ToSlash(plan.To))
		for _, rename := range plan.Renames {
			fmt.Fprintf(&b, "  rename %s -> %s\n", rename.From, rename.To)
		}
	}
	return b.String()
}
