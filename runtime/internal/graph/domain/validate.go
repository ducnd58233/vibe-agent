package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	conditionPattern  = regexp.MustCompile(`^!?[a-z][a-z0-9_]*$`)
)

// ProblemsError collects every validation failure so one run reports all of them
// rather than making the author fix and rerun one at a time.
type ProblemsError []string

func (p ProblemsError) Error() string {
	return strings.Join(p, "\n")
}

func (p ProblemsError) sorted() ProblemsError {
	out := append(ProblemsError(nil), p...)
	sort.Strings(out)
	return out
}

// Validate applies every static rule the schema cannot express, plus the shape
// rules, so the loader works without a JSON Schema validator present.
//
// It mirrors scripts/check-graphs.py deliberately: the shell check runs on a
// fresh clone, this one runs wherever the binary does, and they must agree.
func (g *Graph) Validate() error {
	var problems ProblemsError

	if g.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion %q is not %q", g.APIVersion, APIVersion))
	}
	if g.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind %q is not %q", g.Kind, Kind))
	}
	if !validate.AssetID(g.Metadata.ID) {
		problems = append(problems, fmt.Sprintf("metadata.id %q must be lowercase and hyphenated", g.Metadata.ID))
	}
	if g.Metadata.Description == "" {
		problems = append(problems, "metadata.description must not be empty")
	}
	if g.Spec.MaxTransitions < 1 {
		problems = append(problems, fmt.Sprintf("spec.maxTransitions must be at least 1, got %d", g.Spec.MaxTransitions))
	}
	if len(g.Spec.Nodes) < 2 {
		problems = append(problems, "a graph needs at least two nodes")
	}
	if len(g.Spec.Edges) == 0 {
		problems = append(problems, "a graph needs at least one edge")
	}

	problems = append(problems, g.validateGuards()...)
	problems = append(problems, g.validateNodes()...)
	problems = append(problems, g.validateEdges()...)
	problems = append(problems, g.validateReachability()...)
	problems = append(problems, g.validateChecks()...)

	if len(problems) > 0 {
		return problems.sorted()
	}
	return nil
}

func (g *Graph) validateGuards() ProblemsError {
	var problems ProblemsError
	seen := map[string]bool{}
	for _, guard := range g.Spec.Guards {
		if !identifierPattern.MatchString(guard.Name) {
			problems = append(problems, fmt.Sprintf("guard %q must be lowercase with underscores", guard.Name))
			continue
		}
		if seen[guard.Name] {
			problems = append(problems, fmt.Sprintf("guard %q is declared twice", guard.Name))
		}
		seen[guard.Name] = true
		if guard.Description == "" {
			problems = append(problems, fmt.Sprintf("guard %q needs a description", guard.Name))
		}
		if guard.Source != "" && !guard.Source.valid() {
			problems = append(problems, fmt.Sprintf("guard %q has unknown source %q", guard.Name, guard.Source))
		}
		if guard.Reads != "" && !identifierPattern.MatchString(guard.Reads) {
			problems = append(problems, fmt.Sprintf("guard %q reads %q, which is not a valid key", guard.Name, guard.Reads))
		}
		// Only checks can be skipped, so the setting means nothing anywhere else.
		// Silently ignoring it would let a graph read as though it had granted
		// something it did not.
		if guard.AcceptsSkipped && guard.Source != GuardCheck {
			problems = append(problems, fmt.Sprintf(
				"guard %q sets acceptsSkipped but reads from %s; only a check can be skipped",
				guard.Name, orFlag(guard.Source)))
		}
	}
	return problems
}

// orFlag names the effective source, since an empty source means flag.
func orFlag(source GuardSource) GuardSource {
	if source == "" {
		return GuardFlag
	}
	return source
}

