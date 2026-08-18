package view

import (
	"encoding/json"
	"html/template"
	"os"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

// HostRow is one PATH inventory line for the shell sidebar.
type HostRow struct {
	Binary string
	ID     string
	OnPath bool
	Reason string
}

// ShellPage is the empty-state template model.
type ShellPage struct {
	Workspace           string
	BindAddr            string
	URL                 string
	Hosts               []HostRow
	Sessions            []SessionRow
	HasSessions         bool
	CurrentSlug         string
	CanCompose          bool
	ComposeHosts        []HostRow
	Workspaces          []WorkspaceRow
	CurrentWorkspace    WorkspaceRow
	RecentWorkspaces    []WorkspaceRow
	HasWorkspaces       bool
	HasRecentWorkspaces bool
}

// BuildShellPage loads workspace metadata for the empty shell.
func BuildShellPage(workspaceRoot, bindAddr string, reg domain.Registry, activeRoot string) (ShellPage, error) {
	page := ShellPage{
		Workspace: workspaceRoot,
		BindAddr:  bindAddr,
		URL:       "http://" + bindAddr + "/",
	}
	page.Workspaces = ProjectWorkspaces(reg, activeRoot)
	page.CurrentWorkspace, page.RecentWorkspaces = SplitWorkspaces(page.Workspaces)
	page.HasWorkspaces = len(page.Workspaces) > 0
	page.HasRecentWorkspaces = len(page.RecentWorkspaces) > 0
	for _, entry := range hosts.Inventory() {
		reason := entry.Reason
		if entry.OnPath {
			reason = "on PATH"
		}
		row := HostRow{
			Binary: entry.Binary,
			ID:     entry.ID,
			OnPath: entry.OnPath,
			Reason: reason,
		}
		page.Hosts = append(page.Hosts, row)
		if entry.OnPath {
			page.CanCompose = true
			page.ComposeHosts = append(page.ComposeHosts, row)
		}
	}
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return page, err
	}
	page.Sessions = ProjectSessions(workspaceRoot, slugs, time.Now().UTC())
	page.HasSessions = len(page.Sessions) > 0
	return page, nil
}

// SessionPage is the trajectory shell for one slug.
type SessionPage struct {
	ShellPage
	Slug            string
	RunStatus       string
	Events          []EventRow
	KindCounts      map[session.FilterKind]int
	Tokens          UsageTotals
	ToolbarTokens   string
	ChatEmpty       bool
	ChatPrompts     []ChatPrompt
	HasChatPrompts  bool
	TurnCount       int
	ToolCalls       int
	EventDetails    []EventDetail
	EventDataJSON   template.JS
	GraphID         string
	CurrentNode     string
	GraphNodes      []GraphNodeRow
	GraphTypeCounts map[string]int
	GraphNodeJSON   template.JS
	LastEventSeq    int
}

// BuildSessionPage loads one session log for rendering.
func BuildSessionPage(workspaceRoot, toolkitRoot, bindAddr, slug string, reg domain.Registry, activeRoot string) (SessionPage, error) {
	shell, err := BuildShellPage(workspaceRoot, bindAddr, reg, activeRoot)
	if err != nil {
		return SessionPage{}, err
	}
	page := SessionPage{
		ShellPage: shell,
		Slug:      slug,
		RunStatus: "idle",
	}
	page.CurrentSlug = slug
	var logPath string
	var run *state.Run
	switch slug {
	case "ambient":
		logPath = session.AmbientLogPath(workspaceRoot)
	default:
		logPath = session.LogPath(workspaceRoot, slug)
		if loaded, loadErr := state.Load(state.ManifestPath(workspaceRoot, slug)); loadErr == nil && loaded != nil {
			run = loaded
			page.RunStatus = string(run.Status)
			page.CurrentNode = run.CurrentNode
			page.GraphID = run.GraphID
		}
	}
	if page.GraphID == "" {
		page.GraphID = "goal-delivery"
	}
	if g, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), page.GraphID); err == nil && g != nil {
		page.GraphNodes = ProjectGraph(g, run)
		page.GraphTypeCounts = GraphTypeCounts(page.GraphNodes)
	}
	events, err := session.Replay(logPath)
	if err != nil && !os.IsNotExist(err) {
		return SessionPage{}, err
	}
	page.Events = ProjectEvents(events)
	page.KindCounts = KindCounts(page.Events)
	page.Tokens = SumUsage(page.Events)
	page.ToolbarTokens = FormatToolbarTokens(page.Tokens)
	page.ChatPrompts = AwaitingChatPrompts(page.GraphNodes, slug)
	page.HasChatPrompts = len(page.ChatPrompts) > 0
	page.ChatEmpty = !ChatHasProse(page.Events) && !page.HasChatPrompts
	page.TurnCount = countTurns(page.Events)
	page.ToolCalls = countToolCalls(page.Events)
	page.EventDetails = BuildEventDetails(page.Events)
	if encoded, err := json.Marshal(page.EventDetails); err == nil {
		page.EventDataJSON = template.JS(encoded) //nolint:gosec // G203 JSON script block from server-built rows
	}
	page.LastEventSeq = LastSequence(workspaceRoot, slug)
	return page, nil
}

func countTurns(rows []EventRow) int {
	n := 0
	for _, row := range rows {
		if row.Role == "user" {
			n++
		}
	}
	return n
}

func countToolCalls(rows []EventRow) int {
	n := 0
	for _, row := range rows {
		if row.Role == "tool" {
			n++
		}
	}
	return n
}

func hasAmbientSession(workspaceRoot string) bool {
	path := session.AmbientLogPath(workspaceRoot)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
