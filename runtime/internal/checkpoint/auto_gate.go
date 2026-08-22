package checkpoint

import (
	"encoding/json"
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// maxAutoGateFollow is how many answer-and-advance cycles one checkpoint may
// chain. Literature then design is two gates; a small cap prevents a loop.
const maxAutoGateFollow = 4

// followAutoGates answers document gates on the auto path and walks past them
// without a person. Goal mode still parks at the same gates.
func followAutoGates(req Request, result *Result) (*Result, error) {
	if result == nil || result.Duplicate || result.Run == nil || result.Graph == nil {
		return result, nil
	}
	if !result.Run.Flags["auto"] {
		return result, nil
	}
	if result.Transition != nil && result.Transition.Terminal {
		return result, nil
	}

	runner := loop.New(result.Graph)
	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	logPath := state.EventLogPath(req.WorkspaceRoot, req.Slug)

	for range maxAutoGateFollow {
		node, ok := result.Graph.Node(result.Run.CurrentNode)
		if !ok || node.Type != graph.NodeHumanGate {
			break
		}
		if _, ok := auto.GateSpecFor(result.Run.CurrentNode); !ok {
			break
		}

		answer, err := auto.TryAnswerGate(req.WorkspaceRoot, result.Graph, result.Run)
		if err != nil {
			return result, err
		}
		if !answer.Answered {
			break
		}
		if err := state.Save(manifest, result.Run); err != nil {
			return result, err
		}

		if result.Run.Status != state.StatusRunning {
			break
		}

		from := result.Run.CurrentNode
		transition, err := runner.Advance(result.Run, loop.Outcome{})
		if err != nil {
			return result, err
		}
		payload, err := json.Marshal(transitionEvent{
			From: from, To: transition.To, Via: transition.Via,
			Key: "auto-gate-" + from, Skipped: transition.Skipped,
		})
		if err != nil {
			return result, fmt.Errorf("encode transition: %w", err)
		}
		if _, err := state.AppendRunEvent(logPath, state.Event{
			Type: state.EventTransition, Node: transition.To, Payload: payload, At: req.now(),
		}); err != nil {
			return result, err
		}
		if err := state.Save(manifest, result.Run); err != nil {
			return result, err
		}

		result = &Result{Run: result.Run, Graph: result.Graph, Transition: transition}
		if transition.Terminal {
			break
		}
		if result.Run.Status != state.StatusAwaitingHuman {
			break
		}
	}
	return result, nil
}
