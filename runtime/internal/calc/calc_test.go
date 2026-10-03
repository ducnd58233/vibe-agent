package calc

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

func ptr(n int) *int { return &n }

// Expected values below were computed independently with Python's decimal and
// fractions modules, not with this package, so a defect here cannot agree with
// itself.
func TestEvalValues(t *testing.T) {
	cases := []struct {
		expr  string
		text  string
		exact bool
	}{
		{"1+2*3", "7", true},
		{"(1250-1000)/1000*100", "25", true},
		// The classic float trap: 0.1+0.2 is 0.30000000000000004 in binary floats.
		{"0.1+0.2", "0.3", true},
		{"1/8", "0.125", true},
		{"1/3", "0.333333333333", false},
		{"10/4", "2.5", true},
		{"2^10", "1024", true},
		{"2^-2", "0.25", true},
		{"-2^2", "-4", true},
		{"(-2)^2", "4", true},
		{"2^3^2", "512", true},
		{"10%", "0.1", true},
		{"200*5%", "10", true},
		{"50%*50%", "0.25", true},
		{"1e3", "1000", true},
		{"1.5e-2", "0.015", true},
		{".5+.25", "0.75", true},
		{"1.05^10", "1.62889462677744140625", true},
		{"1000*(1+5%)^10", "1628.89462677744140625", true},
		{"1.1^3", "1.331", true},
		{"sqrt(16)", "4", true},
		{"sqrt(2)", "1.414213562373", false},
		{"2^0.5", "1.414213562373", false},
		{"4^0.5", "2", true},
		{"8^(1/3)", "2", true},
		{"27^(2/3)", "9", true},
		{"1.7^2.5", "3.768098990207", false},
		{"(150/100)^(1/3)-1", "0.144714242553", false},
		{"abs(-3)", "3", true},
		{"floor(-2.5)", "-3", true},
		{"ceil(-2.5)", "-2", true},
		{"trunc(-2.5)", "-2", true},
		{"mod(-1,3)", "2", true},
		{"mod(7,3)", "1", true},
		{"mod(5.5,2)", "1.5", true},
		{"sum(1,2,3)", "6", true},
		{"avg(1,2,3,4)", "2.5", true},
		{"min(3,1,2)", "1", true},
		{"max(3,1,2)", "3", true},
		{"round(2.5,0)", "2", true},
		{"round(3.5,0)", "4", true},
		{`round(2.5,0,"half_up")`, "3", true},
		{`round(-2.5,0,"half_up")`, "-3", true},
		{`round(1.005,2)`, "1", true},
		{`round(1.005,2,"half_up")`, "1.01", true},
		{"round(1234.5678,-2)", "1200", true},
		{"round(1/3,4)", "0.3333", true},
		{"1000*(1-0.999)*0+43.2", "43.2", true},
		{"30*24*60*(1-99.9%)", "43.2", true},
		{`date("2026-10-31")-date("2026-10-03")`, "28", true},
		{`todate(date("2026-10-03")+7)`, "2026-10-10", true},
		{`todate(date("2024-02-28")+1)`, "2024-02-29", true},
		{`todate(date("2023-02-28")+1)`, "2023-03-01", true},
		{"  1 +\t2 ", "3", true},
		{"SUM(1,2)", "3", true},
	}
	for _, tc := range cases {
		got, err := Eval(tc.expr, Options{})
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if got.Text != tc.text || got.Exact != tc.exact {
			t.Errorf("%s = %q (exact %v), want %q (exact %v)", tc.expr, got.Text, got.Exact, tc.text, tc.exact)
		}
	}
}

func TestCommaRule(t *testing.T) {
	ok := []string{"max(5,10)", "sum(1,22)", "round(2.5,0)", "mod(7,3)", "sum(1.5,200)", "sum(1, 200)", "max(1234,5678)"}
	for _, expr := range ok {
		if _, err := Eval(expr, Options{}); err != nil {
			t.Errorf("%s: %v", expr, err)
		}
	}
	bad := []string{"1,000", "12,345+1", "sum(1,000)", "max(5,100)", "1,000.5"}
	for _, expr := range bad {
		if _, err := Eval(expr, Options{}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("%s: want an ambiguity error, got %v", expr, err)
		}
	}
}

