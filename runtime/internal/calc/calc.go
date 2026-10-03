// Package calc evaluates arithmetic exactly, so a number in a doc or an answer
// comes from a program instead of a model's mental arithmetic.
//
// Every decimal literal is a rational number, so the evaluator works on
// math/big Rat values and a result is exact unless the expression took a root
// or a fractional power. Those go through big.Float at a fixed precision and the
// result is flagged inexact, which is the one place a caller has to say how many
// digits it wants.
//
// The grammar is small on purpose. A format with one reading per input is what
// lets a weaker model write an expression and trust the answer:
//
//	number   123  1.5  .5  1e3  5%   (5% is 0.05; there is no modulo operator)
//	string   "2026-10-03"           (only as a function argument)
//	binary   + - * / ^              (^ is right-associative; -2^2 is -4)
//	call     abs min max sum avg round floor ceil trunc mod sqrt date todate
//
// Input is untrusted, so every size is bounded: expression length, literal
// digits, nesting depth, argument count, exponent size, and the bit length of
// every intermediate value. A request over a bound fails with the bound named.
package calc

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Limits bound what one evaluation may spend.
const (
	maxExprLen       = 4096
	maxLiteralDigits = 1000
	maxDepth         = 100
	maxArgs          = 1000
	maxExponent      = 10000
	maxBits          = 200000
	// maxRootIndex bounds the denominator of a fractional exponent. Newton's
	// method on a root of index q costs about q multiplications per step, so
	// an unbounded q is a way to spend minutes on one request.
	maxRootIndex = 10000
	// floatPrec is the working precision, in bits, for roots and fractional
	// powers: about 77 decimal digits.
	floatPrec = 256
	// MaxDigits caps how many decimal places a result may print.
	MaxDigits = 50
	// DefaultDigits is how many decimal places a non-terminating result prints.
	DefaultDigits = 12
)

// Kind says what a value is.
type Kind string

// The value kinds. Only todate produces a date; every other result is a number.
const (
	KindNumber Kind = "number"
	KindDate   Kind = "date"
)

// Result is the answer to one expression.
type Result struct {
	Kind Kind
	// Text is the number as a decimal, or the date as YYYY-MM-DD.
	Text string
	// Exact reports whether Text is the exact value. It is false for a root, a
	// fractional power, or a number that does not end in decimal digits (1/3).
	Exact bool
	// Fraction is the exact value as p/q when it is not a whole number.
	Fraction string
	// Rounded reports that Options.Round was applied to the final value.
	Rounded bool

	value *big.Rat
}

// Rat returns a copy of the numeric value, or nil for a date.
func (r Result) Rat() *big.Rat {
	if r.value == nil {
		return nil
	}
	return new(big.Rat).Set(r.value)
}

// Options controls how a result is finished.
type Options struct {
	// Digits is how many decimal places a non-terminating result prints.
	// Zero means DefaultDigits.
	Digits int
	// Round, when set, rounds the final value to that many decimal places.
	Round *int
	// Mode is the rounding mode for Round. Empty means half_even.
	Mode string
}

type value struct {
	kind  Kind
	num   *big.Rat
	str   string
	exact bool
}

func number(r *big.Rat, exact bool) value { return value{kind: KindNumber, num: r, exact: exact} }

// Eval evaluates one expression.
func Eval(expr string, opts Options) (res Result, err error) {
	// The input is untrusted and this runs inside a hook on every call, so a
	// defect must come back as an error naming itself rather than take the
	// process down. The fuzz test fails on this message, so it is not a way to
	// hide one.
	defer func() {
		if r := recover(); r != nil {
			res, err = Result{}, fmt.Errorf("internal error evaluating %.60q: %v", expr, r)
		}
	}()
	if len(expr) > maxExprLen {
		return Result{}, fmt.Errorf("expression is %d characters; the limit is %d", len(expr), maxExprLen)
	}
	if opts.Digits < 0 || opts.Digits > MaxDigits {
		return Result{}, fmt.Errorf("digits must be 0 to %d", MaxDigits)
	}
	digits := opts.Digits
	if digits == 0 {
		digits = DefaultDigits
	}

	tokens, err := lex(expr)
	if err != nil {
		return Result{}, err
	}
	p := &parser{tokens: tokens}
	v, err := p.parseExpr()
	if err != nil {
		return Result{}, err
	}
	if extra := p.peek(); extra.kind != tokEOF {
		if extra.kind == tokIdent {
			return Result{}, fmt.Errorf("position %d: unexpected %s. There are no units or named constants, write the plain number", extra.pos, extra.describe())
		}
		return Result{}, fmt.Errorf("position %d: unexpected %s after the end of the expression", extra.pos, extra.describe())
	}

	if v.kind == kindString {
		return Result{}, fmt.Errorf("a string is only valid inside date(\"...\") or as a rounding mode, not as a result")
	}
	if v.kind == KindDate {
		if opts.Round != nil {
			return Result{}, fmt.Errorf("a date cannot be rounded")
		}
		return Result{Kind: KindDate, Text: v.str, Exact: true}, nil
	}

	rounded := false
	if opts.Round != nil {
		mode := opts.Mode
		if mode == "" {
			mode = ModeHalfEven
		}
		r, err := RoundRat(v.num, *opts.Round, mode)
		if err != nil {
			return Result{}, err
		}
		// Rounding gives an exact decimal value, and the caller asked for it.
		v = number(r, true)
		rounded = true
	}
	return finish(v, digits, rounded), nil
}

