package harness

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

type projectedMessage struct {
	role    string
	body    string
	tool    string
	command string
	usage   *session.Usage
}

// projectTranscript appends transcript-sourced session rows when the payload
// names a JSONL file. Unfamiliar lines are skipped. skipAssistantBody drops an
// assistant row that duplicates Stop last_assistant_message text.
func projectTranscript(req Request, body payload, skipAssistantBody string) {
	path := body.TranscriptPath
	if path == "" {
		path = body.AgentTranscriptPath
	}
	if path == "" {
		return
	}

	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	skip := strings.TrimSpace(skipAssistantBody)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		msg, ok := parseTranscriptLine(line)
		if !ok {
			continue
		}
		if msg.role == "assistant" && skip != "" && strings.TrimSpace(msg.body) == skip {
			continue
		}
		record := session.Record{
			Type:    session.TypeTranscriptMessage,
			Source:  session.SourceTranscript,
			Role:    msg.role,
			Body:    msg.body,
			Tool:    msg.tool,
			Command: msg.command,
			Usage:   msg.usage,
		}
		appendSession(req, record)
	}
}

func parseTranscriptLine(line string) (projectedMessage, bool) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return projectedMessage{}, false
	}

	if typ, ok := raw["type"].(string); ok {
		switch typ {
		case "user":
			if text := extractTranscriptText(raw); text != "" {
				return projectedMessage{role: "user", body: text}, true
			}
		case "assistant":
			if text := extractTranscriptText(raw); text != "" {
				return projectedMessage{role: "assistant", body: text, usage: extractTranscriptUsage(raw)}, true
			}
		case "tool_result":
			if text := stringField(raw, "content", "result", "output"); text != "" {
				return projectedMessage{role: "tool", body: text}, true
			}
		}
	}

	if text := stringField(raw, "tool_result"); text != "" {
		return projectedMessage{role: "tool", body: text}, true
	}

	if toolInput, ok := raw["tool_input"].(map[string]any); ok {
		tool := stringField(raw, "tool_name", "name")
		command := stringField(toolInput, "command")
		if tool != "" || command != "" {
			return projectedMessage{role: "tool", tool: tool, command: command}, true
		}
	}

	if role, ok := raw["role"].(string); ok {
		if text := extractTranscriptText(raw); text != "" {
			usage := (*session.Usage)(nil)
			if role == "assistant" {
				usage = extractTranscriptUsage(raw)
			}
			return projectedMessage{role: role, body: text, usage: usage}, true
		}
	}

	if text := extractContentArrayText(raw); text != "" {
		role := "assistant"
		if r, ok := raw["role"].(string); ok && r != "" {
			role = r
		}
		usage := (*session.Usage)(nil)
		if role == "assistant" {
			usage = extractTranscriptUsage(raw)
		}
		return projectedMessage{role: role, body: text, usage: usage}, true
	}

	return projectedMessage{}, false
}

func extractTranscriptText(raw map[string]any) string {
	if text := stringField(raw, "text", "content"); text != "" {
		return text
	}
	if message, ok := raw["message"].(map[string]any); ok {
		if text := stringField(message, "content"); text != "" {
			return text
		}
		if text := extractContentArrayText(message); text != "" {
			return text
		}
	}
	return extractContentArrayText(raw)
}

func extractContentArrayText(raw map[string]any) string {
	content, ok := raw["content"].([]any)
	if !ok {
		if message, ok := raw["message"].(map[string]any); ok {
			if nested, ok := message["content"].([]any); ok {
				content = nested
			}
		}
	}
	if content == nil {
		return ""
	}
	var parts []string
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := block["type"].(string); typ != "" && typ != "text" {
			continue
		}
		if text := stringField(block, "text"); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractTranscriptUsage(raw map[string]any) *session.Usage {
	if u := session.ParseUsage(raw); u != nil {
		return u
	}
	if message, ok := raw["message"].(map[string]any); ok {
		return session.ParseUsage(message)
	}
	return nil
}

func stringField(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}
