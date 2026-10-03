package calc

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

type tokKind int

const (
	tokEOF tokKind = iota
	tokNum
	tokStr
	tokIdent
	tokOp
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func (t token) describe() string {
	switch t.kind {
	case tokEOF:
		return "end of expression"
	case tokStr:
		return fmt.Sprintf("string %q", t.text)
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

func lex(s string) ([]token, error) {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case isDigit(c) || (c == '.' && i+1 < len(s) && isDigit(s[i+1])):
			end, err := scanNumber(s, i)
			if err != nil {
				return nil, err
			}
			out = append(out, token{tokNum, s[i:end], i})
			i = end
		case isLetter(c):
			j := i
			for j < len(s) && (isLetter(s[j]) || isDigit(s[j]) || s[j] == '_') {
				j++
			}
			out = append(out, token{tokIdent, strings.ToLower(s[i:j]), i})
			i = j
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return nil, fmt.Errorf("position %d: a string is missing its closing quote", i)
			}
			out = append(out, token{tokStr, s[i+1 : i+1+j], i})
			i += j + 2
		case c == ',':
			// 1,000 inside sum(1,000) would silently be two arguments, 1 and 0.
			// A comma between a short run of digits and exactly three digits,
			// with no space, is a thousands separator or that mistake. Either
			// way the writer has to say which, by dropping the comma or adding
			// a space.
			if looksLikeThousands(s, i) {
				return nil, fmt.Errorf("position %d: a comma between digits is ambiguous; write 1000 instead of 1,000, or put a space after the comma to separate arguments", i)
			}
			out = append(out, token{tokOp, ",", i})
			i++
		case strings.IndexByte("+-*/^()%", c) >= 0:
			out = append(out, token{tokOp, string(c), i})
			i++
		default:
			return nil, fmt.Errorf("position %d: unexpected character %q", i, string(c))
		}
	}
	out = append(out, token{tokEOF, "", len(s)})
	return out, nil
}

// looksLikeThousands reports whether the comma at i sits between one to three
// digits and exactly three digits, as in 1,000 or 12,345.
func looksLikeThousands(s string, i int) bool {
	left := 0
	for j := i - 1; j >= 0 && isDigit(s[j]); j-- {
		left++
	}
	if left == 0 || left > 3 {
		return false
	}
	if start := i - left - 1; start >= 0 && s[start] == '.' {
		return false
	}
	right := 0
	for j := i + 1; j < len(s) && isDigit(s[j]); j++ {
		right++
	}
	return right == 3
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// scanNumber returns the end of the number starting at i: digits, an optional
// fraction, and an optional exponent.
func scanNumber(s string, i int) (int, error) {
	j, digits := i, 0
	for j < len(s) && isDigit(s[j]) {
		j, digits = j+1, digits+1
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isDigit(s[j]) {
			j, digits = j+1, digits+1
		}
	}
	if digits > maxLiteralDigits {
		return 0, fmt.Errorf("position %d: a number may have at most %d digits", i, maxLiteralDigits)
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			if k-j > 6 {
				return 0, fmt.Errorf("position %d: the exponent in a number is too large", i)
			}
			return k, nil
		}
	}
	if j < len(s) && isLetter(s[j]) {
		return 0, fmt.Errorf("position %d: %q is not a valid number; there are no unit suffixes, write the plain number", i, s[i:j+1])
	}
	return j, nil
}

type parser struct {
	tokens   []token
	pos      int
	depth    int
	deadline time.Time
}

// check fails an evaluation that has run past its time limit. The size bounds
// keep any one operation short, and this keeps a long chain of them from adding
// up: an input is not trusted to be cheap just because every step of it is.
func (p *parser) check() error {
	if time.Now().After(p.deadline) {
		return fmt.Errorf("the expression took longer than %s to evaluate; use smaller numbers or fewer operations", evalTimeLimit)
	}
	return nil
}

func (p *parser) peek() token { return p.tokens[p.pos] }

func (p *parser) next() token {
	t := p.tokens[p.pos]
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) isOp(text string) bool {
	t := p.peek()
	return t.kind == tokOp && t.text == text
}

