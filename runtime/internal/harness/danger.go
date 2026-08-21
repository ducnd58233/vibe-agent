package harness

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The danger list: actions that stop for a person whatever else has been
// approved.
//
// It sits beside the other verdicts in this package rather than in the guard
// plan, because the guard plan inspects file *content* after a tool ran and
// reports, while this classifies an *action* before it runs and refuses. Two
// jobs with different subjects, and modelling one as the other would mean
// matching file-content rules against a shell command.
//
// Auto mode is why this exists. Every other condition it checks can be produced
// by a machine: CI passed, tests passed, the linter is clean, ship returned GO.
// This is the set where no amount of green means yes.

//go:embed danger-default.yaml
var dangerDefaultPlan []byte

// ConsumerDangerPlan is where a repository adds its own categories, relative to
// the workspace root.
//
// Additive only. A consumer may extend the list and cannot shorten it: a
// workspace quietly removing the rule that stops production writes is the
// event the rule exists for. That is stricter than the guard plan, which allows
// disabling, and deliberately so - those guards warn, these refuse.
const ConsumerDangerPlan = ".ai-agents/danger.yaml"

const dangerPlanKind = "DangerPlan"

type dangerPlanFile struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		Categories []dangerCategory `yaml:"categories"`
	} `yaml:"spec"`
}

type dangerCategory struct {
	ID     string `yaml:"id"`
	Reason string `yaml:"reason"`
	// Commands match a shell segment. Paths match a file a tool is writing.
	Commands []string `yaml:"commands"`
	Paths    []string `yaml:"paths"`

	commands []*regexp.Regexp
	paths    []*regexp.Regexp
}

// builtInDanger parses the embedded plan once.
//
// This runs in PreToolUse, on the path of every shell command in a session, and
// the plan holds three dozen patterns. Compiling them per call is the
// difference between a guard nobody notices and a guard that taxes every call,
// which is the reason the branch lookup beside it is cached too.
var builtInDanger = sync.OnceValue(func() []dangerCategory {
	plan, err := parseDangerPlan(dangerDefaultPlan)
	if err != nil {
		return nil
	}
	return plan
})

// consumerDangerCache keeps one parsed plan per workspace, for the same reason.
var consumerDangerCache sync.Map

// DangerCategories lists the ids the built-in plan defines, sorted by
// declaration. A test walks it, so a category added without a test fails.
func DangerCategories() []string {
	plan := builtInDanger()
	ids := make([]string, 0, len(plan))
	for _, category := range plan {
		ids = append(ids, category.ID)
	}
	return ids
}

// parseDangerPlan reads a plan and compiles every pattern.
//
// Compiling at load rather than at match time means a broken pattern is a
// startup failure with a name attached, not a rule that silently never fires.
func parseDangerPlan(raw []byte) ([]dangerCategory, error) {
	var file dangerPlanFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse danger plan: %w", err)
	}
	if file.Kind != dangerPlanKind {
		return nil, fmt.Errorf("danger plan: kind %q, want %q", file.Kind, dangerPlanKind)
	}

	seen := map[string]bool{}
	out := make([]dangerCategory, 0, len(file.Spec.Categories))
	for _, category := range file.Spec.Categories {
		switch {
		case category.ID == "":
			return nil, fmt.Errorf("danger plan: a category has no id")
		case seen[category.ID]:
			return nil, fmt.Errorf("danger plan: category %q is declared twice", category.ID)
		case category.Reason == "":
			return nil, fmt.Errorf("danger plan: category %q has no reason; a refusal has to say why", category.ID)
		case len(category.Commands) == 0 && len(category.Paths) == 0:
			return nil, fmt.Errorf("danger plan: category %q matches nothing", category.ID)
		}
		seen[category.ID] = true

		for _, pattern := range category.Commands {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("danger plan: category %q command pattern %q: %w", category.ID, pattern, err)
			}
			category.commands = append(category.commands, compiled)
		}
		for _, pattern := range category.Paths {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("danger plan: category %q path pattern %q: %w", category.ID, pattern, err)
			}
			category.paths = append(category.paths, compiled)
		}
		out = append(out, category)
	}
	return out, nil
}

// dangerPlan is the built-in list plus whatever the workspace added.
//
// A consumer file that will not parse is ignored rather than fatal. The
// built-in list still applies, which is the safe direction to fail: the
// alternative is a typo in an optional file switching the whole gate off.
func dangerPlan(workspaceRoot string) []dangerCategory {
	plan := builtInDanger()
	if cached, ok := consumerDangerCache.Load(workspaceRoot); ok {
		if extra, isPlan := cached.([]dangerCategory); isPlan {
			return append(plan, extra...)
		}
	}

	var extra []dangerCategory
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(workspaceRoot, filepath.FromSlash(ConsumerDangerPlan))))
	if err == nil {
		if parsed, parseErr := parseDangerPlan(raw); parseErr == nil {
			extra = parsed
		}
	}
	consumerDangerCache.Store(workspaceRoot, extra)
	return append(plan, extra...)
}

// dangerVerdict refuses an action on the danger list.
//
// Shell segments are split the same way irreversibleAction splits them, so a
// migration hidden behind && is still seen. The split over-approximates, which
// for a guard means one extra fragment rather than one fewer.
func dangerVerdict(req Request, body payload) *BlockError {
	// The subject first, then the plan.
	//
	// Building the plan parses embedded YAML and compiles about three dozen
	// patterns. The sync.OnceValue around it makes that free within a process,
	// and every hook invocation is a new process, so the cache never gets a
	// second call to serve. A tool call with nothing to match against was
	// paying for the whole compilation and then matching it against nothing.
	segments := shellSegments(body.shellCommand())
	target := body.writeTarget()
	if len(segments) == 0 && target == "" {
		return nil
	}

	plan := dangerPlan(req.WorkspaceRoot)
	if len(plan) == 0 {
		return nil
	}

	for _, category := range plan {
		for _, pattern := range category.commands {
			for _, segment := range segments {
				if strings.TrimSpace(segment) != "" && pattern.MatchString(segment) {
					return &BlockError{Reason: dangerReason(category, "command", strings.TrimSpace(segment))}
				}
			}
		}
		if target == "" {
			continue
		}
		for _, pattern := range category.paths {
			if pattern.MatchString(filepath.ToSlash(target)) {
				return &BlockError{Reason: dangerReason(category, "path", target)}
			}
		}
	}
	return nil
}

func dangerReason(category dangerCategory, kind, subject string) string {
	return strings.Join([]string{
		fmt.Sprintf("Blocked: this is on the danger list (%s).", category.ID),
		strings.TrimSpace(category.Reason),
		fmt.Sprintf("Matched %s: %s", kind, subject),
		"A person decides this one. Nothing a check can produce makes it automatic,",
		"which is why auto mode stops here whatever else has passed.",
	}, "\n")
}
