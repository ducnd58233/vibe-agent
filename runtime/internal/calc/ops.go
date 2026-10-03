package calc

import (
	"fmt"
	"math/big"
)

const kindString Kind = "string"

// Rounding modes. half_even is the default because it is the one that does not
// push a long column of figures in one direction; half_up is what most people
// do by hand, so it is the mode to name when a source says "rounded".
const (
	ModeHalfEven = "half_even"
	ModeHalfUp   = "half_up"
	ModeUp       = "up"
	ModeDown     = "down"
	ModeFloor    = "floor"
	ModeCeil     = "ceil"
)

// Modes lists every rounding mode RoundRat accepts.
func Modes() []string {
	return []string{ModeHalfEven, ModeHalfUp, ModeUp, ModeDown, ModeFloor, ModeCeil}
}

func needNumber(v value, what string) error {
	switch v.kind {
	case KindNumber:
		return nil
	case KindDate:
		return fmt.Errorf("%s needs a number, not a date; dates are days since 1970-01-01, so use date(\"...\") in arithmetic and todate(...) once at the end", what)
	default:
		return fmt.Errorf("%s needs a number, not a string; a string is only valid inside date(\"...\") or as a rounding mode", what)
	}
}

func arith(op string, a, b value) (value, error) {
	if err := needNumber(a, op); err != nil {
		return value{}, err
	}
	if err := needNumber(b, op); err != nil {
		return value{}, err
	}
	out := new(big.Rat)
	switch op {
	case "+":
		out.Add(a.num, b.num)
	case "-":
		out.Sub(a.num, b.num)
	case "*":
		out.Mul(a.num, b.num)
	case "/":
		if b.num.Sign() == 0 {
			return value{}, fmt.Errorf("division by zero")
		}
		out.Quo(a.num, b.num)
	}
	if err := tooBig(out); err != nil {
		return value{}, err
	}
	return number(out, a.exact && b.exact), nil
}

// power is a^b. A whole-number exponent is exact. A fractional exponent p/q is
// the q-th root of a^p: exact when a^p is a perfect q-th power, and otherwise
// computed to floatPrec bits and flagged inexact.
func power(base, exp value) (value, error) {
	if err := needNumber(base, "^"); err != nil {
		return value{}, err
	}
	if err := needNumber(exp, "^"); err != nil {
		return value{}, err
	}
	exact := base.exact && exp.exact
	x, e := base.num, exp.num

	if x.Sign() == 0 && e.Sign() <= 0 {
		return value{}, fmt.Errorf("0 to the power %s is undefined", ratDecimal(e, 6))
	}
	if x.Sign() == 0 {
		return number(new(big.Rat), exact), nil
	}

	if e.IsInt() {
		r, err := intPower(x, e.Num())
		if err != nil {
			return value{}, err
		}
		return number(r, exact), nil
	}

	q := e.Denom()
	if !q.IsInt64() || q.Int64() > maxRootIndex {
		return value{}, fmt.Errorf("the exponent %s is too precise: a fractional exponent may have a denominator up to %d. Round the exponent first", e.RatString(), maxRootIndex)
	}
	if x.Sign() < 0 {
		return value{}, fmt.Errorf("a negative number to a fractional power has no real value")
	}
	// x^(p/q) = root_q(x^p). x^p stays exact, then one root is taken.
	xp, err := intPower(x, e.Num())
	if err != nil {
		return value{}, err
	}
	root, rootExact, err := nthRoot(xp, int(q.Int64()))
	if err != nil {
		return value{}, err
	}
	return number(root, exact && rootExact), nil
}

func intPower(x *big.Rat, e *big.Int) (*big.Rat, error) {
	if !e.IsInt64() || e.Int64() > maxExponent || e.Int64() < -maxExponent {
		return nil, fmt.Errorf("an exponent may be at most %d in size", maxExponent)
	}
	n := e.Int64()
	abs := n
	if abs < 0 {
		abs = -abs
	}
	bits := x.Num().BitLen()
	if d := x.Denom().BitLen(); d > bits {
		bits = d
	}
	// Refuse before computing: the size of x^n is known from the size of x.
	if bits > 1 && int64(bits)*abs > maxBits {
		return nil, fmt.Errorf("the result of raising to the power %d would pass %d bits; reduce the exponent", n, maxBits)
	}
	num := new(big.Int).Exp(x.Num(), big.NewInt(abs), nil)
	den := new(big.Int).Exp(x.Denom(), big.NewInt(abs), nil)
	var out *big.Rat
	if n < 0 {
		out = new(big.Rat).SetFrac(den, num)
	} else {
		out = new(big.Rat).SetFrac(num, den)
	}
	return out, tooBig(out)
}

