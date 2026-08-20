package redact

import "regexp"

// literalSpecs are high-confidence credential shapes with no honest reason to
// appear in source. Harness gate blocks writes; Text replaces them in output.
//
// Each pattern is written so it does not match its own source text (see gate.go).
var literalSpecs = []string{
	`-----BEGIN [A-Z ]*PRIVATE KEY-----`,
	`gh[pousr]_[A-Za-z0-9]{16,}`,
	`\bsk-[A-Za-z0-9_-]{16,}`,
	`xox[baprs]-[A-Za-z0-9-]{10,}`,
	`\bAKIA[0-9A-Z]{16}\b`,
	`(?i)aws_secret_access_key\s*[:=]\s*\S{16,}`,
}

// formatRedactSpecs are format-specific credential shapes used for output
// redaction and memory rejection. Gate stays narrow (literal only).
var formatRedactSpecs = []string{
	`sk-ant-[A-Za-z0-9_-]{20,}`,
	`github_pat_[A-Za-z0-9_]{22,}`,
	`AIza[0-9A-Za-z\-_]{35}`,
	`sk_live_[0-9a-zA-Z]{20,}`,
	`sk_test_[0-9a-zA-Z]{20,}`,
	`rk_live_[0-9a-zA-Z]{20,}`,
	`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`,
	`(?i)-----END [A-Z ]*PRIVATE KEY-----`,
	`https://hooks\.slack\.com/services/[A-Za-z0-9/]+`,
	`https://discord(?:app)?\.com/api/webhooks/[0-9]+/[A-Za-z0-9_-]+`,
}

// contextualSpecs catch keyword-adjacent secrets. Broad by design for redaction
// and memory rejection: a false positive costs one scrubbed line, a false
// negative leaks a secret.
var contextualSpecs = []string{
	`(?i)\b(api[_-]?key|secret|password|passwd|token|credential|private[_-]?key)\b\s*[:=]\s*\S{8,}`,
	`(?i)\bbearer\s+[a-z0-9._-]{16,}`,
	`(?i)\baws_(access|secret)_[a-z_]*key\b`,
}

var (
	literalPatterns      = mustCompile(literalSpecs)
	formatRedactPatterns = mustCompile(formatRedactSpecs)
	contextualPatterns   = mustCompile(contextualSpecs)
	textPatterns         = joinPatterns(literalPatterns, formatRedactPatterns, contextualPatterns)
)

func mustCompile(specs []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(specs))
	for i, spec := range specs {
		out[i] = regexp.MustCompile(spec)
	}
	return out
}

func joinPatterns(groups ...[]*regexp.Regexp) []*regexp.Regexp {
	var n int
	for _, g := range groups {
		n += len(g)
	}
	out := make([]*regexp.Regexp, 0, n)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// LiteralPatterns returns high-confidence shapes used by the harness write gate.
func LiteralPatterns() []*regexp.Regexp {
	return literalPatterns
}

// CompilePatterns builds regexes from pattern specs. Invalid specs panic at init.
func CompilePatterns(specs []string) []*regexp.Regexp {
	return mustCompile(specs)
}

// MemoryRejectPatterns returns all non-gitleaks regex groups used to reject
// credential-shaped memory candidates.
func MemoryRejectPatterns() []*regexp.Regexp {
	return joinPatterns(literalPatterns, formatRedactPatterns, contextualPatterns)
}
