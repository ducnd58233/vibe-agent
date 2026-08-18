package app

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func handleSessionCheckpoint(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/session/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "checkpoint" {
		http.NotFound(w, r)
		return
	}
	slug := parts[0]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	checkName := strings.TrimSpace(r.FormValue("check"))
	verdict := strings.TrimSpace(r.FormValue("verdict"))
	note := strings.TrimSpace(r.FormValue("note"))
	if checkName == "" || (verdict != "passed" && verdict != "failed") {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	ws := d.activeWorkspace(r)
	run, err := state.Load(state.ManifestPath(ws, slug))
	if err != nil {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	loaded, err := graph.LoadByID(graph.DefaultDir(d.toolkitRoot), run.GraphID)
	if err != nil {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	node, ok := loaded.Node(run.CurrentNode)
	if !ok || node.Type != graph.NodeHumanGate {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	if node.Check == "" || node.Check != checkName {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	if len(note) > 500 {
		note = note[:500]
	}
	ref := note
	if ref == "" {
		ref = "web human_gate"
	}
	_, err = checkpoint.Apply(checkpoint.Request{
		WorkspaceRoot: ws,
		GraphDir:      graph.DefaultDir(d.toolkitRoot),
		Slug:          slug,
		Outcome: loop.Outcome{
			Check: &loop.NamedCheck{
				Name: checkName,
				Check: state.Check{
					Passed: verdict == "passed",
					Source: state.SourceHumanEvent,
					Ref:    ref,
					At:     time.Now().UTC(),
				},
			},
		},
	})
	if err != nil {
		http.Error(w, "checkpoint failed", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/session/"+url.PathEscape(slug)+"?view=chat", http.StatusSeeOther)
}
