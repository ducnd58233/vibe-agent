package view

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
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
	summary := fmt.Sprintf(`<dl class="kv">
<dt>Hierarchy</dt><dd>%s message</dd>
<dt>Status</dt><dd>%s</dd>
<dt>Kind</dt><dd>%s</dd>
<dt>Source</dt><dd>%s</dd>
<dt>Seq</dt><dd>%d</dd>
<dt>Model</dt><dd data-testid="model-name">%s</dd>
<dt>Tokens</dt><dd data-testid="inspector-tokens">%s</dd>
</dl>`,
		template.HTMLEscapeString(strings.ToUpper(row.Role)),
		template.HTMLEscapeString(status),
		template.HTMLEscapeString(string(row.Kind)),
		template.HTMLEscapeString(string(row.Source)),
		row.Seq,
		template.HTMLEscapeString(model),
		tokens,
	)
	payload := `<h3 class="panel-title">Payload</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">` +
		template.HTMLEscapeString(prettyJSON(row.PayloadJSON)) + `</pre>`
	resultText := extractResult(row.PayloadJSON)
	result := `<h3 class="panel-title">Result</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">` +
		template.HTMLEscapeString(resultText) + `</pre>`
	schema := `<h3 class="panel-title">Schema</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">` + eventSchema + `</pre>`
	duration := extractDuration(row.PayloadJSON)
	timing := fmt.Sprintf(`<dl class="kv"><dt>Duration</dt><dd>%s</dd><dt>Source</dt><dd>%s</dd><dt>Tokens</dt><dd data-testid="inspector-tokens">%s</dd></dl>`,
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

const eventSchema = `{
  "seq": "int",
  "role": "system | user | context | assistant | tool",
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
	if !row.HasUsage || row.Usage == nil {
		return "in — · out —"
	}
	text := fmt.Sprintf("in %d · out %d", row.Usage.Input, row.Usage.Output)
	if row.Usage.CacheRead > 0 {
		text += fmt.Sprintf(" · cache read %d", row.Usage.CacheRead)
	}
	return text
}
