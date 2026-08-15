package harness

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// The built-in rules travel inside the binary, so a fresh install guards
// something and a malformed consumer file cannot empty the set.
//
//go:embed guards-default.yaml
var defaultGuardPlan []byte

// ConsumerGuardPlan is where a repository states its own rules, relative to the
// workspace root. Tracked in that repository on purpose: weakening a guard
// should be a diff someone reviews.
const ConsumerGuardPlan = ".ai-agents/guards.yaml"

// guardPlanKind is the document this loader accepts. Refusing anything else
// keeps a graph or a check plan from being read as a guard plan because the two
// happen to share a directory.
const (
	guardPlanKind    = "GuardPlan"
	guardPlanVersion = "vibe-agent/v1"
)

// builtinTestBlocks is the guard whose question cannot be asked one line at a
// time. A plan may retarget or disable it, but not restate it.
const builtinTestBlocks = "test-blocks"

type guardPlan struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Spec       planSpec `yaml:"spec"`
}

type planSpec struct {
	Vocabulary map[string]string      `yaml:"vocabulary"`
	Guards     map[string]guardConfig `yaml:"guards"`
}

type guardConfig struct {
	Subject     string        `yaml:"subject"`
	Builtin     string        `yaml:"builtin"`
	Disabled    bool          `yaml:"disabled"`
	FileMarker  bool          `yaml:"fileMarker"`
	AppliesTo   *selector     `yaml:"appliesTo"`
	ExemptFiles []string      `yaml:"exemptFiles"`
	SkipLines   *skipLineSpec `yaml:"skipLines"`
	Line        []patternRule `yaml:"lineChecks"`
	Density     []densityRule `yaml:"densityChecks"`
	// DisabledChecks removes individual rules by id. Granular enough to answer
	// a false positive without switching a whole guard off, and still a diff.
	DisabledChecks []string `yaml:"disabledChecks"`
}

type skipLineSpec struct {
	CommentPrefixes []string `yaml:"commentPrefixes"`
	Matching        string   `yaml:"matching"`
}

// defaultRules is the built-in set, compiled. It panics only if the embedded
// file is broken, which is a build error rather than a runtime condition: the
// file ships inside the binary and no consumer can reach it.
func defaultRules() []ruleSet {
	sets, err := buildRules(nil)
	if err != nil {
		panic("embedded guard plan is invalid: " + err.Error())
	}
	return sets
}

// activeRules is the built-in set with this workspace's plan applied.
func activeRules(req Request) ([]ruleSet, error) {
	overlay, err := readConsumerPlan(req.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	return buildRules(overlay)
}

// readConsumerPlan loads the workspace's plan, if it has one.
//
// An absent file is not an error and not a warning: most repositories want the
// defaults, and saying so on every edit would be noise. Any other read failure
// is reported, because a plan that exists and cannot be read is a guard set
// someone believes is running.
func readConsumerPlan(workspaceRoot string) (*guardPlan, error) {
	if workspaceRoot == "" {
		return nil, nil
	}
	path := filepath.Join(workspaceRoot, filepath.FromSlash(ConsumerGuardPlan))
	raw, err := os.ReadFile(filepath.Clean(path))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("%s cannot be read: %w", ConsumerGuardPlan, err)
	}

	plan, err := parseGuardPlan(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not a usable guard plan: %w", ConsumerGuardPlan, err)
	}
	return plan, nil
}

func parseGuardPlan(raw []byte) (*guardPlan, error) {
	var plan guardPlan
	if err := yaml.Unmarshal(raw, &plan); err != nil {
		return nil, err
	}
	if plan.Kind != guardPlanKind {
		return nil, fmt.Errorf("kind is %q, want %q", plan.Kind, guardPlanKind)
	}
	if plan.APIVersion != guardPlanVersion {
		return nil, fmt.Errorf("apiVersion is %q, want %q", plan.APIVersion, guardPlanVersion)
	}
	return &plan, nil
}

