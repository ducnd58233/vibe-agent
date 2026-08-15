package harness

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// The test guard is the deterministic half of the core-logic rule in the
// test-driven-development skill. The skill states the rule in prose, which
// leaves the model to decide whether it followed it; this reports what the text
// alone settles, and nothing else.
//
// It checks four things only. The skill's bans are semantic and most cannot be
// decided from syntax: reading a file inside a test is discovery when the claim
// is "the config exists" and behaviour when it is "saving writes a manifest",
// and both call the same function. A guard that flagged the API would be wrong
// more often than right. Everything else stays with the reviewer.
//
// It cannot be expressed as line patterns, which is why it is a builtin rather
// than an entry in guards-default.yaml: the question is "how many assertions
// does this block contain", and a block has to be found before it can be
// counted.

// A testDialect is how one language spells a test.
//
// A table rather than a switch, so covering another stack is a matter of adding
// an entry. Each dialect needs a way to find a test, a way to end it, a way to
// recognise an assertion inside it, and a way to spot a deliberate skip.
type testDialect struct {
	// test captures the test's name in group 1.
	test *regexp.Regexp
	// blockEnd ends a block at the next declaration. Nil means blocks run to
	// the start of the following test, which is how nested `it(` calls nest.
	blockEnd *regexp.Regexp
	assert   *regexp.Regexp
	exempt   *regexp.Regexp
	// literals describes the comment and string syntax, so a fixture quoting
	// source code cannot be read as source code.
	literals literalSyntax
	// isTestFile reports whether a base name is a test in this language.
	isTestFile func(base string) bool
}

var (
	cStyleLiterals = literalSyntax{
		comments: []delimiter{{"//", "\n"}, {"/*", "*/"}},
		strings:  []stringDelimiter{{"`", "`", false}, {`"`, `"`, true}, {"'", "'", true}},
	}
	jsLiterals = literalSyntax{
		comments: []delimiter{{"//", "\n"}, {"/*", "*/"}},
		strings:  []stringDelimiter{{"`", "`", true}, {`"`, `"`, true}, {"'", "'", true}},
	}
	pythonLiterals = literalSyntax{
		comments: []delimiter{{"#", "\n"}},
		strings: []stringDelimiter{
			{`"""`, `"""`, false}, {"'''", "'''", false},
			{`"`, `"`, true}, {"'", "'", true},
		},
	}
)

// testDialects is keyed by the language name enry reports.
var testDialects = map[string]testDialect{
	"Go": {
		test:     regexp.MustCompile(`(?m)^func\s+(Test\w+)\s*\(`),
		blockEnd: regexp.MustCompile(`(?m)^func\s`),
		assert:   regexp.MustCompile(`\bt\.(?:Error|Errorf|Fatal|Fatalf)\b|\b(?:require|assert)\.\w+\(`),
		exempt:   regexp.MustCompile(`\bt\.Skip(?:f|Now)?\b`),
		literals: cStyleLiterals,
		isTestFile: func(base string) bool {
			return strings.HasSuffix(base, "_test.go")
		},
	},
	"Python": {
		test:     regexp.MustCompile(`(?m)^[ \t]*def\s+(test_\w+)\s*\(`),
		blockEnd: regexp.MustCompile(`(?m)^[ \t]*(?:def|class)\s`),
		assert:   regexp.MustCompile(`(?m)^\s*assert\b|\bself\.assert\w*\(|\bpytest\.raises\b`),
		exempt:   regexp.MustCompile(`@pytest\.mark\.skip|\bpytest\.skip\(`),
		literals: pythonLiterals,
		isTestFile: func(base string) bool {
			return strings.HasSuffix(base, "_test.py") ||
				(strings.HasPrefix(base, "test_") && strings.EqualFold(filepath.Ext(base), ".py"))
		},
	},
}

// jsDialect is shared by every language enry names for the JS family.
var jsDialect = testDialect{
	test:     regexp.MustCompile("(?m)^[ \t]*(?:it|test)(?:\\.\\w+)?\\s*\\(\\s*['\"`]([^'\"`]+)['\"`]"),
	assert:   regexp.MustCompile(`\bexpect\s*\(|\bassert\b|\.should\b`),
	exempt:   regexp.MustCompile(`(?m)^[ \t]*(?:it|test)\.(?:skip|todo)\b`),
	literals: jsLiterals,
	isTestFile: func(base string) bool {
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
	},
}

func init() {
	for _, name := range []string{"TypeScript", "TSX", "JavaScript", "JSX"} {
		testDialects[name] = jsDialect
	}
}

