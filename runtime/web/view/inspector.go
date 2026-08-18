package view

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// EventDetail is client-side inspector data for one row.
type EventDetail struct {
	Seq     int    `json:"seq"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Payload string `json:"payload"`
	Result  string `json:"result"`
	Schema  string `json:"schema"`
	Timing  string `json:"timing"`
	Tokens  string `json:"tokens"`
}

// BuildEventDetails prepares inspector panes for each row.
func BuildEventDetails(rows []EventRow) []EventDetail {
	out := make([]EventDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, buildEventDetail(row))
	}
	return out
}

func buildEventDetail(row EventRow) EventDetail {
	status := "completed"
	if row.Failed {
		status = "failed"
	} else if row.HostGap {
		status = "host gap"
	}
	model := extractModel(row.PayloadJSON)
	tokens := formatRowTokens(row)
	duration := extractDuration(row.PayloadJSON)
	summary := `<dl class="kv" data-testid="inspector-summary">` +
		kvPair("Time", formatEventTime(row.At)) +
		kvPair("Type", reported(string(row.Type))) +
		kvPair("Role", reported(row.Role)) +
		kvPair("Kind", reported(string(row.Kind))) +
		kvPair("Source", reported(string(row.Source))) +
		kvPair("Seq", fmt.Sprintf("%d", row.Seq)) +
		kvPair("Client", reported(row.Client)) +
		kvPair("Tool", reported(row.Tool)) +
		kvPair("Command", reported(row.Command)) +
		kvPair("Event", reported(row.EventName)) +
		kvPair("Status", status) +
		kvPair("Failed", yesNo(row.Failed)) +
		kvPair("Host gap", yesNo(row.HostGap)) +
		kvPair("Redacted", yesNo(row.Redacted)) +
		fmt.Sprintf(`<dt>Model</dt><dd data-testid="model-name">%s</dd>`, template.HTMLEscapeString(model)) +
		fmt.Sprintf(`<dt>Tokens</dt><dd data-testid="inspector-tokens">%s</dd>`, template.HTMLEscapeString(tokens)) +
		kvPair("Duration", duration) +
		kvPair("Body", excerpt(row.Body, 400)) +
		`</dl>`
	payload := `<h3 class="panel-title">Payload</h3><pre data-testid="inspector-payload" style="white-space:pre-wrap;overflow-wrap:anywhere">` +
		template.HTMLEscapeString(prettyJSON(row.PayloadJSON)) + `</pre>`
	resultText := extractResult(row.PayloadJSON)
	result := `<h3 class="panel-title">Result</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">` +
		template.HTMLEscapeString(resultText) + `</pre>`
	schema := `<h3 class="panel-title">Schema</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">` + eventSchema + `</pre>`
	timing := fmt.Sprintf(`<dl class="kv"><dt>Time</dt><dd>%s</dd><dt>Duration</dt><dd>%s</dd><dt>Source</dt><dd>%s</dd><dt>Tokens</dt><dd data-testid="inspector-tokens">%s</dd></dl>`,
		template.HTMLEscapeString(formatEventTime(row.At)),
		template.HTMLEscapeString(duration),
		template.HTMLEscapeString(string(row.Source)),
		tokens,
	)
	return EventDetail{
		Seq:     row.Seq,
		Title:   string(row.Kind) + " · " + row.Summary,
		Summary: summary,
		Payload: payload,
		Result:  result,
		Schema:  schema,
		Timing:  timing,
		Tokens:  tokens,
	}
}

func kvPair(label, value string) string {
	return "<dt>" + template.HTMLEscapeString(label) + "</dt><dd>" + template.HTMLEscapeString(value) + "</dd>"
}

func reported(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "not reported"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatEventTime(at time.Time) string {
	if at.IsZero() {
		return "not reported"
	}
	return at.UTC().Format(time.RFC3339)
}

func excerpt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "not reported"
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

const eventSchema = `{
  "seq": "int",
  "role": "system | user | context | assistant | tool | hook | graph",
  "kind": "hook | tool | skill | graph | transcript",
  "source": "hook | transcript | graph",
  "summary": "string",
  "payload": "redacted object",
  "result": "string or empty",
  "durationMs": "int",
  "usage": "optional { input, output, cacheRead }"
}`

func prettyJSON(raw string) string {
	if raw == "" {
		return "(empty)"
	}
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	encoded, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return raw
	}
	return string(encoded)
}

func extractModel(payload string) string {
	if payload == "" {
		return "unknown"
	}
	var body struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "unknown"
	}
	if body.Model == "" {
		return "unknown"
	}
	return body.Model
}

func extractResult(payload string) string {
	if payload == "" {
		return "(empty)"
	}
	var body struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "(empty)"
	}
	if body.Result == "" {
		return "(empty)"
	}
	return body.Result
}

func extractDuration(payload string) string {
	if payload == "" {
		return "not reported"
	}
	var body struct {
		DurationMS int `json:"durationMs"`
	}
	if err := json.Unmarshal([]byte(payload), &body); err != nil || body.DurationMS <= 0 {
		return "not reported"
	}
	return fmt.Sprintf("%d ms", body.DurationMS)
}

func formatRowTokens(row EventRow) string {
	if row.TokensText != "" {
		return row.TokensText
	}
	if !row.HasUsage || row.Usage == nil {
		return "not reported"
	}
	return formatUsage(*row.Usage)
}
