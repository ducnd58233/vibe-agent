package domain

import "sort"

type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

const (
	RuleTodoComment      = "todo_comment"
	RuleEmptyFunction    = "empty_function"
	RuleIgnoredResult    = "ignored_result"
	RuleDebugPrint       = "debug_print"
	RulePanicPlaceholder = "panic_placeholder"
	RuleLongFunction     = "long_function"
	RuleLongFile         = "long_file"
	RuleDuplicateLine    = "duplicate_line"
	RuleParseError       = "parse_error"
	RuleScanError        = "scan_error"
)

const (
	StatusClean      = "clean"
	StatusSuspicious = "suspicious"
	StatusInflated   = "inflated"
	StatusCritical   = "critical"
)

const (
	CleanThreshold      = 30
	SuspiciousThreshold = 50
	InflatedThreshold   = 70
	MaxScore            = 100
	MinimumScoredLines  = 200
	LinesPerKLOC        = 1000
	ScoreDensityDivisor = 10
)

const (
	InfoWeight   = 1
	LowWeight    = 4
	MediumWeight = 8
	HighWeight   = 16
)

type Finding struct {
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type ScanSummary struct {
	FilesScanned       int            `json:"files_scanned"`
	LinesScanned       int            `json:"lines_scanned"`
	TreeSitterParsed   int            `json:"tree_sitter_parsed"`
	TreeSitterFailures int            `json:"tree_sitter_failures"`
	Languages          map[string]int `json:"languages"`
	Parser             string         `json:"parser"`
	Scoring            string         `json:"scoring"`
}

type ScanResult struct {
	Findings []Finding
	Summary  ScanSummary
}

type AdapterStatus string

const (
	AdapterPassed  AdapterStatus = "passed"
	AdapterFailed  AdapterStatus = "failed"
	AdapterSkipped AdapterStatus = "skipped"
)

type AdapterResult struct {
	Name     string        `json:"name"`
	Status   AdapterStatus `json:"status"`
	Command  []string      `json:"command,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	ExitCode int           `json:"exit_code,omitempty"`
}

type Report struct {
	Target   string          `json:"target"`
	Score    int             `json:"score"`
	Status   string          `json:"status"`
	Summary  ScanSummary     `json:"summary"`
	Findings []Finding       `json:"findings"`
	Adapters []AdapterResult `json:"adapters"`
}

func Score(findings []Finding, linesScanned int) int {
	total := 0
	for _, finding := range findings {
		switch finding.Severity {
		case SeverityHigh:
			total += HighWeight
		case SeverityMedium:
			total += MediumWeight
		case SeverityLow:
			total += LowWeight
		default:
			total += InfoWeight
		}
	}
	effectiveLines := linesScanned
	if effectiveLines < MinimumScoredLines {
		effectiveLines = MinimumScoredLines
	}
	score := total * LinesPerKLOC / effectiveLines / ScoreDensityDivisor
	if score > MaxScore {
		return MaxScore
	}
	return score
}

func MergeSummary(summaries []ScanSummary) ScanSummary {
	merged := ScanSummary{
		Languages: map[string]int{},
		Parser:    "tree-sitter plus text",
		Scoring:   "weighted finding density per KLOC, capped at 100",
	}
	for _, summary := range summaries {
		merged.FilesScanned += summary.FilesScanned
		merged.LinesScanned += summary.LinesScanned
		merged.TreeSitterParsed += summary.TreeSitterParsed
		merged.TreeSitterFailures += summary.TreeSitterFailures
		for language, count := range summary.Languages {
			merged.Languages[language] += count
		}
		if summary.Parser != "" {
			merged.Parser = summary.Parser
		}
		if summary.Scoring != "" {
			merged.Scoring = summary.Scoring
		}
	}
	return merged
}

func Status(score int) string {
	switch {
	case score >= InflatedThreshold:
		return StatusCritical
	case score >= SuspiciousThreshold:
		return StatusInflated
	case score >= CleanThreshold:
		return StatusSuspicious
	default:
		return StatusClean
	}
}

func SortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Rule < right.Rule
	})
}

func SortAdapters(adapters []AdapterResult) {
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Name < adapters[j].Name
	})
}
