package calc

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Issue is one calculation in a document that does not hold.
type Issue struct {
	Line    int // 1-indexed, in the markdown file
	Message string
}

func (i Issue) String() string { return fmt.Sprintf("line %d: %s", i.Line, i.Message) }

// A logged calculation is one line inside a fenced block whose info string is
// exactly "calc":
//
//	```calc
//	(1250 - 1000) / 1000 * 100 => 25
//	1000 / 7 => ~142.86
//	2.5 * 2 => 5 [half_up]
//	```
//
// "=> X" means the value is exactly X. "=> ~X" means the value rounds to X at
// the number of decimals X shows, half-even unless a [mode] follows. The two
// forms are separate on purpose: writing 25 for 25.4 must fail, and writing
// ~25 says the figure is rounded.
var (
	expectedNumber = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	expectedDate   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	modeSuffix     = regexp.MustCompile(`\s*\[([a-z_]+)\]\s*$`)
)

// CheckMarkdown recomputes every calc block in a markdown document. It returns
// how many blocks and how many calculation lines it found, and one Issue per
// line that does not hold. A document with no calc block has nothing to check
// and no issues.
func CheckMarkdown(raw []byte) (blocks, lines int, issues []Issue) {
	var open fence
	for i, line := range strings.Split(string(raw), "\n") {
		if open.char != 0 {
			if open.closedBy(line) {
				open = fence{}
				continue
			}
			trimmed := strings.TrimSpace(line)
			if !open.isCalc || trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			lines++
			if msg := checkLine(trimmed); msg != "" {
				issues = append(issues, Issue{Line: i + 1, Message: msg})
			}
			continue
		}
		if f, ok := openingFence(line); ok {
			open = f
			if f.isCalc {
				blocks++
			}
		}
	}
	return blocks, lines, issues
}

// fence is a fenced code block that is currently open. It follows CommonMark:
// a block opens with three or more backticks or tildes and closes only on a
// fence of the same character at least as long, so a four-backtick block can
// show a three-backtick block as text without that inner fence ending it.
type fence struct {
	char   byte
	length int
	isCalc bool
}

// openingFence reports whether line opens a fenced block.
func openingFence(line string) (fence, bool) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 || len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return fence{}, false
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return fence{}, false
	}
	info := strings.TrimSpace(t[n:])
	// A backtick fence's info string may not contain a backtick; that is an
	// inline code span that happens to start a line.
	if t[0] == '`' && strings.Contains(info, "`") {
		return fence{}, false
	}
	return fence{char: t[0], length: n, isCalc: info == "calc"}, true
}

// closedBy reports whether line closes the open fence.
func (f fence) closedBy(line string) bool {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return false
	}
	n := 0
	for n < len(t) && t[n] == f.char {
		n++
	}
	return n >= f.length && strings.TrimSpace(t[n:]) == ""
}

func checkLine(line string) string {
	at := strings.LastIndex(line, "=>")
	if at < 0 {
		return `no "=>": write the expression, then "=>", then the value it must have`
	}
	expr := strings.TrimSpace(line[:at])
	expected := strings.TrimSpace(line[at+2:])
	if expr == "" || expected == "" {
		return `both sides of "=>" are needed`
	}

	mode := ModeHalfEven
	if m := modeSuffix.FindStringSubmatch(expected); m != nil {
		mode = m[1]
		expected = strings.TrimSpace(expected[:len(expected)-len(m[0])])
	}
	approx := strings.HasPrefix(expected, "~")
	expected = strings.TrimSpace(strings.TrimPrefix(expected, "~"))

	if expectedDate.MatchString(expected) {
		got, err := Eval(expr, Options{})
		if err != nil {
			return fmt.Sprintf("%s: %v", expr, err)
		}
		if got.Kind != KindDate || got.Text != expected {
			return fmt.Sprintf("%s => %s is wrong: the result is %s", expr, expected, got.Text)
		}
		return ""
	}
	if !expectedNumber.MatchString(expected) {
		return fmt.Sprintf("%q is not a plain decimal or a YYYY-MM-DD date; write the number without commas, units, or a percent sign", expected)
	}

	want, _ := new(big.Rat).SetString(expected)
	places := 0
	if dot := strings.IndexByte(expected, '.'); dot >= 0 {
		places = len(expected) - dot - 1
	}

	got, err := Eval(expr, Options{Digits: min(places+6, MaxDigits)})
	if err != nil {
		return fmt.Sprintf("%s: %v", expr, err)
	}
	if got.Kind != KindNumber {
		return fmt.Sprintf("%s is a date, not a number", expr)
	}

	if !approx {
		if got.Rat().Cmp(want) != 0 {
			return fmt.Sprintf("%s => %s is wrong: the result is %s%s", expr, expected, got.Text, hintApprox(got))
		}
		return ""
	}
	rounded, err := RoundRat(got.Rat(), places, mode)
	if err != nil {
		return fmt.Sprintf("%s: %v", expr, err)
	}
	if rounded.Cmp(want) != 0 {
		return fmt.Sprintf("%s => ~%s is wrong: the result is %s, which rounds to %s at %d decimals (%s)",
			expr, expected, got.Text, ratDecimal(rounded, places), places, mode)
	}
	return ""
}

func hintApprox(got Result) string {
	if got.Exact {
		return ""
	}
	return "; it is not a terminating decimal, so write ~ and the rounded value"
}
