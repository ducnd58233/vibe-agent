package view

import (
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
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

// AwaitingChatPrompts returns human_gate and verifier cards for the current node.
func AwaitingChatPrompts(rows []GraphNodeRow, slug string) []ChatPrompt {
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
