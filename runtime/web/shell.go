package web

import (
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// HostRow is one PATH inventory line for the shell sidebar.
type HostRow struct {
	Binary string
	OnPath bool
	Reason string
}

// ShellPage is the empty-state template model.
type ShellPage struct {
	Workspace   string
	BindAddr    string
	URL         string
	Hosts       []HostRow
	Sessions    []string
	HasSessions bool
}

// BuildShellPage loads workspace metadata for the empty shell.
func BuildShellPage(workspaceRoot string, port int) (ShellPage, error) {
	addr := Addr(port)
	page := ShellPage{
		Workspace: workspaceRoot,
		BindAddr:  addr,
		URL:       "http://" + addr + "/",
	}
	for _, entry := range hosts.Inventory() {
		reason := entry.Reason
		if entry.OnPath {
			reason = "on PATH"
		}
		page.Hosts = append(page.Hosts, HostRow{
			Binary: entry.Binary,
			OnPath: entry.OnPath,
			Reason: reason,
		})
	}
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return page, err
	}
	page.Sessions = slugs
	if hasAmbientSession(workspaceRoot) {
		page.Sessions = append(page.Sessions, "ambient")
	}
	page.HasSessions = len(page.Sessions) > 0
	return page, nil
}

func hasAmbientSession(workspaceRoot string) bool {
	path := session.AmbientLogPath(workspaceRoot)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
