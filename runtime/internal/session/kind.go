package session

import "strings"

// Kind maps a stored event to the UI filter bucket.
func Kind(ev Event) FilterKind {
	switch ev.Type {
	case TypeSessionStart, TypePromptSubmit, TypePreTool, TypeStop, TypeSubagentStop:
		return FilterHook
	case TypeTranscriptMessage:
		return FilterTranscript
	case TypeToolUse:
		tool := strings.ToLower(evToolName(ev))
		if tool == "skill" {
			return FilterSkill
		}
		return FilterTool
	default:
		return FilterHook
	}
}

func evToolName(ev Event) string {
	var body Payload
	if len(ev.Payload) == 0 {
		return ""
	}
	if err := decodePayload(ev.Payload, &body); err != nil {
		return ""
	}
	return body.Tool
}
