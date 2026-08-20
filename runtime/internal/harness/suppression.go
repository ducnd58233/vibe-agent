package harness

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The suppression gate: an edit may not make a check pass by switching the
// check off.
//
// It refuses, so it sits beside the other refusals rather than with the guards,
// which advise. What separates it from every other refusal here is the input.
// The danger list classifies one action. This compares a pair - the text going
// out against the text coming in - because "this file contains a nolint" is not
// the finding. Nearly every repository has suppressions it decided on
// deliberately, and a rule that fired on those would be off within a day.
//
// The finding is that an edit leaves more of them than it found.

//go:embed suppression-default.yaml
var suppressionDefaultPlan []byte

// ConsumerSuppressionPlan is where a repository adds its own shapes, relative
// to the workspace root. Additive only, for the reason the danger list is.
const ConsumerSuppressionPlan = ".ai-agents/suppression.yaml"

const suppressionPlanKind = "SuppressionPlan"

// SuppressionAllowMarker acknowledges one deliberate suppression on the line
// that carries it.
//
// A single greppable string rather than a config setting, so every exemption in
// a repository can be listed with one search and reviewed as a diff. That is
// the convention the credential gate and the content guards already use, and
// having three spellings of "I meant this" would be worse than having one.
const SuppressionAllowMarker = "vibe-agent: allow-suppression"

type suppressionPlanFile struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		Shapes     []suppressionShape     `yaml:"shapes"`
		Thresholds []suppressionThreshold `yaml:"thresholds"`
	} `yaml:"spec"`
}

type suppressionShape struct {
	ID       string   `yaml:"id"`
	Reason   string   `yaml:"reason"`
	Patterns []string `yaml:"patterns"`

	compiled []*regexp.Regexp
}

// suppressionThreshold is a number a check compares against, where the finding
// is that it moved rather than that it appeared.
type suppressionThreshold struct {
	ID     string `yaml:"id"`
	Reason string `yaml:"reason"`
	// Pattern must capture the number in group one.
	Pattern string `yaml:"pattern"`
	// Direction is which way is weaker: increase for a failure ceiling,
	// decrease for a floor.
	Direction string `yaml:"direction"`

	compiled *regexp.Regexp
}

type suppressionPlan struct {
	shapes     []suppressionShape
	thresholds []suppressionThreshold
}

func (p suppressionPlan) empty() bool { return len(p.shapes) == 0 && len(p.thresholds) == 0 }

// builtInSuppression parses the embedded plan once, for the reason the danger
// plan is cached: this runs on the path of every write in a session.
var builtInSuppression = sync.OnceValue(func() suppressionPlan {
	plan, err := parseSuppressionPlan(suppressionDefaultPlan)
	if err != nil {
		return suppressionPlan{}
	}
	return plan
})

var consumerSuppressionCache sync.Map

// SuppressionShapes lists the ids the built-in plan defines. A test walks it, so
// a shape added without a test fails.
func SuppressionShapes() []string {
	plan := builtInSuppression()
	ids := make([]string, 0, len(plan.shapes)+len(plan.thresholds))
	for _, shape := range plan.shapes {
		ids = append(ids, shape.ID)
	}
	for _, threshold := range plan.thresholds {
		ids = append(ids, threshold.ID)
	}
	return ids
}

const (
	directionIncrease = "increase"
	directionDecrease = "decrease"
)