// buildRules merges the overlay onto the defaults and compiles the result.
func buildRules(overlay *guardPlan) ([]ruleSet, error) {
	base, err := parseGuardPlan(defaultGuardPlan)
	if err != nil {
		return nil, fmt.Errorf("built-in guard plan: %w", err)
	}

	vocabulary := map[string]string{}
	for name, fragment := range base.Spec.Vocabulary {
		vocabulary[name] = fragment
	}
	if overlay != nil {
		for name, fragment := range overlay.Spec.Vocabulary {
			vocabulary[name] = fragment
		}
	}

	merged := map[string]guardConfig{}
	for name, config := range base.Spec.Guards {
		merged[name] = config
	}
	if overlay != nil {
		for name, config := range overlay.Spec.Guards {
			merged[name] = mergeGuard(merged[name], config)
		}
	}

	// Sorted by name so two runs over one file report in the same order, and so
	// a diff of the output is about the file rather than about map iteration.
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	sets := make([]ruleSet, 0, len(names))
	for _, name := range names {
		set, err := compileGuard(name, merged[name], vocabulary)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, nil
}

// mergeGuard applies an overlay entry to a built-in one.
//
// Checks are appended rather than replaced, and removal goes through
// disabledChecks. A plan that could silently replace a rule list would let a
// one-line edit drop a credential check while still looking like a guard.
func mergeGuard(base, overlay guardConfig) guardConfig {
	result := base
	if overlay.Subject != "" {
		result.Subject = overlay.Subject
	}
	if overlay.Builtin != "" {
		result.Builtin = overlay.Builtin
	}
	if overlay.AppliesTo != nil {
		result.AppliesTo = overlay.AppliesTo
	}
	if overlay.SkipLines != nil {
		result.SkipLines = overlay.SkipLines
	}
	if overlay.Disabled {
		result.Disabled = true
	}
	if overlay.FileMarker {
		result.FileMarker = true
	}
	result.ExemptFiles = append(result.ExemptFiles, overlay.ExemptFiles...)
	result.Line = append(result.Line, overlay.Line...)
	result.Density = append(result.Density, overlay.Density...)
	result.DisabledChecks = append(result.DisabledChecks, overlay.DisabledChecks...)
	return result
}

// compileGuard turns one merged entry into a runnable rule set.
func compileGuard(name string, config guardConfig, vocabulary map[string]string) (ruleSet, error) {
	set := ruleSet{
		Name:        name,
		Disabled:    config.Disabled,
		Subject:     config.Subject,
		ExemptFiles: config.ExemptFiles,
		FileMarker:  config.FileMarker,
	}
	if set.Subject == "" {
		set.Subject = "finding(s)"
	}
	if config.AppliesTo != nil {
		set.AppliesTo = *config.AppliesTo
	}
	// A guard that names no file matches nothing and would look like a guard
	// with nothing to say, which is the failure this whole port exists to end.
	if !set.Disabled && set.AppliesTo.empty() {
		return ruleSet{}, fmt.Errorf("guard %q selects no files: give it appliesTo.types, .languages, or .extensions", name)
	}

	switch config.Builtin {
	case "":
	case builtinTestBlocks:
		set.inspect = inspectTestBlocks
	default:
		return ruleSet{}, fmt.Errorf("guard %q names builtin %q, which this build does not have", name, config.Builtin)
	}

	skip, err := compileSkipLines(name, config.SkipLines)
	if err != nil {
		return ruleSet{}, err
	}
	set.skipLine = skip

	dropped := map[string]struct{}{}
	for _, id := range config.DisabledChecks {
		dropped[id] = struct{}{}
	}

	for _, rule := range config.Line {
		if _, off := dropped[rule.ID]; off {
			continue
		}
		compiled, err := compilePattern(name, rule.ID, rule.Pattern, vocabulary)
		if err != nil {
			return ruleSet{}, err
		}
		rule.compiled = compiled
		if rule.NotPlaceholder != "" {
			rule.placeholder, err = compilePattern(name, rule.ID+".notPlaceholder", rule.NotPlaceholder, vocabulary)
			if err != nil {
				return ruleSet{}, err
			}
		}
		set.Line = append(set.Line, rule)
	}

	for _, rule := range config.Density {
		if _, off := dropped[rule.ID]; off {
			continue
		}
		compiled, err := compilePattern(name, rule.ID, rule.Pattern, vocabulary)
		if err != nil {
			return ruleSet{}, err
		}
		if rule.Threshold < 1 {
			return ruleSet{}, fmt.Errorf("guard %q check %q needs a threshold of at least 1", name, rule.ID)
		}
		rule.compiled = compiled
		set.Density = append(set.Density, rule)
	}

	if !set.Disabled && set.inspect == nil && len(set.Line) == 0 && len(set.Density) == 0 {
		return ruleSet{}, fmt.Errorf("guard %q has no checks left; disable it explicitly instead", name)
	}
	return set, nil
}

// vocabularyRef matches a {{name}} placeholder.
var vocabularyRef = regexp.MustCompile(`\{\{(\w+)\}\}`)

// compilePattern expands the vocabulary and compiles the result.
//
// The error names the guard, the check, and the unknown fragment, because a
// pattern is unreadable enough without a message that only says "invalid".
func compilePattern(guard, check, pattern string, vocabulary map[string]string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, fmt.Errorf("guard %q check %q has no pattern", guard, check)
	}

	var missing []string
	expanded := vocabularyRef.ReplaceAllStringFunc(pattern, func(ref string) string {
		key := ref[2 : len(ref)-2]
		fragment, ok := vocabulary[key]
		if !ok {
			missing = append(missing, key)
			return ref
		}
		return fragment
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("guard %q check %q refers to unknown vocabulary %s",
			guard, check, strings.Join(missing, ", "))
	}

	compiled, err := regexp.Compile(expanded)
	if err != nil {
		return nil, fmt.Errorf("guard %q check %q has an unusable pattern: %w "+
			"(RE2 has no lookahead or lookbehind; use `boundary` instead)", guard, check, err)
	}
	return compiled, nil
}

// compileSkipLines builds the "this line cannot be a finding" test.
func compileSkipLines(guard string, spec *skipLineSpec) (func(string) bool, error) {
	if spec == nil {
		return nil, nil
	}
	var matching *regexp.Regexp
	if spec.Matching != "" {
		var err error
		matching, err = regexp.Compile(spec.Matching)
		if err != nil {
			return nil, fmt.Errorf("guard %q has an unusable skipLines.matching: %w", guard, err)
		}
	}
	prefixes := spec.CommentPrefixes

	return func(line string) bool {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return true
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(trimmed, prefix) {
				return true
			}
		}
		return matching != nil && matching.MatchString(line)
	}, nil
}
