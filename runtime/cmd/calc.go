package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/calc"
)

// calcCommand evaluates one expression exactly. It exists so that a number in
// an answer, a doc, or a plan comes from a program rather than from a model's
// mental arithmetic, and so a person can repeat the same calculation.
func calcCommand(args []string) error {
	flags := newFlagSet("calc")
	digits := flags.Int("digits", calc.DefaultDigits, "decimal places to print for a result that does not terminate")
	round := flags.Int("round", 0, "round the final result to this many decimal places")
	mode := flags.String("mode", calc.ModeHalfEven, "rounding mode for --round: "+strings.Join(calc.Modes(), ", "))
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := flags.Parse(args); err != nil {
		if strings.Contains(err.Error(), "not defined") {
			return fmt.Errorf("%w; an expression that starts with '-' needs a -- first: vibe-agent calc -- \"-2^2\"", err)
		}
		return err
	}
	if flags.NArg() == 0 {
		return fmt.Errorf("calc needs an expression, for example: vibe-agent calc \"(1250 - 1000) / 1000 * 100\"")
	}
	expr := strings.Join(flags.Args(), " ")

	opts := calc.Options{Digits: *digits, Mode: *mode}
	flagSet := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })
	if flagSet["round"] {
		opts.Round = round
	}
	if flagSet["mode"] && !flagSet["round"] {
		return fmt.Errorf("--mode only applies with --round; inside an expression use round(x, places, \"mode\")")
	}

	result, err := calc.Eval(expr, opts)
	if err != nil {
		return err
	}
	return printCalc(os.Stdout, expr, result, *asJSON)
}

// printCalc writes one result. The plain form leads with the figure a person
// copies; the rest says how much to trust it.
func printCalc(out io.Writer, expr string, result calc.Result, asJSON bool) error {
	if asJSON {
		body := map[string]any{
			"expr":   expr,
			"kind":   string(result.Kind),
			"result": result.Text,
			"exact":  result.Exact,
		}
		if result.Fraction != "" {
			body["fraction"] = result.Fraction
		}
		if result.Rounded {
			body["rounded"] = true
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode result: %w", err)
		}
		_, err = fmt.Fprintln(out, string(encoded))
		return err
	}

	fmt.Fprintf(out, "expr      %s\n", expr)
	fmt.Fprintf(out, "result    %s\n", result.Text)
	switch {
	case result.Rounded:
		fmt.Fprintln(out, "exact     no, rounded as asked")
	case result.Exact:
		fmt.Fprintln(out, "exact     yes")
	default:
		fmt.Fprintln(out, "exact     no, the value does not end; the figure above is rounded")
	}
	if result.Fraction != "" {
		fmt.Fprintf(out, "fraction  %s\n", result.Fraction)
	}
	return nil
}