// parseSuppressionPlan reads a plan and compiles every pattern, so a broken one
// is a startup failure with a name attached rather than a rule that never fires.
func parseSuppressionPlan(raw []byte) (suppressionPlan, error) {
	var file suppressionPlanFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return suppressionPlan{}, fmt.Errorf("parse suppression plan: %w", err)
	}
	if file.Kind != suppressionPlanKind {
		return suppressionPlan{}, fmt.Errorf("suppression plan: kind %q, want %q", file.Kind, suppressionPlanKind)
	}

	seen := map[string]bool{}
	plan := suppressionPlan{}

	for _, shape := range file.Spec.Shapes {
		if err := checkSuppressionID(seen, shape.ID, shape.Reason); err != nil {
			return suppressionPlan{}, err
		}
		if len(shape.Patterns) == 0 {
			return suppressionPlan{}, fmt.Errorf("suppression plan: shape %q matches nothing", shape.ID)
		}
		for _, pattern := range shape.Patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return suppressionPlan{}, fmt.Errorf("suppression plan: shape %q pattern %q: %w", shape.ID, pattern, err)
			}
			shape.compiled = append(shape.compiled, compiled)
		}
		plan.shapes = append(plan.shapes, shape)
	}

	for _, threshold := range file.Spec.Thresholds {
		if err := checkSuppressionID(seen, threshold.ID, threshold.Reason); err != nil {
			return suppressionPlan{}, err
		}
		if threshold.Direction != directionIncrease && threshold.Direction != directionDecrease {
			return suppressionPlan{}, fmt.Errorf(
				"suppression plan: threshold %q has direction %q; say which way is weaker, %s or %s",
				threshold.ID, threshold.Direction, directionIncrease, directionDecrease)
		}
		compiled, err := regexp.Compile(threshold.Pattern)
		if err != nil {
			return suppressionPlan{}, fmt.Errorf("suppression plan: threshold %q pattern %q: %w", threshold.ID, threshold.Pattern, err)
		}
		if compiled.NumSubexp() < 1 {
			return suppressionPlan{}, fmt.Errorf(
				"suppression plan: threshold %q captures no number; group one is the value being compared", threshold.ID)
		}
		threshold.compiled = compiled
		plan.thresholds = append(plan.thresholds, threshold)
	}
	return plan, nil
}

func checkSuppressionID(seen map[string]bool, id, reason string) error {
	switch {
	case id == "":
		return fmt.Errorf("suppression plan: an entry has no id")
	case seen[id]:
		return fmt.Errorf("suppression plan: %q is declared twice", id)
	case reason == "":
		return fmt.Errorf("suppression plan: %q has no reason; a refusal has to say why", id)
	}
	seen[id] = true
	return nil
}

// suppressionRules is the built-in plan plus whatever the workspace added. A
// consumer file that will not parse is ignored rather than fatal, so a typo in
// an optional file cannot switch the gate off.
func suppressionRules(workspaceRoot string) suppressionPlan {
	plan := builtInSuppression()
	if cached, ok := consumerSuppressionCache.Load(workspaceRoot); ok {
		if extra, isPlan := cached.(suppressionPlan); isPlan {
			return mergeSuppression(plan, extra)
		}
	}

	var extra suppressionPlan
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(workspaceRoot, filepath.FromSlash(ConsumerSuppressionPlan))))
	if err == nil {
		if parsed, parseErr := parseSuppressionPlan(raw); parseErr == nil {
			extra = parsed
		}
	}
	consumerSuppressionCache.Store(workspaceRoot, extra)
	return mergeSuppression(plan, extra)
}

func mergeSuppression(base, extra suppressionPlan) suppressionPlan {
	if extra.empty() {
		return base
	}
	return suppressionPlan{
		shapes:     append(append([]suppressionShape{}, base.shapes...), extra.shapes...),
		thresholds: append(append([]suppressionThreshold{}, base.thresholds...), extra.thresholds...),
	}
}

// countShape reports how many lines in text carry this shape, ignoring lines
// that acknowledge it.
func (s suppressionShape) count(text string) (int, string) {
	total, example := 0, ""
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, SuppressionAllowMarker) {
			continue
		}
		for _, pattern := range s.compiled {
			if pattern.MatchString(line) {
				total++
				if example == "" {
					example = strings.TrimSpace(line)
				}
				break
			}
		}
	}
	return total, example
}

// weakest returns the weakest value this threshold finds in text, and whether
// it found one at all.
//
// The weakest rather than the first: a file with two thresholds in it is
// answered by whichever one lets the most through, because that is the one a
// check will actually compare against.
func (t suppressionThreshold) weakest(text string) (int, bool) {
	found, weakest := false, 0
	for _, match := range t.compiled.FindAllStringSubmatch(text, -1) {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if !found || t.weaker(value, weakest) {
			found, weakest = true, value
		}
	}
	return weakest, found
}

func (t suppressionThreshold) weaker(candidate, than int) bool {
	if t.Direction == directionIncrease {
		return candidate > than
	}
	return candidate < than
}

