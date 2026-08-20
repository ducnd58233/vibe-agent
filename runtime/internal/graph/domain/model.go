// Package graph loads and validates workflow graphs.
//
// A graph is the control flow for a multi-phase workflow: which node runs next,
// and on what evidence. The shapes match schemas/workflow-graph.schema.json.
//
// Edge conditions are guard names, never expressions. That keeps the loader a
// parser rather than an interpreter, and lets a typo be a static error.
package domain

import "fmt"

// APIVersion and Kind are the only values this loader accepts.
const (
	APIVersion = "vibe-agent/v1"
	Kind       = "WorkflowGraph"
)

// NodeType is what performs the work at a node.
type NodeType string

const (
	// NodeAgent is free-form work by the host coding agent. It never produces
	// an automatic pass.
	NodeAgent NodeType = "agent"
	// NodeArtifact is agent work followed by a file assertion.
	NodeArtifact NodeType = "artifact"
	// NodeVerifier is a runtime subprocess or file check that writes evidence.
	NodeVerifier NodeType = "verifier"
	// NodeHumanGate is an approval. It precedes irreversible actions.
	NodeHumanGate NodeType = "human_gate"
	// NodeTerminal ends the run.
	NodeTerminal NodeType = "terminal"
)

func (t NodeType) valid() bool {
	switch t {
	case NodeAgent, NodeArtifact, NodeVerifier, NodeHumanGate, NodeTerminal:
		return true
	}
	return false
}

// VerifierKind selects the implementation that produces evidence.
type VerifierKind string

const (
	VerifierCommand VerifierKind = "command"
	VerifierFiles   VerifierKind = "files"
	VerifierGit     VerifierKind = "git"
	// VerifierScreen proves an app rendered: no crash, the expected content in
	// the view hierarchy, and a frame that is not blank. An exit code cannot
	// answer any of those.
	VerifierScreen VerifierKind = "screen"
	// VerifierTasks reads the machine-readable task list a plan produced and
	// reports whether any task is still in scope.
	VerifierTasks VerifierKind = "tasks"
)

func (k VerifierKind) valid() bool {
	switch k {
	case VerifierCommand, VerifierFiles, VerifierGit, VerifierScreen, VerifierTasks:
		return true
	}
	return false
}

// GuardSource says where the runtime reads a guard's boolean.
type GuardSource string

const (
	// GuardFlag reads run state flags, set at intake or from the spec.
	GuardFlag GuardSource = "flag"
	// GuardCheck reads checks[key].passed, written only by real evidence.
	GuardCheck GuardSource = "check"
	// GuardResult reads the outcome recorded for the node that just ran.
	GuardResult GuardSource = "result"
	// GuardRuntime is maintained by the runner itself: budgets and repeated
	// blockers. No node produces it.
	GuardRuntime GuardSource = "runtime"
)

func (s GuardSource) valid() bool {
	switch s {
	case GuardFlag, GuardCheck, GuardResult, GuardRuntime:
		return true
	}
	return false
}

// TerminalStatus is how a run ends.
type TerminalStatus string

const (
	TerminalDone      TerminalStatus = "done"
	TerminalFailed    TerminalStatus = "failed"
	TerminalCancelled TerminalStatus = "cancelled"
)

func (s TerminalStatus) valid() bool {
	switch s {
	case TerminalDone, TerminalFailed, TerminalCancelled:
		return true
	}
	return false
}

// Guard is a named boolean an edge can branch on.
type Guard struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Source      GuardSource `yaml:"source"`
	// Reads is the key the guard resolves against. A guard is a question
	// (unit_passed); a check is an evidence slot (unit). When they differ, the
	// mapping is stated here rather than left to runtime convention.
	Reads string `yaml:"reads"`
	// AcceptsSkipped lets a skipped check satisfy this guard.
	//
	// Off by default, and that default is the point. A skipped check is not a
	// passed check, so treating the two alike opens a gate on the strength of
	// something not having run. The runner used to do this for every
	// check-sourced guard, which meant any check that got skipped satisfied its
	// own gate. Where a graph genuinely wants it, it says so here and the reason
	// belongs in the description.
	AcceptsSkipped bool `yaml:"acceptsSkipped"`
}

