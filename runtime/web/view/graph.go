package view

import (
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// GraphNodeStatus is the display word for one workflow node.
type GraphNodeStatus string

const (
	GraphStatusPassed   GraphNodeStatus = "passed"
	GraphStatusAwaiting GraphNodeStatus = "awaiting"
	GraphStatusPending  GraphNodeStatus = "pending"
	GraphStatusSkipped  GraphNodeStatus = "skipped"
	GraphStatusFailed   GraphNodeStatus = "failed"
)

// GraphEdgeRow is one outgoing edge for the inspector Schema pane.
type GraphEdgeRow struct {
	To   string
	When string
}

// GraphNodeRow is one node in the Graph tab rail.
type GraphNodeRow struct {
	ID          string
	Type        string
	Description string
	Prompt      string
	Check       string
	Status      string
	Optional    bool
	Current     bool
	Outgoing    []GraphEdgeRow
	SearchText  string
}

// WalkOrder returns node ids reachable from Initial in BFS order, then any
// terminal nodes not reached (for example failed when the run ended early).
func WalkOrder(g *graph.Graph) []string {
	if g == nil || g.Spec.Initial == "" {
		return nil
	}
	seen := make(map[string]bool)
	order := make([]string, 0, len(g.Spec.Nodes))
	queue := []string{g.Spec.Initial}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		order = append(order, id)
		for _, edge := range g.OutgoingEdges(id) {
			if !seen[edge.To] {
				queue = append(queue, edge.To)
			}
		}
	}
	for id, node := range g.Spec.Nodes {
		if node.Type == graph.NodeTerminal && !seen[id] {
			order = append(order, id)
		}
	}
	return order
}

// GraphNeighborRow is one reachable next node from the current step.
type GraphNeighborRow struct {
	To               string
	Via              string
	GuardDescription string
	ToType           string
	ToDescription    string
	MatchesNow       bool
	ActivePath       bool
	EvidenceHint     string
}

// GraphStepView is the focused graph panel: current node plus outgoing neighbors.
type GraphStepView struct {
	Current   GraphNodeRow
	Neighbors []GraphNeighborRow
	HasStep   bool
}

// ProjectGraphStep builds the neighbor-focused graph panel for one run.
func ProjectGraphStep(g *graph.Graph, run *state.Run) GraphStepView {
	if g == nil || run == nil || run.CurrentNode == "" {
		return GraphStepView{}
	}
	node, ok := g.Node(run.CurrentNode)
	if !ok {
		return GraphStepView{}
	}
	cur := GraphNodeRow{
		ID:          run.CurrentNode,
		Type:        string(node.Type),
		Description: node.Description,
		Prompt:      node.Prompt,
		Check:       node.Check,
		Current:     true,
		Status:      string(GraphStatusAwaiting),
	}
	if run.Status == state.StatusFailed {
		cur.Status = string(GraphStatusFailed)
	}
	runner := loop.New(g)
	neighbors, err := runner.Neighbors(run)
	if err != nil {
		return GraphStepView{Current: cur, HasStep: true}
	}
	rows := make([]GraphNeighborRow, 0, len(neighbors))
	for _, nb := range neighbors {
		rows = append(rows, GraphNeighborRow{
			To:               nb.To,
			Via:              nb.Via,
			GuardDescription: nb.GuardDescription,
			ToType:           nb.ToType,
			ToDescription:    nb.ToDescription,
			MatchesNow:       nb.MatchesNow,
			ActivePath:       nb.ActivePath,
			EvidenceHint:     nb.EvidenceHint,
		})
	}
	return GraphStepView{Current: cur, Neighbors: rows, HasStep: true}
}