func TestEvalFractionIsExact(t *testing.T) {
	got, err := Eval("sqrt(16/9)", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fraction != "4/3" {
		t.Errorf("fraction = %q, want 4/3", got.Fraction)
	}
	got, _ = Eval("2/6", Options{})
	if got.Fraction != "1/3" {
		t.Errorf("2/6 fraction = %q, want 1/3", got.Fraction)
	}
}

func TestEvalDatesAreDates(t *testing.T) {
	got, err := Eval(`todate(date("2026-10-03")+7)`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindDate {
		t.Errorf("kind = %s, want date", got.Kind)
	}
	if got.Rat() != nil {
		t.Error("a date has no numeric value")
	}
}

func TestEvalFinalRounding(t *testing.T) {
	got, err := Eval("1000/7", Options{Round: ptr(2)})
	if err != nil {
		t.Fatal(err)
	}
	// 142.857142857... to two places, half-even, is 142.86.
	if got.Text != "142.86" || !got.Rounded {
		t.Errorf("got %+v", got)
	}
	got, err = Eval("2.5", Options{Round: ptr(0), Mode: ModeHalfUp})
	if err != nil || got.Text != "3" {
		t.Errorf("half_up 2.5 = %q, %v", got.Text, err)
	}
	if _, err := Eval(`todate(1)`, Options{Round: ptr(0)}); err == nil {
		t.Error("a date was rounded")
	}
	if _, err := Eval("1", Options{Round: ptr(0), Mode: "nearest"}); err == nil {
		t.Error("an unknown rounding mode was accepted")
	}
}

func TestEvalDigits(t *testing.T) {
	got, err := Eval("1/3", Options{Digits: 4})
	if err != nil || got.Text != "0.3333" {
		t.Errorf("1/3 at 4 digits = %q, %v", got.Text, err)
	}
	if _, err := Eval("1", Options{Digits: MaxDigits + 1}); err == nil {
		t.Error("digits over the cap were accepted")
	}
}

func TestEvalRefusals(t *testing.T) {
	cases := []struct {
		expr string
		want string // a fragment the message must contain
	}{
		{"1/0", "division by zero"},
		{"mod(1,0)", "mod by zero"},
		{"1,000+1", "ambiguous"},
		{"sum(1,000)", "ambiguous"},
		{"pi", "not a function"},
		{"3 4", "unexpected"},
		{"(1+2", "closing parenthesis"},
		{"1 +", "ends where a value"},
		{"abs()", "exactly 1"},
		{"abs(1,2)", "exactly 1"},
		{"0^0", "undefined"},
		{"0^-1", "undefined"},
		{"(-8)^(1/3)", "negative"},
		{"sqrt(-1)", "negative"},
		{"2^20000", "at most"},
		{"99999999999999999999^10000", "bits"},
		{"2^0.123457", "too precise"},
		{"5kg", "unit"},
		{"5 kg", "no units"},
		{`round(1,"a")`, "needs a number"},
		{`round(1.5,0,"nearest")`, "unknown rounding mode"},
		{`round(1.5,0.5)`, "whole number"},
		{`round(1,2,3)`, "string"},
		{`date("2026-02-30")`, "real date"},
		{`date(5)`, "one string"},
		{`todate(1.5)`, "whole number"},
		{`todate(99999999)`, "range"},
		{`todate(date("2026-01-01"))+1`, "date"},
		{`"abc"+1`, "string"},
		{`("")`, "not as a result"},
		{`"abc"`, "not as a result"},
		{`1 & 2`, "unexpected character"},
		{`"open`, "closing quote"},
		{"foo(1)", "unknown function"},
		{strings.Repeat("1", maxLiteralDigits+1), "digits"},
		{strings.Repeat("(", maxDepth+5) + "1" + strings.Repeat(")", maxDepth+5), "nested"},
		{strings.Repeat("1+", maxExprLen), "characters"},
	}
	for _, tc := range cases {
		_, err := Eval(tc.expr, Options{})
		if err == nil {
			t.Errorf("%.40q was accepted", tc.expr)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%.40q: error %q does not mention %q", tc.expr, err, tc.want)
		}
	}
}

// A request that asks for a great deal of work has to end quickly with a named
// limit, not run for minutes. This is the resilience property the bounds exist
// for.
func TestEvalHostileInputsReturnQuickly(t *testing.T) {
	inputs := []string{
		"9^9999^9999",
		"(1/3)^9999",
		"7^(1/9999)",
		"123456789^(1/9973)",
		"2^9999*2^9999*2^9999*2^9999*2^9999*2^9999*2^9999*2^9999",
		"sum(" + strings.Repeat("1,", 900) + "1)",
		"1e999999",
		"1e99999999999",
	}
	for _, in := range inputs {
		start := time.Now()
		_, _ = Eval(in, Options{})
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("%.40q took %s", in, elapsed)
		}
	}
}

// Differential test: integer expressions against direct big.Rat arithmetic.
func TestEvalAgreesWithRatArithmetic(t *testing.T) {
	ops := []string{"+", "-", "*", "/"}
	for a := -7; a <= 7; a++ {
		for b := -7; b <= 7; b++ {
			for c := 1; c <= 5; c++ {
				for _, o1 := range ops {
					for _, o2 := range ops {
						expr := "(" + itoa(a) + ")" + o1 + "(" + itoa(b) + ")" + o2 + "(" + itoa(c) + ")"
						want, ok := refEval(a, b, c, o1, o2)
						got, err := Eval(expr, Options{})
						if !ok {
							if err == nil {
								t.Fatalf("%s should have failed", expr)
							}
							continue
						}
						if err != nil {
							t.Fatalf("%s: %v", expr, err)
						}
						if got.Rat().Cmp(want) != 0 {
							t.Fatalf("%s = %s, want %s", expr, got.Rat().RatString(), want.RatString())
						}
					}
				}
			}
		}
	}
}

func itoa(n int) string { return big.NewInt(int64(n)).String() }

func refEval(a, b, c int, o1, o2 string) (*big.Rat, bool) {
	// Precedence: * and / bind tighter than + and -, left to right.
	x, y, z := big.NewRat(int64(a), 1), big.NewRat(int64(b), 1), big.NewRat(int64(c), 1)
	apply := func(op string, l, r *big.Rat) (*big.Rat, bool) {
		out := new(big.Rat)
		switch op {
		case "+":
			return out.Add(l, r), true
		case "-":
			return out.Sub(l, r), true
		case "*":
			return out.Mul(l, r), true
		}
		if r.Sign() == 0 {
			return nil, false
		}
		return out.Quo(l, r), true
	}
	high := func(op string) bool { return op == "*" || op == "/" }
	switch {
	case high(o2) && !high(o1):
		yz, ok := apply(o2, y, z)
		if !ok {
			return nil, false
		}
		return apply(o1, x, yz)
	default:
		xy, ok := apply(o1, x, y)
		if !ok {
			return nil, false
		}
		return apply(o2, xy, z)
	}
}

func TestRoundRatProperties(t *testing.T) {
	half := big.NewRat(1, 2)
	for num := -300; num <= 300; num++ {
		x := big.NewRat(int64(num), 8)
		for n := 0; n <= 2; n++ {
			unit := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil))
			for _, mode := range Modes() {
				r, err := RoundRat(x, n, mode)
				if err != nil {
					t.Fatal(err)
				}
				diff := new(big.Rat).Sub(r, x)
				absDiff := new(big.Rat).Abs(diff)
				bound := new(big.Rat).Mul(unit, half)
				switch mode {
				case ModeHalfEven, ModeHalfUp:
					if absDiff.Cmp(bound) > 0 {
						t.Fatalf("%s %s at %d: off by more than half a unit", x.RatString(), mode, n)
					}
				case ModeFloor:
					if diff.Sign() > 0 || absDiff.Cmp(unit) >= 0 {
						t.Fatalf("floor %s at %d = %s", x.RatString(), n, r.RatString())
					}
				case ModeCeil:
					if diff.Sign() < 0 || absDiff.Cmp(unit) >= 0 {
						t.Fatalf("ceil %s at %d = %s", x.RatString(), n, r.RatString())
					}
				case ModeDown:
					if absDiff.Cmp(unit) >= 0 || (x.Sign() != 0 && r.Sign() != 0 && x.Sign() != r.Sign()) {
						t.Fatalf("down %s at %d = %s", x.RatString(), n, r.RatString())
					}
				}
			}
		}
	}
}

func FuzzEval(f *testing.F) {
	for _, seed := range []string{"1+2", "2^0.5", "sum(1, 2)", `todate(date("2026-10-03")+1)`, "((((1))))", "1e5%", "round(1/3, 2)", "-2^-2"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		// Any input must return, not panic, and a success must print something.
		got, err := Eval(expr, Options{})
		if err == nil && got.Text == "" {
			t.Fatalf("%q succeeded with an empty result", expr)
		}
		if err != nil && strings.Contains(err.Error(), "internal error") {
			t.Fatalf("%q hit a defect: %v", expr, err)
		}
	})
}

func TestEvalStopsAtItsTimeLimit(t *testing.T) {
	saved := evalTimeLimit
	evalTimeLimit = time.Nanosecond
	defer func() { evalTimeLimit = saved }()

	_, err := Eval("1+2+3+4+5+6+7+8+9", Options{})
	if err == nil || !strings.Contains(err.Error(), "took longer") {
		t.Fatalf("want a time-limit error, got %v", err)
	}
}