// Key is the state key this guard resolves against.
func (g Guard) Key() string {
	if g.Reads != "" {
		return g.Reads
	}
	return g.Name
}

// Node is one step in the workflow.
type Node struct {
	Type           NodeType `yaml:"type"`
	Description    string   `yaml:"description"`
	TimeoutSeconds int      `yaml:"timeoutSeconds"`

	// agent and artifact
	Command  string   `yaml:"command"`
	Optional bool     `yaml:"optional"`
	Outputs  []string `yaml:"outputs"`

	// verifier
	Verifier VerifierKind `yaml:"verifier"`
	SkipWhen string       `yaml:"skipWhen"`

	// verifier and human_gate
	Check string `yaml:"check"`

	// human_gate
	Prompt string `yaml:"prompt"`
	Guards string `yaml:"guards"`

	// terminal
	Status TerminalStatus `yaml:"status"`
}

// SkipCondition returns the guard name in SkipWhen and whether it is negated,
// mirroring Edge.Negated. Both conditions use the same syntax, so both need the
// same accessor rather than callers reimplementing the "!" prefix.
func (n Node) SkipCondition() (string, bool) {
	return splitCondition(n.SkipWhen)
}

// Edge is a transition. When is a guard name, optionally negated with "!".
// An edge with no When is the fallback and at most one may leave a node.
type Edge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	When string `yaml:"when"`
}

// Negated reports whether the condition is inverted, and returns the bare name.
func (e Edge) Negated() (string, bool) {
	return splitCondition(e.When)
}

// Metadata identifies the graph.
type Metadata struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
}

// Spec is the workflow itself.
type Spec struct {
	Initial        string          `yaml:"initial"`
	MaxTransitions int             `yaml:"maxTransitions"`
	Guards         []Guard         `yaml:"guards"`
	Nodes          map[string]Node `yaml:"nodes"`
	Edges          []Edge          `yaml:"edges"`
}

// Graph is a loaded workflow graph.
type Graph struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`

	// guardsByName is built by Index once the spec is decoded. It stays
	// unexported so nothing outside can populate it with something the spec
	// does not contain.
	guardsByName map[string]Guard
}

// Guard returns a declared guard by name.
func (g *Graph) Guard(name string) (Guard, bool) {
	guard, ok := g.guardsByName[name]
	return guard, ok
}

// Node returns a node by id.
func (g *Graph) Node(id string) (Node, bool) {
	node, ok := g.Spec.Nodes[id]
	return node, ok
}

// OutgoingEdges returns the edges leaving a node, conditional ones first so the
// fallback is only reached when nothing matched.
func (g *Graph) OutgoingEdges(from string) []Edge {
	var conditional, fallback []Edge
	for _, edge := range g.Spec.Edges {
		if edge.From != from {
			continue
		}
		if edge.When == "" {
			fallback = append(fallback, edge)
		} else {
			conditional = append(conditional, edge)
		}
	}
	return append(conditional, fallback...)
}

// TerminalStatusOf returns the end status of a terminal node.
func (g *Graph) TerminalStatusOf(id string) (TerminalStatus, error) {
	node, ok := g.Spec.Nodes[id]
	if !ok {
		return "", fmt.Errorf("no node %q", id)
	}
	if node.Type != NodeTerminal {
		return "", fmt.Errorf("node %q is %s, not terminal", id, node.Type)
	}
	return node.Status, nil
}

func splitCondition(condition string) (string, bool) {
	if condition == "" {
		return "", false
	}
	if condition[0] == '!' {
		return condition[1:], true
	}
	return condition, false
}

// Index builds the lookups a decoded graph needs.
//
// The decoder fills the spec and nothing else, so a graph that has not been
// indexed answers no guard. This is a method rather than an exported field
// because the index has to agree with the spec, and the only way to guarantee
// that is to derive it here.
func (g *Graph) Index() {
	g.guardsByName = make(map[string]Guard, len(g.Spec.Guards))
	for _, guard := range g.Spec.Guards {
		g.guardsByName[guard.Name] = guard
	}
}
