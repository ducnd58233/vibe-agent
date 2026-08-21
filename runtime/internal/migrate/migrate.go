// Package migrate moves legacy flat docs/ and workspace-root tmp/ trees into
// the current layout. The runtime itself does not read tmp/; this package is
// the only place that still knows that name, and only as a migration source.
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

// sourceTmp is the former workspace-root evidence directory. Migrate moves it
// into .agent-state/runs/; the runtime path helpers do not reference it.
const sourceTmp = "tmp"

// Plan is one directory move the migrator will perform.
type Plan struct {
	Slug       string
	Date       string
	Kind       string // "docs", "tmp", or "tmp-versioned"
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

// PlanWorkspace lists every tree that should move: flat docs/, flat tmp/, and
// versioned tmp/<date>/<slug>/<version>/ into .agent-state/runs/...
func PlanWorkspace(root string, now time.Time) ([]Plan, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var plans []Plan

	docsPlans, err := planFlatDocs(root, now)
	if err != nil {
		return nil, err
	}
	plans = append(plans, docsPlans...)

	tmpFlat, err := planFlatTmp(root, now)
	if err != nil {
		return nil, err
	}
	plans = append(plans, tmpFlat...)

	tmpVersioned, err := planVersionedTmp(root)
	if err != nil {
		return nil, err
	}
	plans = append(plans, tmpVersioned...)

	return plans, nil
}

func planFlatDocs(root string, now time.Time) ([]Plan, error) {
	base := filepath.Join(root, workspace.DocsDirName)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var plans []Plan
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if validate.Date(name) || !validate.Slug(name) {
			continue
		}
		from := filepath.Join(base, name)
		date, err := chooseDate(root, name, from, now)
		if err != nil {
			return nil, err
		}
		to := filepath.Join(base, date, name, "1")
		plan := Plan{Slug: name, Date: date, Kind: workspace.DocsDirName, From: from, To: to}
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
	return plans, nil
}

func planFlatTmp(root string, now time.Time) ([]Plan, error) {
	base := filepath.Join(root, sourceTmp)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var plans []Plan
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if validate.Date(name) || !validate.Slug(name) {
			continue
		}
		from := filepath.Join(base, name)
		date, err := chooseDate(root, name, from, now)
		if err != nil {
			return nil, err
		}
		to := workspace.RunDirAt(root, date, name, 1)
		plan := Plan{Slug: name, Date: date, Kind: sourceTmp, From: from, To: to}
		if _, err := os.Stat(to); err == nil {
			plan.SkipReason = "target already exists"
			plans = append(plans, plan)
			continue
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func planVersionedTmp(root string) ([]Plan, error) {
	base := filepath.Join(root, sourceTmp)
	dates, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var plans []Plan
	for _, dateEnt := range dates {
		if !dateEnt.IsDir() || !validate.Date(dateEnt.Name()) {
			continue
		}
		date := dateEnt.Name()
		slugs, err := os.ReadDir(filepath.Join(base, date))
		if err != nil {
			continue
		}
		for _, slugEnt := range slugs {
			if !slugEnt.IsDir() || !validate.Slug(slugEnt.Name()) {
				continue
			}
			slug := slugEnt.Name()
			versions, err := os.ReadDir(filepath.Join(base, date, slug))
			if err != nil {
				continue
			}
			for _, verEnt := range versions {
				if !verEnt.IsDir() {
					continue
				}
				var version int
				if _, scanErr := fmt.Sscanf(verEnt.Name(), "%d", &version); scanErr != nil || version < 1 {
					continue
				}
				from := filepath.Join(base, date, slug, verEnt.Name())
				to := workspace.RunDirAt(root, date, slug, version)
				plan := Plan{
					Slug: slug,
					Date: date,
					Kind: "tmp-versioned",
					From: from,
					To:   to,
				}
				if _, err := os.Stat(to); err == nil {
					plan.SkipReason = "target already exists"
				}
				plans = append(plans, plan)
			}
		}
	}
	return plans, nil
}

// Apply executes plans. Dry-run returns the same plans without writing.
// When the target already exists, leftover source trees under tmp/ are removed
// so doctor can pass after a partial migrate.
func Apply(root string, plans []Plan, opts Options) error {
	for _, plan := range plans {
		if plan.SkipReason != "" {
			if !opts.DryRun && strings.HasPrefix(filepath.ToSlash(plan.From), filepath.ToSlash(filepath.Join(root, sourceTmp))+"/") {
				_ = os.RemoveAll(plan.From)
			}
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
		if entry.Version > 1 || entry.Date > plan.Date {
			return nil
		}
	}
	version := 1
	if plan.Kind == "tmp-versioned" {
		base := filepath.Base(plan.To)
		if _, scanErr := fmt.Sscanf(base, "%d", &version); scanErr != nil || version < 1 {
			version = 1
		}
	}
	return runpath.SaveIndex(root, runpath.Entry{
		SchemaVersion: 1,
		Slug:          plan.Slug,
		Date:          plan.Date,
		Version:       version,
	})
}

func chooseDate(root, slug, dir string, now time.Time) (string, error) {
	manifest := filepath.Join(root, sourceTmp, slug, "manifest.json")
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
			out = append(out, Rename{From: name, To: "tasks-" + date + ".json"})
			continue
		}
		for _, stem := range markdownStems {
			if name == stem+".md" {
				out = append(out, Rename{From: name, To: stem + "-" + date + ".md"})
			}
		}
	}
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
