package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const composerTimeout = 2 * time.Minute

var hostPrint = runHostPrint

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
	go appendHostPrint(ctx, workspaceRoot, slug, host, message)
	return nil
}

func appendHostPrint(parent context.Context, workspaceRoot, slug string, host hosts.Host, message string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), composerTimeout)
	defer cancel()
	out, err := hostPrint(ctx, host, message)
	if err != nil || strings.TrimSpace(out) == "" {
		body := "Host produced no output."
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			body = "Host timed out while generating output."
		}
		logPath := session.LogPath(workspaceRoot, slug)
		_, _ = session.Append(logPath, session.Record{
			Type:   session.TypeTranscriptMessage,
			Source: session.SourcePrint,
			Client: host.Binary,
			Role:   "assistant",
			Body:   session.RedactText(body),
			At:     time.Now().UTC(),
		})
		return
	}
	logPath := session.LogPath(workspaceRoot, slug)
	text := session.RedactText(strings.TrimSpace(out))
	if text == "" {
		return
	}
	fragments, usage := parsePrintOutput(out)
	if len(fragments) == 0 {
		fragments = []printFragment{{Role: "assistant", Body: text}}
	}
	attachUsage(fragments, usage)
	now := time.Now().UTC()
	for _, frag := range fragments {
		if strings.TrimSpace(frag.Body) == "" && frag.Usage == nil {
			continue
		}
		body := frag.Body
		if body == "" {
			body = "Host finished."
		}
		_, _ = session.Append(logPath, session.Record{
			Type:   session.TypeTranscriptMessage,
			Source: session.SourcePrint,
			Client: host.Binary,
			Role:   frag.Role,
			Body:   session.RedactText(body),
			Usage:  frag.Usage,
			At:     now,
		})
	}
}

type printFragment struct {
	Role  string
	Body  string
	Usage *session.Usage
}

func attachUsage(fragments []printFragment, usage *session.Usage) {
	if usage == nil || len(fragments) == 0 {
		return
	}
	for i := len(fragments) - 1; i >= 0; i-- {
		if fragments[i].Role == "assistant" {
			fragments[i].Usage = usage
			return
		}
	}
	fragments[len(fragments)-1].Usage = usage
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
	fragments, _ := parsePrintOutput(raw)
	texts := make([]string, 0, len(fragments))
	for _, frag := range fragments {
		if frag.Body != "" {
			texts = append(texts, frag.Body)
		}
	}
	return texts
}

func parsePrintOutput(raw string) ([]printFragment, *session.Usage) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return nil, nil
	}
	var fragments []printFragment
	var usage *session.Usage
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var rawObj map[string]any
		if err := json.Unmarshal([]byte(line), &rawObj); err != nil {
			continue
		}
		if parsed := session.ParseUsage(rawObj); parsed != nil {
			usage = parsed
		}
		if nested, ok := rawObj["message"].(map[string]any); ok {
			if parsed := session.ParseUsage(nested); parsed != nil {
				usage = parsed
			}
		}
		typ, _ := rawObj["type"].(string)
		if frag, ok := fragmentFromPrint(typ, rawObj); ok {
			fragments = append(fragments, frag)
		}
	}
	return fragments, usage
}

func fragmentFromPrint(typ string, raw map[string]any) (printFragment, bool) {
	switch typ {
	case "question", "ask_user", "user_question", "permission_request", "elicitation":
		body := firstString(raw, "text", "content", "prompt", "message")
		if body == "" {
			return printFragment{}, false
		}
		return printFragment{Role: "question", Body: body}, true
	case "message", "assistant_message", "agent_message", "assistant":
		body := firstString(raw, "content", "text")
		if body == "" {
			body = nestedAssistantText(raw)
		}
		if body == "" {
			return printFragment{}, false
		}
		return printFragment{Role: "assistant", Body: body}, true
	case "item.completed":
		item, _ := raw["item"].(map[string]any)
		if item == nil {
			return printFragment{}, false
		}
		itemType, _ := item["type"].(string)
		body := firstString(item, "text", "content", "prompt", "message")
		if body == "" {
			return printFragment{}, false
		}
		if isQuestionItem(itemType) {
			return printFragment{Role: "question", Body: body}, true
		}
		if itemType == "agent_message" {
			return printFragment{Role: "assistant", Body: body}, true
		}
	}
	return printFragment{}, false
}

func isQuestionItem(itemType string) bool {
	switch itemType {
	case "user_input", "question", "ask", "permission", "elicitation":
		return true
	default:
		return false
	}
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := raw[key].(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func nestedAssistantText(raw map[string]any) string {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return ""
	}
	if body := firstString(msg, "text"); body != "" {
		return body
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return firstString(msg, "content")
	}
	var parts []string
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := firstString(block, "text"); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}