// ProjectGraph builds Graph tab rows from a loaded graph and run manifest.
func ProjectGraph(g *graph.Graph, run *state.Run) []GraphNodeRow {
	if g == nil {
		return nil
	}
	order := WalkOrder(g)
	cur := ""
	failedRun := false
	if run != nil {
		cur = run.CurrentNode
		failedRun = run.Status == state.StatusFailed
	}
	curIdx := indexOf(order, cur)
	rows := make([]GraphNodeRow, 0, len(order))
	for i, id := range order {
		node, ok := g.Node(id)
		if !ok {
			continue
		}
		status := nodeStatus(g, run, id, i, curIdx, failedRun)
		outgoing := make([]GraphEdgeRow, 0, len(g.OutgoingEdges(id)))
		for _, edge := range g.OutgoingEdges(id) {
			outgoing = append(outgoing, GraphEdgeRow{To: edge.To, When: edge.When})
		}
		row := GraphNodeRow{
			ID:          id,
			Type:        string(node.Type),
			Description: node.Description,
			Prompt:      node.Prompt,
			Check:       node.Check,
			Status:      string(status),
			Optional:    node.Optional,
			Current:     id == cur,
			Outgoing:    outgoing,
			SearchText: strings.ToLower(strings.Join([]string{
				id,
				string(node.Type),
				node.Description,
				string(status),
			}, " ")),
		}
		rows = append(rows, row)
	}
	return progressiveGraphRows(rows, order, cur)
}

// progressiveGraphRows limits the rail to nodes the run has already reached.
// Later steps stay out of the UI until the run arrives at them.
func progressiveGraphRows(rows []GraphNodeRow, order []string, current string) []GraphNodeRow {
	if len(rows) == 0 {
		return rows
	}
	if current == "" {
		return rows[:1]
	}
	curIdx := indexOf(order, current)
	if curIdx < 0 {
		return rows
	}
	if curIdx+1 >= len(rows) {
		return rows
	}
	return rows[:curIdx+1]
}

func nodeStatus(g *graph.Graph, run *state.Run, id string, idx, curIdx int, failedRun bool) GraphNodeStatus {
	if skippedNode(g, run, id) {
		return GraphStatusSkipped
	}
	if node, ok := g.Node(id); ok && node.Type == graph.NodeTerminal {
		if id == "failed" {
			if failedRun {
				return GraphStatusFailed
			}
			return GraphStatusPending
		}
		if run != nil && run.Status == state.StatusDone && id == "done" {
			return GraphStatusPassed
		}
		return GraphStatusPending
	}
	if curIdx < 0 {
		return GraphStatusPending
	}
	switch {
	case idx < curIdx:
		return GraphStatusPassed
	case idx == curIdx:
		if failedRun {
			return GraphStatusFailed
		}
		return GraphStatusAwaiting
	default:
		return GraphStatusPending
	}
}

func skippedNode(g *graph.Graph, run *state.Run, id string) bool {
	node, ok := g.Node(id)
	if !ok || node.SkipWhen == "" || run == nil {
		return false
	}
	name, negated := node.SkipCondition()
	value, ok := evalFlag(run, g, name)
	if !ok {
		return false
	}
	return value != negated
}

func evalFlag(run *state.Run, g *graph.Graph, name string) (bool, bool) {
	guard, ok := g.Guard(name)
	if !ok {
		return false, false
	}
	switch guard.Source {
	case graph.GuardFlag, "":
		return run.Flags[guard.Key()], true
	case graph.GuardCheck:
		check, recorded := run.Checks[guard.Key()]
		if !recorded {
			return false, true
		}
		if check.Skipped {
			return true, true
		}
		return check.Passed, true
	default:
		return false, false
	}
}

func indexOf(list []string, target string) int {
	for i, v := range list {
		if v == target {
			return i
		}
	}
	return -1
}

// GraphTypeOrder lists node types for the Graph filter menu.
var GraphTypeOrder = []graph.NodeType{
	graph.NodeHumanGate,
	graph.NodeAgent,
	graph.NodeArtifact,
	graph.NodeVerifier,
	graph.NodeTerminal,
}

// GraphTypeCounts tallies nodes by type for filter badges.
func GraphTypeCounts(rows []GraphNodeRow) map[string]int {
	counts := make(map[string]int)
	for _, row := range rows {
		counts[row.Type]++
	}
	return counts
}

// GraphTypeLabels returns stable type labels for filter menus.
func GraphTypeLabels() []string {
	out := make([]string, 0, len(GraphTypeOrder))
	for _, t := range GraphTypeOrder {
		out = append(out, string(t))
	}
	return out
}