func (g *Graph) validateNodes() ProblemsError {
	var problems ProblemsError
	for id, node := range g.Spec.Nodes {
		if !identifierPattern.MatchString(id) {
			problems = append(problems, fmt.Sprintf("node %q must be lowercase with underscores", id))
		}
		if !node.Type.valid() {
			problems = append(problems, fmt.Sprintf("node %q has unknown type %q", id, node.Type))
			continue
		}

		switch node.Type {
		case NodeAgent:
			if node.Command == "" {
				problems = append(problems, fmt.Sprintf("agent node %q needs a command", id))
			}
		case NodeArtifact:
			if node.Command == "" {
				problems = append(problems, fmt.Sprintf("artifact node %q needs a command", id))
			}
			if len(node.Outputs) == 0 {
				problems = append(problems, fmt.Sprintf("artifact node %q needs at least one output", id))
			}
		case NodeVerifier:
			if !node.Verifier.valid() {
				problems = append(problems, fmt.Sprintf("verifier node %q has unknown verifier %q", id, node.Verifier))
			}
			if !identifierPattern.MatchString(node.Check) {
				problems = append(problems, fmt.Sprintf("verifier node %q needs a check key", id))
			}
		case NodeHumanGate:
			if !identifierPattern.MatchString(node.Check) {
				problems = append(problems, fmt.Sprintf("human gate %q needs a check key", id))
			}
			if node.Prompt == "" {
				problems = append(problems, fmt.Sprintf("human gate %q needs a prompt the human actually reads", id))
			}
		case NodeTerminal:
			if !node.Status.valid() {
				problems = append(problems, fmt.Sprintf("terminal node %q has unknown status %q", id, node.Status))
			}
		}

		// skipWhen used to be checked for shape inside the verifier case only,
		// which meant a human gate could carry a malformed condition and the
		// only complaint would come later, from the guard-declared pass, or not
		// at all. Both node types that may skip are checked here, in one place.
		if node.SkipWhen != "" {
			switch node.Type {
			case NodeVerifier, NodeHumanGate:
				if !conditionPattern.MatchString(node.SkipWhen) {
					problems = append(problems, fmt.Sprintf("node %q has skipWhen %q, which is not a guard name", id, node.SkipWhen))
				}
				// A negated condition on a gate skips it by default. Guards
				// resolve from run state, and a flag nobody set reads as false,
				// so "!something" is true in a fresh run and the gate is gone
				// before anything had a chance to set anything. Positive only,
				// on the node type where getting this wrong means an
				// irreversible step ran with no one asked.
				if _, negated := node.SkipCondition(); negated && node.Type == NodeHumanGate {
					problems = append(problems, fmt.Sprintf(
						"human gate %q has skipWhen %q; a negated condition holds in a fresh run, so the gate would be skipped by default. Name a flag that has to be set instead",
						id, node.SkipWhen))
				}
			default:
				problems = append(problems, fmt.Sprintf("node %q is a %s and may not declare skipWhen; only verifiers and human gates skip", id, node.Type))
			}
		}
	}
	return problems
}

func (g *Graph) validateEdges() ProblemsError {
	var problems ProblemsError
	declared := map[string]bool{}
	for _, guard := range g.Spec.Guards {
		declared[guard.Name] = true
	}

	used := map[string]bool{}
	fallbacks := map[string]int{}

	for _, edge := range g.Spec.Edges {
		if _, ok := g.Spec.Nodes[edge.From]; !ok {
			problems = append(problems, fmt.Sprintf("edge from unknown node %q", edge.From))
		}
		if _, ok := g.Spec.Nodes[edge.To]; !ok {
			problems = append(problems, fmt.Sprintf("edge to unknown node %q", edge.To))
		}
		if edge.When == "" {
			fallbacks[edge.From]++
			continue
		}
		if !conditionPattern.MatchString(edge.When) {
			problems = append(problems, fmt.Sprintf("edge %s -> %s has condition %q; conditions are guard names, optionally negated, never expressions", edge.From, edge.To, edge.When))
			continue
		}
		name, _ := edge.Negated()
		used[name] = true
		if !declared[name] {
			problems = append(problems, fmt.Sprintf("edge %s -> %s uses undeclared guard %q", edge.From, edge.To, name))
		}
	}

	// A reset names evidence to drop. Naming one nothing produces is a typo
	// that would read as working: the transition would clear a key nobody
	// writes, and the check it meant to clear would survive.
	written := checksNodesWrite(g)
	for _, edge := range g.Spec.Edges {
		for _, name := range edge.Resets {
			if !written[name] {
				problems = append(problems, fmt.Sprintf(
					"edge %s -> %s resets %q, which no node writes", edge.From, edge.To, name))
			}
		}
	}

	for _, node := range g.Spec.Nodes {
		if node.SkipWhen == "" {
			continue
		}
		name, _ := splitCondition(node.SkipWhen)
		used[name] = true
		if !declared[name] {
			problems = append(problems, fmt.Sprintf("skipWhen uses undeclared guard %q", name))
		}
	}

	for _, guard := range g.Spec.Guards {
		if !used[guard.Name] {
			problems = append(problems, fmt.Sprintf("guard %q is declared but never used", guard.Name))
		}
	}

	for id, count := range fallbacks {
		if count > 1 {
			problems = append(problems, fmt.Sprintf("node %q has %d unconditional edges; at most one may be the fallback", id, count))
		}
	}

	// A node whose edges are all conditional strands the run when nothing
	// matches. The safe shape is a guard paired with its negation.
	for id, node := range g.Spec.Nodes {
		if node.Type == NodeTerminal || fallbacks[id] > 0 {
			continue
		}
		positive, negative := map[string]bool{}, map[string]bool{}
		outgoing := 0
		for _, edge := range g.Spec.Edges {
			if edge.From != id || edge.When == "" {
				continue
			}
			outgoing++
			name, negated := edge.Negated()
			if negated {
				negative[name] = true
			} else {
				positive[name] = true
			}
		}
		if outgoing == 0 {
			continue
		}
		paired := false
		for name := range positive {
			if negative[name] {
				paired = true
				break
			}
		}
		if !paired {
			problems = append(problems, fmt.Sprintf("node %q has only conditional edges and no guard paired with its negation, so a run can strand there", id))
		}
	}

	return problems
}

