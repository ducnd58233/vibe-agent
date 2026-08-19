package view

import (
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// ChatPrompt is a graph question that belongs on the Chat tab.
type ChatPrompt struct {
	NodeID    string
	Type      string
	Title     string
	Prompt    string
	Check     string
	CanDecide bool
}

const confirmGoalTitle = "Confirm this goal"

// AwaitingChatPrompts returns human_gate and verifier cards for the current node.
// When goal is set, a human_gate card titles a confirm line and shows that goal,
// not the graph YAML description.
func AwaitingChatPrompts(rows []GraphNodeRow, slug, goal string) []ChatPrompt {
	goal = session.RedactText(strings.TrimSpace(goal))
	out := make([]ChatPrompt, 0)
	for _, row := range rows {
		if !row.Current || row.Status != string(GraphStatusAwaiting) {
			continue
		}
		switch row.Type {
		case string(graph.NodeHumanGate), string(graph.NodeVerifier):
		default:
			continue
		}
		prompt := strings.ReplaceAll(row.Prompt, "${slug}", slug)
		title := row.Description
		if strings.TrimSpace(title) == "" {
			title = row.ID
		}
		if row.Type == string(graph.NodeHumanGate) && goal != "" {
			title = confirmGoalTitle
			prompt = goal
		}
		if strings.TrimSpace(prompt) == strings.TrimSpace(title) {
			prompt = ""
		}
		out = append(out, ChatPrompt{
			NodeID:    row.ID,
			Type:      row.Type,
			Title:     title,
			Prompt:    prompt,
			Check:     row.Check,
			CanDecide: row.Type == string(graph.NodeHumanGate) && row.Check != "",
		})
	}
	return out
}
