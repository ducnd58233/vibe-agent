package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const composerTimeout = 2 * time.Minute

// SendComposerMessage records a redacted user prompt and optional print output.
func SendComposerMessage(ctx context.Context, workspaceRoot, slug, hostID, message string) error {
	if message == "" {
		return fmt.Errorf("message required")
	}
	host, ok := hosts.EvalHost(hostID)
	if !ok {
		return fmt.Errorf("unknown host")
	}
	entry := hostsInventoryEntry(host)
	if !entry.OnPath {
		return fmt.Errorf("host not on PATH")
	}
	logPath := session.LogPath(workspaceRoot, slug)
	if _, err := session.Append(logPath, session.Record{
		Type:   session.TypePromptSubmit,
		Source: session.SourceHook,
		Client: host.Binary,
		Body:   message,
		At:     time.Now().UTC(),
	}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, composerTimeout)
	defer cancel()
	out, err := runHostPrint(ctx, host, message)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	text := session.RedactText(strings.TrimSpace(out))
	if text == "" {
		return nil
	}
	if parsed := parsePrintLines(out); len(parsed) > 0 {
		for _, line := range parsed {
			if line == "" {
				continue
			}
			if _, err := session.Append(logPath, session.Record{
				Type:   session.TypeTranscriptMessage,
				Source: session.SourcePrint,
				Client: host.Binary,
				Role:   "assistant",
				Body:   session.RedactText(line),
				At:     time.Now().UTC(),
			}); err != nil {
				return err
			}
		}
		return nil
	}
	_, err = session.Append(logPath, session.Record{
		Type:   session.TypeTranscriptMessage,
		Source: session.SourcePrint,
		Client: host.Binary,
		Role:   "assistant",
		Body:   text,
		At:     time.Now().UTC(),
	})
	return err
}

func hostsInventoryEntry(host hosts.Host) hosts.Entry {
	for _, entry := range hosts.Inventory() {
		if entry.Binary == host.Binary {
			return entry
		}
	}
	return hosts.Entry{Host: host, Reason: host.Binary + " not on PATH"}
}

func runHostPrint(ctx context.Context, host hosts.Host, prompt string) (string, error) {
	parts := strings.Fields(host.EvalCommand)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty host command")
	}
	args := append([]string{}, parts[1:]...)
	if host.PromptAsArg {
		args = append(args, prompt)
	}
	cmd, err := safexec.CommandContext(ctx, parts[0], args...)
	if err != nil {
		return "", err
	}
	if !host.PromptAsArg {
		cmd.Stdin = strings.NewReader(prompt)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parsePrintLines(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return nil
	}
	var texts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var item struct {
			Type    string `json:"type"`
			Content string `json:"content"`
			Text    string `json:"text"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		switch item.Type {
		case "message", "assistant_message", "agent_message":
			body := item.Content
			if body == "" {
				body = item.Text
			}
			if body != "" {
				texts = append(texts, body)
			}
		}
	}
	return texts
}