func (p *parser) enter() error {
	p.depth++
	if p.depth > maxDepth {
		return fmt.Errorf("expression is nested more than %d levels deep", maxDepth)
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

// parseExpr is addition and subtraction, the loosest level.
func (p *parser) parseExpr() (value, error) {
	if err := p.enter(); err != nil {
		return value{}, err
	}
	defer p.leave()

	left, err := p.parseTerm()
	if err != nil {
		return value{}, err
	}
	for p.isOp("+") || p.isOp("-") {
		if err := p.check(); err != nil {
			return value{}, err
		}
		op := p.next().text
		right, err := p.parseTerm()
		if err != nil {
			return value{}, err
		}
		left, err = arith(op, left, right)
		if err != nil {
			return value{}, err
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (value, error) {
	left, err := p.parseUnary()
	if err != nil {
		return value{}, err
	}
	for p.isOp("*") || p.isOp("/") {
		if err := p.check(); err != nil {
			return value{}, err
		}
		op := p.next().text
		right, err := p.parseUnary()
		if err != nil {
			return value{}, err
		}
		left, err = arith(op, left, right)
		if err != nil {
			return value{}, err
		}
	}
	return left, nil
}

// parseUnary handles a leading minus or plus. It sits below ^ in binding
// strength on purpose, so -2^2 is -(2^2), which is -4, the convention a person
// reading a formula expects.
func (p *parser) parseUnary() (value, error) {
	if p.isOp("-") || p.isOp("+") {
		op := p.next().text
		if err := p.enter(); err != nil {
			return value{}, err
		}
		defer p.leave()
		v, err := p.parseUnary()
		if err != nil {
			return value{}, err
		}
		if err := needNumber(v, "unary "+op); err != nil {
			return value{}, err
		}
		if op == "-" {
			return number(new(big.Rat).Neg(v.num), v.exact), nil
		}
		return v, nil
	}
	return p.parsePow()
}

// parsePow is right-associative: 2^3^2 is 2^(3^2).
func (p *parser) parsePow() (value, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return value{}, err
	}
	if !p.isOp("^") {
		return base, nil
	}
	p.next()
	if err := p.check(); err != nil {
		return value{}, err
	}
	if err := p.enter(); err != nil {
		return value{}, err
	}
	defer p.leave()
	// The exponent may carry its own sign: 2^-3.
	exp, err := p.parseUnary()
	if err != nil {
		return value{}, err
	}
	return power(base, exp)
}

// parsePostfix is a primary followed by any number of percent signs.
func (p *parser) parsePostfix() (value, error) {
	v, err := p.parsePrimary()
	if err != nil {
		return value{}, err
	}
	for p.isOp("%") {
		p.next()
		if err := needNumber(v, "%"); err != nil {
			return value{}, err
		}
		v = number(new(big.Rat).Quo(v.num, big.NewRat(100, 1)), v.exact)
	}
	return v, nil
}

func (p *parser) parsePrimary() (value, error) {
	t := p.next()
	switch t.kind {
	case tokNum:
		r, ok := new(big.Rat).SetString(t.text)
		if !ok {
			return value{}, fmt.Errorf("position %d: %q is not a number", t.pos, t.text)
		}
		if err := tooBig(r); err != nil {
			return value{}, err
		}
		return number(r, true), nil
	case tokStr:
		return value{kind: "string", str: t.text, exact: true}, nil
	case tokIdent:
		if !p.isOp("(") {
			return value{}, fmt.Errorf("position %d: %q is not a function. Calls need parentheses, for example %s(1, 2). There are no named constants", t.pos, t.text, t.text)
		}
		return p.parseCall(t)
	case tokOp:
		if t.text == "(" {
			v, err := p.parseExpr()
			if err != nil {
				return value{}, err
			}
			if !p.isOp(")") {
				return value{}, fmt.Errorf("position %d: missing a closing parenthesis", t.pos)
			}
			p.next()
			return v, nil
		}
	}
	if t.kind == tokEOF {
		return value{}, fmt.Errorf("the expression ends where a value was expected")
	}
	return value{}, fmt.Errorf("position %d: unexpected %s", t.pos, t.describe())
}

func (p *parser) parseCall(name token) (value, error) {
	p.next() // the opening parenthesis
	var args []value
	if p.isOp(")") {
		p.next()
		return callFunc(name, args)
	}
	for {
		if len(args) >= maxArgs {
			return value{}, fmt.Errorf("%s has more than %d arguments", name.text, maxArgs)
		}
		if err := p.check(); err != nil {
			return value{}, err
		}
		arg, err := p.parseExpr()
		if err != nil {
			return value{}, err
		}
		args = append(args, arg)
		switch {
		case p.isOp(","):
			p.next()
		case p.isOp(")"):
			p.next()
			return callFunc(name, args)
		default:
			return value{}, fmt.Errorf("position %d: expected a comma or a closing parenthesis in the arguments of %s", p.peek().pos, name.text)
		}
	}
}
