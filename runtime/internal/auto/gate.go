package auto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// GateSpec names the document a skippable gate reads, the flag that says the
// document passed structural tests, and the check a person records instead.
type GateSpec struct {
	Flag  string
	Check string
	Files []string
}

// GateSpecs maps human_gate node ids to the artifacts auto mode can answer.
var GateSpecs = map[string]GateSpec{
	"approve_spec":          {Flag: "spec_unambiguous", Check: "spec_approved", Files: []string{"SPEC"}},
	"approve_plan":          {Flag: "plan_unambiguous", Check: "plan_approved", Files: []string{"PLAN", "TASKS"}},
	"approve_applicability": {Flag: "applicability_ok", Check: "applicability_approved", Files: []string{"RESEARCH"}},
	"approve_design":        {Flag: "design_ok", Check: "design_approved", Files: []string{"PLAN", "TASKS"}},
}

// GateNodeNames returns sorted gate node ids TryAnswerGate understands.
func GateNodeNames() []string {
	names := make([]string, 0, len(GateSpecs))
	for id := range GateSpecs {
		names = append(names, id)
	}
	sort.Strings(names)
	return names
}

// GateSpecFor returns the gate spec for a node, if any.
func GateSpecFor(nodeID string) (GateSpec, bool) {
	spec, ok := GateSpecs[nodeID]
	return spec, ok
}

// ResolveGateDoc picks the dated basename when the run has a date, else the
// undated legacy name, under the resolved docs directory.
func ResolveGateDoc(workspaceRoot, slug, stem, date string) string {
	dir := workspace.DocsDir(workspaceRoot, slug)
	if entry, err := runpath.Resolve(workspaceRoot, slug); err == nil {
		dir = workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
		if date == "" {
			date = entry.Date
		}
	}
	if date != "" {
		if name, err := workspace.DocsArtifact(stem, date); err == nil {
			dated := filepath.Join(dir, name)
			if _, err := os.Stat(dated); err == nil {
				return dated
			}
			undated := filepath.Join(dir, stem+".md")
			if _, err := os.Stat(undated); err == nil {
				return undated
			}
			return dated
		}
	}
	return filepath.Join(dir, stem+".md")
}

// ScanGateDocuments reads the artifacts behind a gate and reports open markers.
func ScanGateDocuments(workspaceRoot, slug, nodeID, date string) ([]Ambiguity, []string, error) {
	spec, ok := GateSpecs[nodeID]
	if !ok {
		return nil, nil, fmt.Errorf("node %q is not a document gate", nodeID)
	}

	var findings []Ambiguity
	var resolved []string
	for _, stem := range spec.Files {
		path := ResolveGateDoc(workspaceRoot, slug, stem, date)
		resolved = append(resolved, filepath.Base(path))
		document, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, resolved, err
		}
		findings = append(findings, Scan(string(document))...)
		switch stem {
		case "RESEARCH":
			findings = append(findings, RequireApplicability(string(document))...)
		case "PLAN":
			if nodeID == "approve_design" {
				findings = append(findings, RequireExperimentDiagram(string(document))...)
			}
		}
	}
	return findings, resolved, nil
}

// GateAnswer reports whether a gate was answered from documents.
type GateAnswer struct {
	Answered bool
	Findings []Ambiguity
	Flag     string
}

// TryAnswerGate scans the gate's documents and opens it when nothing is open.
//
// It only runs on the auto path, only at document gates, and only while the
// run is waiting at that gate. A missing artifact is not an error: the host
// may checkpoint before the file exists.
func TryAnswerGate(workspaceRoot string, g *graph.Graph, run *state.Run) (GateAnswer, error) {
	var empty GateAnswer
	if run == nil || !run.Flags["auto"] {
		return empty, nil
	}
	if run.Status != state.StatusAwaitingHuman {
		return empty, nil
	}
	spec, ok := GateSpecs[run.CurrentNode]
	if !ok {
		return empty, nil
	}

	findings, resolved, err := ScanGateDocuments(workspaceRoot, run.Slug, run.CurrentNode, run.Date)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return empty, err
	}
	if len(findings) > 0 {
		return GateAnswer{Findings: findings}, nil
	}

	now := time.Now().UTC()
	if err := run.SetFlagAt(spec.Flag, true, now); err != nil {
		return empty, err
	}
	payload, err := json.Marshal(map[string]any{
		"flag":  spec.Flag,
		"value": true,
		"note":  "auto gate found nothing open in " + strings.Join(resolved, " and "),
	})
	if err != nil {
		return empty, fmt.Errorf("encode flag event: %w", err)
	}
	if _, err := state.AppendRunEvent(state.EventLogPath(workspaceRoot, run.Slug),
		state.Event{Type: state.EventFlagSet, Node: run.CurrentNode, At: run.UpdatedAt, Payload: payload},
	); err != nil {
		return empty, err
	}

	skipped, err := loop.New(g).SettleGate(run)
	if err != nil {
		return empty, err
	}
	if !skipped {
		return empty, fmt.Errorf("flag %q is set but gate %q did not open", spec.Flag, run.CurrentNode)
	}
	return GateAnswer{Answered: true, Flag: spec.Flag}, nil
}
