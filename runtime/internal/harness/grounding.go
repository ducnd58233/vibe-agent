package harness

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The grounding rule - never describe a file you have not opened, report
// ACCESS-FAILED instead - is written in AGENTS.md, which leaves the model to
// decide whether it followed it. This turns it into a sensor: it reads the
// subagent's own transcript and names the paths the final message cites that
// appear in no tool call and no tool result.
//
// Warn only, always. SubagentStop can refuse with exit 2, but a path heuristic
// produces false positives - a path being proposed rather than inspected reads
// exactly like a path being invented - and a wrong refusal strands finished
// work. Blocking is a decision to make after the report proves quiet on a real
// workload, not before.

// pathKeys are the fields whose value is a file the agent actually touched.
var pathKeys = map[string]struct{}{
	"file_path": {}, "path": {}, "notebook_path": {}, "file": {}, "filename": {},
}

// pathToken is what counts as a citation: a separator with a name either side,
// optionally an extension, optionally a trailing slash.
//
// The character class is Unicode rather than RE2's ASCII-only \w. The script
// this replaces decoded its input as UTF-8 precisely so non-ASCII paths
// survived to be compared, and \w would have dropped them at the last step.
var pathToken = regexp.MustCompile(`[\p{L}\p{N}_.@~-]*(?:[\\/][\p{L}\p{N}_.@-]+)+(?:\.[A-Za-z0-9]{1,8})?/?`)

// accessFailed marks a path the agent correctly reported as unreachable.
var accessFailed = regexp.MustCompile(`ACCESS-FAILED:\s*(\S+)`)

// maxReported keeps one bad turn from filling the transcript with its own
// diagnosis.
const maxReported = 15

// groundingReport names paths the final message cites but never opened, or ""
// when there is nothing to say.
//
// Every failure path is silence. A transcript in an unfamiliar shape, an
// unreadable file, or a payload from a host that sends neither field are all
// reasons to say nothing rather than to guess.
func groundingReport(body payload) string {
	if body.StopHookActive {
		return ""
	}
	transcript := body.TranscriptPath
	if transcript == "" {
		transcript = body.AgentTranscriptPath
	}
	if transcript == "" {
		return ""
	}

	observed, texts, whole := readTranscript(transcript)
	if len(texts) == 0 {
		return ""
	}

	seen := make(map[string]struct{}, len(observed))
	for path := range observed {
		if normalized := normalizePath(path); normalized != "" {
			seen[normalized] = struct{}{}
		}
	}

	unsupported := unsupportedCitations(texts[len(texts)-1], seen)
	if len(unsupported) == 0 {
		return ""
	}
	return groundingMessage(unsupported, whole)
}

// readTranscript walks the JSONL transcript for paths the agent touched and for
// the assistant text it produced.
func readTranscript(path string) (observed map[string]struct{}, texts []string, whole bool) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, false
	}
	defer func() { _ = file.Close() }()

	observed = map[string]struct{}{}

	scanner := bufio.NewScanner(file)
	// Transcript lines carry whole tool results and outgrow the default 64KB.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		walkTranscript(entry, observed, &texts)
	}
	// A scan that stopped on an error ends the loop exactly as a clean
	// end-of-file does. Without this the caller cannot tell a whole transcript
	// from most of one, and the report it builds is evidence a gate reads: a
	// grounding decision made on a partial file is the failure this control
	// plane exists to prevent, arriving through the back door.
	return observed, texts, scanner.Err() == nil
}

