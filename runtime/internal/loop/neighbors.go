package loop

import (
	"fmt"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Neighbor is one outgoing edge from the run's current node, with enough
// context for a host agent to see where the graph can go next and what evidence
// opens each path. The runtime still picks the edge on Advance; neighbors are
// advisory, not a second router.
type Neighbor struct {
	To               string
	Via              string
	Guard            string
	Negated          bool
	GuardDescription string
	GuardSource      string
	ToType           string
	ToDescription    string
	MatchesNow       bool
	ActivePath       bool
	Resets           []string
	EvidenceHint     string
}

// Neighbors lists outgoing edges from the run's current node in graph order.
func (r *Runner) Neighbors(run *state.Run) ([]Neighbor, error) {
	if run == nil {
		return nil, fmt.Errorf("neighbors need a run")
	}
	if run.CurrentNode == "" {
		return nil, nil
	}
	node, ok := r.Graph.Node(run.CurrentNode)
	if !ok {
		return nil, fmt.Errorf("run is at node %q, which the graph does not define", run.CurrentNode)
	}
	if node.Type == graph.NodeTerminal {
		return nil, nil
	}

	out := make([]Neighbor, 0, len(r.Graph.OutgoingEdges(run.CurrentNode)))
	activeMarked := false
	for _, edge := range r.Graph.OutgoingEdges(run.CurrentNode) {
		nb := Neighbor{To: edge.To, Via: edge.When, Resets: append([]string(nil), edge.Resets...)}
		if edge.When == "" {
			nb.GuardDescription = "fallback when no conditional edge matches"
			nb.EvidenceHint = "Record evidence for the current node, then call vibe_verify or vibe_checkpoint."
			nb.MatchesNow = true
		} else {
			name, negated := edge.Negated()
			nb.Guard = name
			nb.Negated = negated
			value, err := r.evaluate(run, Outcome{}, name)
			if err != nil {
				return nil, err
			}
			nb.MatchesNow = value != negated
			if guard, ok := r.Graph.Guard(name); ok {
				nb.GuardDescription = guard.Description
				nb.GuardSource = string(guard.Source)
				nb.EvidenceHint = evidenceHint(guard, negated)
			}
		}
		if dest, ok := r.Graph.Node(edge.To); ok {
			nb.ToType = string(dest.Type)
			nb.ToDescription = dest.Description
		}
		if nb.MatchesNow && !activeMarked {
			nb.ActivePath = true
			activeMarked = true
		}
		out = append(out, nb)
	}
	return out, nil
}

func evidenceHint(guard graph.Guard, negated bool) string {
	key := guard.Key()
	switch guard.Source {
	case graph.GuardCheck:
		if negated {
			return fmt.Sprintf("Leave check %q unmet or record it failed via vibe_verify.", key)
		}
		return fmt.Sprintf("Record check %q as passed via vibe_verify with real evidence.", key)
	case graph.GuardFlag:
		if negated {
			return fmt.Sprintf("Clear flag %q with vibe-agent run flag --clear %s at a human gate.", key, key)
		}
		return fmt.Sprintf("Set flag %q with vibe-agent run flag --set %s at a human gate.", key, key)
	case graph.GuardResult:
		return fmt.Sprintf("Record a result guard %q through vibe_checkpoint when the node completes.", guard.Name)
	case graph.GuardRuntime:
		return fmt.Sprintf("Runtime guard %q; usually not set by the host agent.", guard.Name)
	default:
		return ""
	}
}

// NeighborSummary is one line for CLI output.
func NeighborSummary(nb Neighbor) string {
	mark := " "
	if nb.ActivePath {
		mark = "*"
	}
	via := "fallback"
	if nb.Via != "" {
		via = nb.Via
	}
	desc := strings.TrimSpace(nb.ToDescription)
	if desc == "" {
		desc = nb.ToType
	}
	return fmt.Sprintf("%s %s via %s — %s", mark, nb.To, via, desc)
}