func finish(v value, digits int, rounded bool) Result {
	var text string
	terminates := false
	if v.exact {
		text, terminates = formatRat(v.num, digits)
	} else {
		// An inexact value carries about 77 good digits and a long binary
		// tail past them; print only what the caller asked for.
		rounded, _ := RoundRat(v.num, digits, ModeHalfEven)
		text = ratDecimal(rounded, digits)
	}
	res := Result{
		Kind:    KindNumber,
		Text:    text,
		Exact:   v.exact && terminates,
		Rounded: rounded,
		value:   v.num,
	}
	// A fraction is shown only when it says something the decimal cannot: an
	// exact value that does not end, like 1/3. A terminating decimal needs no
	// fraction, and an inexact value has no short exact fraction to show.
	if v.exact && !terminates {
		res.Fraction = v.num.Num().String() + "/" + v.num.Denom().String()
	}
	return res
}

// formatRat prints r as a decimal. When r has a terminating expansion it is
// printed in full and terminates is true; otherwise it is rounded half-even to
// digits places and terminates is false.
func formatRat(r *big.Rat, digits int) (text string, terminates bool) {
	places, ok := terminatingPlaces(r.Denom())
	if ok {
		return ratDecimal(r, places), true
	}
	rounded, _ := RoundRat(r, digits, ModeHalfEven)
	return ratDecimal(rounded, digits), false
}

// terminatingPlaces returns how many decimal places an exact expansion of
// 1/denom needs, and false when denom has a prime factor other than 2 or 5.
func terminatingPlaces(denom *big.Int) (int, bool) {
	d := new(big.Int).Set(denom)
	two, five := big.NewInt(2), big.NewInt(5)
	zero := new(big.Int)
	var twos, fives int
	rem := new(big.Int)
	for {
		q, m := new(big.Int).QuoRem(d, two, rem)
		if m.Cmp(zero) != 0 {
			break
		}
		d, twos = q, twos+1
	}
	for {
		q, m := new(big.Int).QuoRem(d, five, rem)
		if m.Cmp(zero) != 0 {
			break
		}
		d, fives = q, fives+1
	}
	if d.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	return max(twos, fives), true
}

// ratDecimal prints r with exactly places decimals, then trims trailing zeros
// and a dangling point, so 2.50 prints as 2.5 and 3.00 as 3.
func ratDecimal(r *big.Rat, places int) string {
	s := r.FloatString(places)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func tooBig(r *big.Rat) error {
	if r.Num().BitLen() > maxBits || r.Denom().BitLen() > maxBits {
		return fmt.Errorf("an intermediate result grew past %d bits; reduce the exponents or the number of digits", maxBits)
	}
	return nil
}

// dateOf parses a strict YYYY-MM-DD date into days since 1970-01-01.
func dateOf(s string) (*big.Rat, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("date(%q): want a real date written YYYY-MM-DD", s)
	}
	return new(big.Rat).SetInt64(t.Unix() / 86400), nil
}

// dateString renders days since 1970-01-01.
func dateString(days *big.Rat) (string, error) {
	if !days.IsInt() {
		return "", fmt.Errorf("todate needs a whole number of days, got %s", days.FloatString(3))
	}
	n := days.Num()
	// 3,000,000 days is about 8,200 years either side of 1970.
	if n.CmpAbs(big.NewInt(3_000_000)) > 0 {
		return "", fmt.Errorf("todate: %s days is outside the supported range", n.String())
	}
	return time.Unix(n.Int64()*86400, 0).UTC().Format("2006-01-02"), nil
}
