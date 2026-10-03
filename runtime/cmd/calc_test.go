package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/calc"
)

func TestPrintCalcPlainAndJSON(t *testing.T) {
	exact, err := calc.Eval("0.1+0.2", calc.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printCalc(&out, "0.1+0.2", exact, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"result    0.3", "exact     yes"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("plain output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "fraction") {
		t.Errorf("a terminating decimal printed a fraction:\n%s", out.String())
	}

	third, _ := calc.Eval("1/3", calc.Options{})
	out.Reset()
	if err := printCalc(&out, "1/3", third, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"result":"0.333333333333"`, `"exact":false`, `"fraction":"1/3"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("json output lacks %q: %s", want, out.String())
		}
	}
}

func TestCalcCommandFlags(t *testing.T) {
	if err := calcCommand([]string{"--round", "2", "1000/7"}); err != nil {
		t.Fatal(err)
	}
	if err := calcCommand([]string{"--mode", "half_up", "1"}); err == nil {
		t.Error("--mode without --round was accepted")
	}
	if err := calcCommand(nil); err == nil {
		t.Error("calc with no expression was accepted")
	}
	if err := calcCommand([]string{"1/0"}); err == nil {
		t.Error("division by zero was accepted")
	}
	if err := calcCommand([]string{"-2^2"}); err == nil || !strings.Contains(err.Error(), "--") {
		t.Errorf("a leading minus gave %v, want a hint about --", err)
	}
	if err := calcCommand([]string{"--", "-2^2"}); err != nil {
		t.Errorf("-- -2^2: %v", err)
	}
}