// The four things the text can settle.
var (
	envOnly   = regexp.MustCompile(`\bos\.Getenv\s*\(|\bos\.environ\b|\bprocess\.env\b|\bSystem\.getenv\s*\(`)
	mockOnly  = regexp.MustCompile(`toHaveBeenCalled(?:Times|With)?\s*\(|\.assert_called(?:_once)?(?:_with)?\s*\(|\bAssertExpectations\s*\(|\bverify\s*\(`)
	existence = regexp.MustCompile(`\bos\.Stat\s*\(|\bexistsSync\s*\(|\bos\.path\.exists\s*\(|\.exists\s*\(\s*\)|\.is_file\s*\(\s*\)|\.is_dir\s*\(\s*\)|\bfs\.access\s*\(`)
	// Names promising the test is about the environment rather than the product.
	discoveryName = regexp.MustCompile(`(?i)exists|is[_ ]?present|has[_ ]?file|folder|directory|layout|structure|scaffold`)
	healthName    = regexp.MustCompile(`(?i)is[_ ]?up|is[_ ]?running|is[_ ]?alive|started|container|health|ping|connects?|connection|reachable`)
)

// markerContext is how many lines above a test the opt-out may sit on, so a
// marker written as a comment over the declaration still counts.
const markerContext = 4

// inspectTestBlocks reports tests that cannot fail when behaviour changes.
func inspectTestBlocks(file subject) []string {
	dialect, known := testDialects[file.Language]
	if !known || !dialect.isTestFile(filepath.Base(file.Path)) {
		return nil
	}

	masked := maskLiterals(dialect.literals, file.Text)
	marker := guardMarker(guardCoreLogicTest)

	var findings []string
	for _, found := range testBlocks(dialect, masked, file.Text) {
		if blockAllowed(file.Text, marker, found) {
			continue
		}
		if problem := inspectBlock(dialect, found); problem != "" {
			findings = append(findings, fmt.Sprintf("L%d %s: %s", found.line, found.name, problem))
		}
	}
	return findings
}

// A block is one test and its body.
//
// Two copies of the body, because the two questions want different text. text
// is masked and answers "does this check anything", where a literal is data.
// source is verbatim and answers "did someone opt out", where the marker lives
// in a comment that masking blanks.
type block struct {
	name   string
	text   string
	source string
	line   int
}

// testBlocks finds the test blocks in the masked text, carrying the verbatim
// body along.
//
// Boundaries come from the masked copy so a declaration quoted inside a fixture
// cannot end a block early. Masking preserves byte length and newlines, so the
// same offsets slice the original and reported line numbers match the file.
func testBlocks(dialect testDialect, masked, source string) []block {
	matches := dialect.test.FindAllStringSubmatchIndex(masked, -1)

	// Matched once over the whole text rather than by re-searching a slice.
	// Slicing would put a fresh start-of-string in front of a (?m)^ anchor and
	// end every block at its own first line.
	var ends [][]int
	if dialect.blockEnd != nil {
		ends = dialect.blockEnd.FindAllStringIndex(masked, -1)
	}

	blocks := make([]block, 0, len(matches))
	for index, match := range matches {
		start := match[1]
		end := len(masked)
		if dialect.blockEnd == nil {
			// Blocks that end where the next one begins, as nested `it(` do.
			if index+1 < len(matches) {
				end = matches[index+1][0]
			}
		} else {
			for _, candidate := range ends {
				if candidate[0] >= start {
					end = candidate[0]
					break
				}
			}
		}
		blocks = append(blocks, block{
			name:   masked[match[2]:match[3]],
			text:   masked[start:end],
			source: source[start:end],
			line:   strings.Count(masked[:match[0]], "\n") + 1,
		})
	}
	return blocks
}

// blockAllowed reports whether someone acknowledged this test.
//
// The marker may sit inside the block or on the lines just above it, because a
// reader explaining an exception writes the comment over the declaration.
func blockAllowed(text, marker string, target block) bool {
	if strings.Contains(target.source, marker) {
		return true
	}
	lines := splitLines(text)
	start := max(0, target.line-markerContext)
	stop := min(target.line, len(lines))
	for _, line := range lines[start:stop] {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

// inspectBlock returns what is wrong with a test, or "" when the text does not
// settle it.
func inspectBlock(dialect testDialect, target block) string {
	if dialect.exempt.MatchString(target.text) {
		return ""
	}
	switch len(dialect.assert.FindAllString(target.text, -1)) {
	case 0:
		return "asserts nothing, so it cannot fail when behaviour changes. " +
			"Assert the outcome, or delete the test."
	case 1:
		return inspectLoneAssertion(target)
	default:
		return ""
	}
}

// inspectLoneAssertion reads the single assertion a test does make.
func inspectLoneAssertion(target block) string {
	switch {
	case envOnly.MatchString(target.text):
		return "asserts only an environment variable, which tests the machine rather " +
			"than the code. Move it to CI or test setup."
	case mockOnly.MatchString(target.text):
		return "asserts only that a dependency was called, which passes for any " +
			"pass-through wrapper. Assert the outcome the code produced."
	case existence.MatchString(target.text) && discoveryName.MatchString(target.name):
		return "reads as path discovery: the name promises existence and the only " +
			"assertion is an existence check. If a write is the behaviour under test, " +
			"say so in the name; otherwise move this to CI."
	case healthName.MatchString(target.name):
		return "reads as a service or container health check, which tests the harness " +
			"rather than the application. Start dependencies in setup and assert a " +
			"domain outcome here."
	}
	return ""
}
