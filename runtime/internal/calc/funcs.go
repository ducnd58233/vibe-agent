package calc

import (
	"fmt"
	"math/big"
)

// callFunc applies a function. Every function name is spelled out here, so the
// list a caller may use is the list in this switch and nothing else.
func callFunc(name token, args []value) (value, error) {
	fn := name.text
	switch fn {
	case "abs", "floor", "ceil", "trunc", "sqrt":
		if len(args) != 1 {
			return value{}, arityErr(fn, "exactly 1 argument", len(args))
		}
		return unary(fn, args[0])
	case "min", "max":
		return extremum(fn, args)
	case "sum", "avg":
		return aggregate(fn, args)
	case "mod":
		if len(args) != 2 {
			return value{}, arityErr(fn, "exactly 2 arguments", len(args))
		}
		return modulo(args[0], args[1])
	case "round":
		return roundCall(args)
	case "date":
		if len(args) != 1 || args[0].kind != kindString {
			return value{}, fmt.Errorf(`date needs one string, for example date("2026-10-03")`)
		}
		days, err := dateOf(args[0].str)
		if err != nil {
			return value{}, err
		}
		return number(days, true), nil
	case "todate":
		if len(args) != 1 {
			return value{}, arityErr(fn, "exactly 1 argument", len(args))
		}
		if err := needNumber(args[0], "todate"); err != nil {
			return value{}, err
		}
		s, err := dateString(args[0].num)
		if err != nil {
			return value{}, err
		}
		return value{kind: KindDate, str: s, exact: args[0].exact}, nil
	}
	return value{}, fmt.Errorf("position %d: unknown function %q. The functions are abs, min, max, sum, avg, round, floor, ceil, trunc, mod, sqrt, date, todate", name.pos, fn)
}

func arityErr(fn, want string, got int) error {
	return fmt.Errorf("%s takes %s, got %d", fn, want, got)
}

func unary(fn string, a value) (value, error) {
	if err := needNumber(a, fn); err != nil {
		return value{}, err
	}
	switch fn {
	case "abs":
		return number(new(big.Rat).Abs(a.num), a.exact), nil
	case "floor":
		r, _ := RoundRat(a.num, 0, ModeFloor)
		return number(r, a.exact), nil
	case "ceil":
		r, _ := RoundRat(a.num, 0, ModeCeil)
		return number(r, a.exact), nil
	case "trunc":
		r, _ := RoundRat(a.num, 0, ModeDown)
		return number(r, a.exact), nil
	default: // sqrt
		root, rootExact, err := nthRoot(a.num, 2)
		if err != nil {
			return value{}, err
		}
		return number(root, a.exact && rootExact), nil
	}
}

func extremum(fn string, args []value) (value, error) {
	if len(args) == 0 {
		return value{}, arityErr(fn, "at least 1 argument", 0)
	}
	best := args[0]
	for _, a := range args {
		if err := needNumber(a, fn); err != nil {
			return value{}, err
		}
		c := a.num.Cmp(best.num)
		if (fn == "min" && c < 0) || (fn == "max" && c > 0) {
			best = a
		}
	}
	exact := true
	for _, a := range args {
		exact = exact && a.exact
	}
	return number(new(big.Rat).Set(best.num), exact), nil
}

func aggregate(fn string, args []value) (value, error) {
	if len(args) == 0 {
		return value{}, arityErr(fn, "at least 1 argument", 0)
	}
	total := new(big.Rat)
	exact := true
	for _, a := range args {
		if err := needNumber(a, fn); err != nil {
			return value{}, err
		}
		total.Add(total, a.num)
		exact = exact && a.exact
	}
	if fn == "avg" {
		total.Quo(total, big.NewRat(int64(len(args)), 1))
	}
	if err := tooBig(total); err != nil {
		return value{}, err
	}
	return number(total, exact), nil
}

// modulo is a - b*floor(a/b): the result takes the sign of b, so mod(-1, 3) is
// 2. There is no % operator for it because % after a number means percent.
func modulo(a, b value) (value, error) {
	if err := needNumber(a, "mod"); err != nil {
		return value{}, err
	}
	if err := needNumber(b, "mod"); err != nil {
		return value{}, err
	}
	if b.num.Sign() == 0 {
		return value{}, fmt.Errorf("mod by zero")
	}
	q, _ := RoundRat(new(big.Rat).Quo(a.num, b.num), 0, ModeFloor)
	out := new(big.Rat).Sub(a.num, new(big.Rat).Mul(b.num, q))
	return number(out, a.exact && b.exact), nil
}

// roundCall is round(x, places) or round(x, places, "mode").
func roundCall(args []value) (value, error) {
	if len(args) != 2 && len(args) != 3 {
		return value{}, arityErr("round", "2 or 3 arguments: round(x, places) or round(x, places, \"mode\")", len(args))
	}
	if err := needNumber(args[0], "round"); err != nil {
		return value{}, err
	}
	if err := needNumber(args[1], "round"); err != nil {
		return value{}, err
	}
	if !args[1].num.IsInt() || !args[1].num.Num().IsInt64() {
		return value{}, fmt.Errorf("round needs a whole number of decimal places")
	}
	mode := ModeHalfEven
	if len(args) == 3 {
		if args[2].kind != kindString {
			return value{}, fmt.Errorf(`the rounding mode is a string, for example "half_up"`)
		}
		mode = args[2].str
	}
	r, err := RoundRat(args[0].num, int(args[1].num.Num().Int64()), mode)
	if err != nil {
		return value{}, err
	}
	// Rounding is exact: the rounded value is itself a terminating decimal.
	return number(r, true), nil
}