// nthRoot returns the q-th root of r, which must not be negative. The second
// result is true when the root is exactly rational.
func nthRoot(r *big.Rat, q int) (*big.Rat, bool, error) {
	if r.Sign() < 0 {
		return nil, false, fmt.Errorf("a root of a negative number has no real value")
	}
	if r.Sign() == 0 || q == 1 {
		return new(big.Rat).Set(r), true, nil
	}
	if exact, ok := exactRoot(r, q); ok {
		return exact, true, nil
	}

	x := new(big.Float).SetPrec(floatPrec).SetRat(r)
	// Start above the root so Newton's iteration only ever falls toward it.
	// x < 2^e, so the root is below 2^ceil(e/q).
	e := x.MantExp(nil)
	y := new(big.Float).SetPrec(floatPrec).SetMantExp(big.NewFloat(1), (e+q-1)/q)
	qf := new(big.Float).SetPrec(floatPrec).SetInt64(int64(q))
	qm1 := new(big.Float).SetPrec(floatPrec).SetInt64(int64(q - 1))
	for range 200000 {
		// y' = ((q-1) y + x / y^(q-1)) / q
		next := new(big.Float).SetPrec(floatPrec).Quo(x, floatPow(y, q-1))
		next.Add(next, new(big.Float).SetPrec(floatPrec).Mul(qm1, y))
		next.Quo(next, qf)
		if next.Cmp(y) >= 0 {
			break
		}
		y = next
	}
	out, _ := y.Rat(nil)
	return out, false, tooBig(out)
}

func floatPow(y *big.Float, n int) *big.Float {
	result := new(big.Float).SetPrec(floatPrec).SetInt64(1)
	base := new(big.Float).SetPrec(floatPrec).Set(y)
	for ; n > 0; n >>= 1 {
		if n&1 == 1 {
			result.Mul(result, base)
		}
		base.Mul(base, base)
	}
	return result
}

// exactRoot reports the q-th root of r when both its numerator and denominator
// are perfect q-th powers, so sqrt(16/9) is exactly 4/3 rather than a decimal.
func exactRoot(r *big.Rat, q int) (*big.Rat, bool) {
	// Newton on very large integers is slow, and a number that large is not a
	// perfect power anyone typed in.
	if r.Num().BitLen() > 20000 || r.Denom().BitLen() > 20000 {
		return nil, false
	}
	n, ok := intRoot(r.Num(), q)
	if !ok {
		return nil, false
	}
	d, ok := intRoot(r.Denom(), q)
	if !ok {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}

// intRoot returns the integer q-th root of n and true when n is a perfect power.
func intRoot(n *big.Int, q int) (*big.Int, bool) {
	if n.Sign() == 0 || n.Cmp(big.NewInt(1)) == 0 {
		return new(big.Int).Set(n), true
	}
	qb := big.NewInt(int64(q))
	qm1 := big.NewInt(int64(q - 1))
	// Start at a power of two above the root, then fall.
	x := new(big.Int).Lsh(big.NewInt(1), uint((n.BitLen()+q-1)/q))
	for {
		xq1 := new(big.Int).Exp(x, qm1, nil)
		next := new(big.Int).Mul(qm1, x)
		next.Add(next, new(big.Int).Quo(n, xq1))
		next.Quo(next, qb)
		if next.Cmp(x) >= 0 {
			break
		}
		x = next
	}
	return x, new(big.Int).Exp(x, qb, nil).Cmp(n) == 0
}

// RoundRat rounds x to n decimal places (n may be negative: -2 rounds to
// hundreds) using one of Modes().
func RoundRat(x *big.Rat, n int, mode string) (*big.Rat, error) {
	if n > 1000 || n < -1000 {
		return nil, fmt.Errorf("round to between -1000 and 1000 decimal places")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(n))), nil)
	y := new(big.Rat).Set(x)
	if n >= 0 {
		y.Mul(y, new(big.Rat).SetInt(scale))
	} else {
		y.Quo(y, new(big.Rat).SetInt(scale))
	}

	q, err := roundToInt(y, mode)
	if err != nil {
		return nil, err
	}
	out := new(big.Rat).SetInt(q)
	if n >= 0 {
		out.Quo(out, new(big.Rat).SetInt(scale))
	} else {
		out.Mul(out, new(big.Rat).SetInt(scale))
	}
	return out, nil
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func floorInt(y *big.Rat) *big.Int {
	// Euclidean division by a positive denominator is floor division.
	return new(big.Int).Div(y.Num(), y.Denom())
}

func roundToInt(y *big.Rat, mode string) (*big.Int, error) {
	switch mode {
	case ModeFloor:
		return floorInt(y), nil
	case ModeCeil:
		neg := new(big.Rat).Neg(y)
		return new(big.Int).Neg(floorInt(neg)), nil
	case ModeDown:
		return new(big.Int).Quo(y.Num(), y.Denom()), nil
	case ModeUp, ModeHalfUp, ModeHalfEven:
	default:
		return nil, fmt.Errorf("unknown rounding mode %q; use one of %v", mode, Modes())
	}

	abs := new(big.Rat).Abs(y)
	f := floorInt(abs)
	frac := new(big.Rat).Sub(abs, new(big.Rat).SetInt(f))
	var q *big.Int
	switch mode {
	case ModeUp:
		q = f
		if frac.Sign() > 0 {
			q = new(big.Int).Add(f, big.NewInt(1))
		}
	default:
		switch frac.Cmp(big.NewRat(1, 2)) {
		case -1:
			q = f
		case 1:
			q = new(big.Int).Add(f, big.NewInt(1))
		default:
			if mode == ModeHalfUp || f.Bit(0) == 1 {
				q = new(big.Int).Add(f, big.NewInt(1))
			} else {
				q = f
			}
		}
	}
	if y.Sign() < 0 {
		q = new(big.Int).Neg(q)
	}
	return q, nil
}
