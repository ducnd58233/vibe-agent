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
func SendComposerMessage(ctx context.Context, workspaceRoot, slug, hostID, message string, opts hosts.PrintOptions) error {
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
	prompt := message
	if prefix := session.ComposePrefixFromLog(logPath); prefix != "" {
		prompt = prefix + "\n\n" + message
	}
	prompt = session.RedactText(prompt)
	if _, err := session.Append(logPath, session.Record{
		Type:   session.TypePromptSubmit,
		Source: session.SourceHook,
		Client: host.Binary,
		Body:   message,
		At:     time.Now().UTC(),
	}); err != nil {
		return err
	}
	go appendHostPrint(ctx, workspaceRoot, slug, host, prompt, opts)
	return nil
}

func appendHostPrint(parent context.Context, workspaceRoot, slug string, host hosts.Host, message string, opts hosts.PrintOptions) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), composerTimeout)
	defer cancel()
	out, err := hostPrint(ctx, host, message, opts)
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
		if looksLikeJSONStream(out) {
			fragments = []printFragment{emptyDisplayFragment()}
		} else {
			fragments = []printFragment{{Role: "assistant", Body: text}}
		}
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
		rec := session.Record{
			Type:   session.TypeTranscriptMessage,
			Source: session.SourcePrint,
			Client: host.Binary,
			Role:   frag.Role,
			Body:   session.RedactText(body),
			Usage:  frag.Usage,
			At:     now,
		}
		if frag.Type == session.TypeToolUse {
			rec.Type = session.TypeToolUse
			rec.Tool = frag.Tool
			rec.Command = session.RedactText(frag.Command)
			rec.Body = session.RedactText(frag.Body)
		}
		_, _ = session.Append(logPath, rec)
	}
}

const emptyPrintCopy = "Host produced no displayable output."

func emptyDisplayFragment() printFragment {
	return printFragment{Role: "assistant", Body: emptyPrintCopy}
}

type printFragment struct {
	Type    session.Type
	Role    string
	Body    string
	Tool    string
	Command string
	Usage   *session.Usage
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

func runHostPrint(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
	parts := strings.Fields(host.EvalCommand)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty host command")
	}
	args := hosts.PrintArgv(host, opts)
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
	var thinking strings.Builder
	flushThinking := func() {
		text := strings.TrimSpace(thinking.String())
		thinking.Reset()
		if text == "" {
			return
		}
		fragments = append(fragments, printFragment{Role: "thinking", Body: text})
	}
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
		switch typ {
		case "thinking", "reasoning":
			subtype, _ := rawObj["subtype"].(string)
			if subtype == "completed" {
				flushThinking()
				continue
			}
			if text := firstString(rawObj, "text", "thinking", "content"); text != "" {
				thinking.WriteString(text)
			}
			continue
		case "system", "result", "user", "turn.started", "turn.completed", "thread.started":
			continue
		}
		flushThinking()
		fragments = append(fragments, fragmentsFromPrint(typ, rawObj)...)
	}
	flushThinking()
	return fragments, usage
}

func fragmentsFromPrint(typ string, raw map[string]any) []printFragment {
	switch typ {
	case "question", "ask_user", "user_question", "permission_request", "elicitation":
		body := firstString(raw, "text", "content", "prompt", "message")
		if body == "" {
			return nil
		}
		return []printFragment{{Role: "question", Body: body}}
	case "tool_call":
		if frag, ok := cursorToolCall(raw); ok {
			return []printFragment{frag}
		}
		return nil
	case "tool_use":
		name := firstString(raw, "name", "tool")
		if name == "" {
			return nil
		}
		cmd := summarizeToolArgs(name, raw)
		return []printFragment{{Type: session.TypeToolUse, Role: "tool", Tool: name, Command: cmd, Body: cmd}}
	case "message", "assistant_message", "agent_message", "assistant":
		return assistantFragments(raw)
	case "item.completed":
		item, _ := raw["item"].(map[string]any)
		if item == nil {
			return nil
		}
		itemType, _ := item["type"].(string)
		body := firstString(item, "text", "content", "prompt", "message")
		if body == "" {
			return nil
		}
		if isQuestionItem(itemType) {
			return []printFragment{{Role: "question", Body: body}}
		}
		if itemType == "agent_message" {
			return []printFragment{{Role: "assistant", Body: body}}
		}
	}
	return nil
}

func assistantFragments(raw map[string]any) []printFragment {
	var out []printFragment
	for _, block := range messageContentBlocks(raw) {
		blockType, _ := block["type"].(string)
		switch blockType {
		case "thinking":
			if text := firstString(block, "thinking", "text"); text != "" {
				out = append(out, printFragment{Role: "thinking", Body: text})
			}
		case "tool_use":
			name := firstString(block, "name")
			if name == "" {
				continue
			}
			cmd := summarizeToolArgs(name, block)
			out = append(out, printFragment{Type: session.TypeToolUse, Role: "tool", Tool: name, Command: cmd, Body: cmd})
		default:
			if text := firstString(block, "text"); text != "" {
				out = append(out, printFragment{Role: "assistant", Body: text})
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	body := firstString(raw, "content", "text")
	if body == "" {
		body = nestedAssistantText(raw)
	}
	if body == "" {
		return nil
	}
	return []printFragment{{Role: "assistant", Body: body}}
}

func messageContentBlocks(raw map[string]any) []map[string]any {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(content))
	for _, item := range content {
		block, ok := item.(map[string]any)
		if ok {
			out = append(out, block)
		}
	}
	return out
}

func cursorToolCall(raw map[string]any) (printFragment, bool) {
	subtype, _ := raw["subtype"].(string)
	if subtype == "completed" {
		return printFragment{}, false
	}
	tc, _ := raw["tool_call"].(map[string]any)
	if tc == nil {
		return printFragment{}, false
	}
	preferred := []string{"readToolCall", "writeToolCall", "grepToolCall", "shellToolCall", "function"}
	for _, key := range preferred {
		if val, ok := tc[key]; ok {
			return toolCallValue(key, val)
		}
	}
	for key, val := range tc {
		return toolCallValue(key, val)
	}
	return printFragment{}, false
}

func toolCallValue(key string, val any) (printFragment, bool) {
	name := strings.TrimSuffix(key, "ToolCall")
	cmd := name
	if m, ok := val.(map[string]any); ok {
		if key == "function" {
			if n := firstString(m, "name"); n != "" {
				name = n
			}
			if args := firstString(m, "arguments"); args != "" {
				cmd = name + " " + args
			} else {
				cmd = name
			}
		} else if args, ok := m["args"].(map[string]any); ok {
			cmd = summarizeToolArgs(name, args)
		}
	}
	if name == "" {
		return printFragment{}, false
	}
	return printFragment{Type: session.TypeToolUse, Role: "tool", Tool: name, Command: cmd, Body: cmd}, true
}

func summarizeToolArgs(name string, args map[string]any) string {
	if p := firstString(args, "path"); p != "" {
		return name + " " + p
	}
	if c := firstString(args, "command"); c != "" {
		return c
	}
	if q := firstString(args, "pattern", "query"); q != "" {
		return name + " " + q
	}
	return name
}

func looksLikeJSONStream(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line[0] == '{'
	}
	return false
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