// walkTranscript collects from arbitrary nested JSON, because the transcript
// layout is a host's private shape and probing it beats assuming it.
func walkTranscript(node any, observed map[string]struct{}, texts *[]string) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			text, isString := child.(string)
			if _, isPath := pathKeys[key]; isPath && isString && strings.TrimSpace(text) != "" {
				observed[strings.TrimSpace(text)] = struct{}{}
				continue
			}
			// A "text" field is what the agent said, so it is the thing being
			// checked rather than evidence of anything opened.
			if key == "text" && isString {
				*texts = append(*texts, text)
				continue
			}
			walkTranscript(child, observed, texts)
		}
	case []any:
		for _, child := range value {
			walkTranscript(child, observed, texts)
		}
	case string:
		// Tool results list paths as plain prose, so any path-like token in one
		// counts as seen.
		for _, token := range pathToken.FindAllString(value, -1) {
			observed[token] = struct{}{}
		}
	}
}

// unsupportedCitations returns the paths this text cites without support.
func unsupportedCitations(text string, seen map[string]struct{}) []string {
	// A path already reported unreachable is correctly grounded. Only the path
	// attached to the marker is excused, not everything sharing its line.
	excused := map[string]struct{}{}
	for _, match := range accessFailed.FindAllStringSubmatch(text, -1) {
		excused[normalizePath(match[1])] = struct{}{}
	}

	var unsupported []string
	reported := map[string]struct{}{}
	for _, token := range pathToken.FindAllString(text, -1) {
		if !looksLikeFile(token) {
			continue
		}
		normalized := normalizePath(token)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		if _, ok := excused[normalized]; ok {
			continue
		}
		if overlapsSeen(normalized, seen) {
			continue
		}
		if _, already := reported[normalized]; already {
			continue
		}
		reported[normalized] = struct{}{}
		unsupported = append(unsupported, normalized)
	}
	return unsupported
}

// looksLikeFile keeps directory-ish and extensionless prose out of the report.
func looksLikeFile(token string) bool {
	if strings.HasSuffix(token, "/") {
		return true
	}
	last := token
	if index := strings.LastIndex(token, "/"); index >= 0 {
		last = token[index+1:]
	}
	return strings.Contains(last, ".")
}

// overlapsSeen accepts a citation that is a prefix or suffix of something
// opened, which is how a relative mention of an absolute path stays quiet.
func overlapsSeen(normalized string, seen map[string]struct{}) bool {
	for candidate := range seen {
		if strings.Contains(normalized, candidate) || strings.Contains(candidate, normalized) {
			return true
		}
	}
	return false
}

// normalizePath renders a citation and a tool argument the same way, so the two
// can be compared: forward slashes, no surrounding punctuation, no leading `./`.
func normalizePath(value string) string {
	cleaned := strings.ReplaceAll(value, `\`, "/")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, "`'\"()[],.")
	return strings.TrimPrefix(cleaned, "./")
}

func groundingMessage(unsupported []string, whole bool) string {
	shown := unsupported
	extra := 0
	if len(shown) > maxReported {
		extra = len(shown) - maxReported
		shown = shown[:maxReported]
	}

	var text strings.Builder
	text.WriteString("subagent-grounding-guard: the final message cites path(s) that appear in no " +
		"tool call or tool result in this subagent's transcript:")
	for _, path := range shown {
		text.WriteString("\n  - " + path)
	}
	if extra > 0 {
		text.WriteString("\n  - ... and " + strconv.Itoa(extra) + " more")
	}
	text.WriteString("\nVerify each one was actually opened. If a path could not be read, report it ")
	text.WriteString("as `ACCESS-FAILED: <path>` rather than describing its contents. Paths being ")
	text.WriteString("proposed rather than inspected are expected here and can be ignored.")

	// Said plainly, because a partial read makes this finding unreliable in the
	// direction that matters. `observed` is the set of paths the transcript
	// proved were opened; a truncated read means some proof went unread, so a
	// path can be listed above precisely because the evidence for it was in the
	// part that did not load. A reader who does not know that will chase a
	// citation that was fine.
	if !whole {
		text.WriteString("\n\nThe transcript could not be read to the end, so this list may name ")
		text.WriteString("paths whose evidence was in the part that did not load. Treat it as a ")
		text.WriteString("prompt to check rather than a finding.")
	}
	return text.String()
}