// suppressionVerdict refuses an edit that leaves more suppressions than it
// found, or moves a threshold the weak way.
//
// Both halves need the text on the way out as well as the text going in. Edit
// sends both. Write sends only the new content, so the file it is replacing is
// read; a Write creating a new file has nothing to compare against and every
// suppression in it is one being added.
func suppressionVerdict(req Request, body payload) *BlockError {
	after, ok := suppressionSubject(body)
	if !ok {
		return nil
	}
	plan := suppressionRules(req.WorkspaceRoot)
	if plan.empty() || definesTheRule(req, body.writeTarget()) {
		return nil
	}
	before := suppressionBefore(req, body)

	for _, shape := range plan.shapes {
		added, example := shape.count(after)
		existing, _ := shape.count(before)
		if added > existing {
			return &BlockError{Reason: suppressionReason(shape.ID, shape.Reason,
				fmt.Sprintf("this edit leaves %d where it found %d", added, existing), example)}
		}
	}

	for _, threshold := range plan.thresholds {
		now, foundNow := threshold.weakest(after)
		was, foundBefore := threshold.weakest(before)
		if !foundNow || !foundBefore || !threshold.weaker(now, was) {
			continue
		}
		return &BlockError{Reason: suppressionReason(threshold.ID, threshold.Reason,
			fmt.Sprintf("this edit moves it from %d to %d", was, now), "")}
	}
	return nil
}

// selfDefining are the files where these shapes have to be writable, because
// they are where the shapes are defined and exercised.
//
// A rule whose own test cannot be extended is a rule that stops being extended.
// The plan file is not on the list and does not need to be: its patterns are
// written as regular expressions, so `//\s*nolint` carries a backslash where a
// suppression carries the word, and nothing in it matches itself. The test file
// spells the shapes out literally, which is the point of it.
//
// An exact list rather than a prefix or a directory: three named paths can be
// read and argued with, and a consumer cannot widen it by naming a file
// suppression-something.
var selfDefining = map[string]bool{
	"runtime/internal/harness/suppression.go":      true,
	"runtime/internal/harness/suppression_test.go": true,
}

// definesTheRule reports whether a write targets one of those files.
func definesTheRule(req Request, target string) bool {
	if target == "" {
		return false
	}
	relative := target
	if filepath.IsAbs(relative) && req.WorkspaceRoot != "" {
		rel, err := filepath.Rel(req.WorkspaceRoot, relative)
		if err != nil {
			return false
		}
		relative = rel
	}
	return selfDefining[filepath.ToSlash(relative)]
}

// suppressionSubject is the text an edit is about to write, and whether this
// tool call writes any.
func suppressionSubject(body payload) (string, bool) {
	switch {
	case body.ToolInput.NewString != "":
		return body.ToolInput.NewString, true
	case body.ToolInput.Content != "":
		return body.ToolInput.Content, true
	default:
		return "", false
	}
}

// suppressionBefore is the text being replaced.
//
// For an Edit that is the string the tool was given. For a Write it is the file
// on disk, read scoped to the workspace so a path built from a payload cannot
// reach outside it. A file that cannot be read is treated as empty, which is
// the strict direction: a new file's suppressions are all additions.
func suppressionBefore(req Request, body payload) string {
	if body.ToolInput.OldString != "" {
		return body.ToolInput.OldString
	}
	target := body.writeTarget()
	if target == "" || req.WorkspaceRoot == "" {
		return ""
	}
	relative := target
	if filepath.IsAbs(relative) {
		rel, err := filepath.Rel(req.WorkspaceRoot, relative)
		if err != nil {
			return ""
		}
		relative = rel
	}
	scoped, err := os.OpenRoot(req.WorkspaceRoot)
	if err != nil {
		return ""
	}
	defer func() { _ = scoped.Close() }()

	existing, err := scoped.ReadFile(relative)
	if err != nil {
		return ""
	}
	return string(existing)
}

func suppressionReason(id, why, delta, example string) string {
	lines := []string{
		fmt.Sprintf("Blocked: this edit adds a suppression (%s).", id),
		strings.TrimSpace(why),
		delta,
	}
	if example != "" {
		lines = append(lines, "First one: "+example)
	}
	return strings.Join(append(lines,
		"Fix the code the check is pointing at. If it cannot be fixed here, stop and report why",
		"rather than making the check stop asking.",
		"If this one is deliberate, say so on the line: "+SuppressionAllowMarker,
	), "\n")
}
