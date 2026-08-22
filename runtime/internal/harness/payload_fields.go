package harness

import (
	"bytes"
	"encoding/json"
	"strings"
)

func firstRawString(fields map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func applyToolInput(body *payload, raw json.RawMessage) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return
	}
	if raw[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return
		}
		applyToolInput(body, json.RawMessage(encoded))
		return
	}
	if raw[0] != '{' {
		return
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(raw, &input); err != nil {
		return
	}
	if body.ToolInput.Command == "" {
		body.ToolInput.Command = firstRawString(input, "command", "CommandLine")
	}
	if body.ToolInput.FilePath == "" {
		body.ToolInput.FilePath = firstRawString(input, "file_path", "filePath", "path")
	}
	if body.ToolInput.NotebookPath == "" {
		body.ToolInput.NotebookPath = firstRawString(input, "notebook_path", "notebookPath")
	}
	if body.Command == "" {
		body.Command = firstRawString(input, "pattern", "query")
	}
}

func (p *payload) enrichFromRaw() {
	if len(p.raw) == 0 {
		return
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(p.raw, &top); err != nil {
		return
	}
	if p.ToolName == "" {
		p.ToolName = firstRawString(top, "tool_name", "toolName", "tool", "name")
	}
	if p.Command == "" {
		p.Command = firstRawString(top, "command")
	}
	if p.FilePath == "" {
		p.FilePath = firstRawString(top, "file_path", "filePath")
	}
	if raw, ok := top["tool_input"]; ok {
		applyToolInput(p, raw)
	}
	if raw, ok := top["toolCall"]; ok {
		applyToolCall(p, raw)
	}
	if p.TranscriptPath == "" {
		p.TranscriptPath = firstRawString(top, "transcriptPath", "transcript_path")
	}
}

func applyToolCall(body *payload, raw json.RawMessage) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return
	}
	var call map[string]json.RawMessage
	if err := json.Unmarshal(raw, &call); err != nil {
		return
	}
	if body.ToolName == "" {
		body.ToolName = firstRawString(call, "name")
	}
	if args, ok := call["args"]; ok {
		applyToolInput(body, args)
		if body.ToolInput.Command == "" {
			var argsMap map[string]json.RawMessage
			if err := json.Unmarshal(args, &argsMap); err == nil {
				body.ToolInput.Command = firstRawString(argsMap, "CommandLine", "command")
			}
		}
	}
}

func (p payload) sessionCommand() string {
	if cmd := strings.TrimSpace(p.shellCommand()); cmd != "" {
		return cmd
	}
	return strings.TrimSpace(p.writeTarget())
}

func (p payload) sessionTool() string {
	if name := strings.TrimSpace(p.ToolName); name != "" {
		return name
	}
	if strings.TrimSpace(p.shellCommand()) != "" {
		return "Shell"
	}
	return ""
}