func (g *Graph) validateReachability() ProblemsError {
	var problems ProblemsError

	if _, ok := g.Spec.Nodes[g.Spec.Initial]; !ok {
		problems = append(problems, fmt.Sprintf("spec.initial %q is not a declared node", g.Spec.Initial))
		return problems
	}

	forward := map[string][]string{}
	backward := map[string][]string{}
	for _, edge := range g.Spec.Edges {
		forward[edge.From] = append(forward[edge.From], edge.To)
		backward[edge.To] = append(backward[edge.To], edge.From)
	}

	reachable := walk([]string{g.Spec.Initial}, forward)
	for _, id := range sortedKeys(g.Spec.Nodes) {
		if !reachable[id] {
			problems = append(problems, fmt.Sprintf("node %q is unreachable from %q", id, g.Spec.Initial))
		}
	}

	var terminals []string
	for id, node := range g.Spec.Nodes {
		if node.Type == NodeTerminal {
			terminals = append(terminals, id)
			if len(forward[id]) > 0 {
				problems = append(problems, fmt.Sprintf("terminal node %q has outgoing edges", id))
			}
		} else if len(forward[id]) == 0 {
			problems = append(problems, fmt.Sprintf("node %q is a dead end: not terminal and nothing leaves it", id))
		}
	}
	if len(terminals) == 0 {
		problems = append(problems, "no terminal node, so no run can end")
		return problems
	}

	canEnd := walk(terminals, backward)
	for _, id := range sortedKeys(g.Spec.Nodes) {
		if !canEnd[id] {
			problems = append(problems, fmt.Sprintf("node %q cannot reach any terminal, so a run there spins forever", id))
		}
	}
	return problems
}

// validateChecks holds the two invariants that keep evidence honest: no node
// silently overwrites another's check, and no guard reads a key nothing writes.
func (g *Graph) validateChecks() ProblemsError {
	var problems ProblemsError

	writers := map[string][]string{}
	for _, id := range sortedKeys(g.Spec.Nodes) {
		node := g.Spec.Nodes[id]
		if node.Check != "" {
			writers[node.Check] = append(writers[node.Check], id)
		}
	}
	for _, key := range sortedStringKeys(writers) {
		if len(writers[key]) > 1 {
			problems = append(problems, fmt.Sprintf("check %q is written by %s; the second would overwrite the first one's evidence", key, strings.Join(writers[key], " and ")))
		}
	}

	for _, guard := range g.Spec.Guards {
		if guard.Source != GuardCheck {
			continue
		}
		if _, ok := writers[guard.Key()]; !ok {
			problems = append(problems, fmt.Sprintf("guard %q reads check %q, which no node writes, so it is permanently false", guard.Name, guard.Key()))
		}
	}
	return problems
}

func walk(start []string, adjacency map[string][]string) map[string]bool {
	seen := map[string]bool{}
	queue := append([]string(nil), start...)
	for _, id := range start {
		seen[id] = true
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return seen
}

func sortedKeys(nodes map[string]Node) []string {
	keys := make([]string, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// checksNodesWrite is every check name some node in the graph produces.
func checksNodesWrite(g *Graph) map[string]bool {
	written := map[string]bool{}
	for _, node := range g.Spec.Nodes {
		if node.Check != "" {
			written[node.Check] = true
		}
	}
	return written
}
