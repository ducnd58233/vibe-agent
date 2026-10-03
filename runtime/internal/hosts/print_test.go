package hosts

import (
	"slices"
	"testing"
)

// Every test here walks the catalog and selects hosts by what their row says,
// so a host gains coverage by gaining the property, not by being named.

func hostsWhere(want func(Host) bool) []Host {
	var out []Host
	for _, host := range catalog {
		if want(host) {
			out = append(out, host)
		}
	}
	return out
}

func TestPrintArgvNeverMutatesTheCatalog(t *testing.T) {
	for _, host := range catalog {
		orig := host.EvalCommand
		_ = PrintArgv(host, PrintOptions{Model: "some-model", Mode: agentMode})
		again, _ := EvalHost(host.ID)
		if host.EvalCommand != orig || again.EvalCommand != orig {
			t.Errorf("%s: EvalCommand mutated", host.ID)
		}
	}
}

func TestPrintFlagsArePresentOnceAfterThePrintFlag(t *testing.T) {
	for _, host := range hostsWhere(func(h Host) bool { return len(h.PrintFlags) > 0 }) {
		argv := PrintArgv(host, PrintOptions{})
		for _, flag := range host.PrintFlags {
			count := 0
			for _, arg := range argv {
				if arg == flag.Name {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s: %s appears %d times in %v", host.ID, flag.Name, count, argv)
			}
			if flag.Value != "" && !hasFlagValue(argv, flag.Name, flag.Value) {
				t.Errorf("%s: %s is not %s in %v", host.ID, flag.Name, flag.Value, argv)
			}
		}
	}
}

func TestAskModeIsTheDefaultAndAgentModeDropsIt(t *testing.T) {
	for _, host := range hostsWhere(func(h Host) bool { return h.AskMode != nil }) {
		ask := *host.AskMode
		if argv := PrintArgv(host, PrintOptions{}); !hasFlagValue(argv, ask.Name, ask.Value) {
			t.Errorf("%s: the read-only default is missing: %v", host.ID, argv)
		}
		if argv := PrintArgv(host, PrintOptions{Mode: agentMode}); slices.Contains(argv, ask.Name) {
			t.Errorf("%s: agent mode kept %s: %v", host.ID, ask.Name, argv)
		}
	}
}

func TestAModelIsPassedOnlyToHostsThatTakeOne(t *testing.T) {
	for _, host := range catalog {
		argv := PrintArgv(host, PrintOptions{Model: "chosen-model"})
		passed := len(argv) >= 2 && slices.Equal(argv[len(argv)-2:], []string{"--model", "chosen-model"})
		if AcceptsModel(host) != passed {
			t.Errorf("%s: accepts a model = %t, but argv %v", host.ID, AcceptsModel(host), argv)
		}
	}
}

func TestAFlagLikeModelIsNeverPassed(t *testing.T) {
	for _, host := range hostsWhere(AcceptsModel) {
		if argv := PrintArgv(host, PrintOptions{Model: "--dangerous"}); slices.Contains(argv, "--dangerous") {
			t.Errorf("%s: a model value starting with - reached argv: %v", host.ID, argv)
		}
	}
}

func TestModelSuggestionsAreTheHostsOwnList(t *testing.T) {
	for _, host := range catalog {
		if !slices.Equal(ModelSuggestions(host), host.Models) {
			t.Errorf("%s: suggestions %v, catalog %v", host.ID, ModelSuggestions(host), host.Models)
		}
	}
}

func hasFlagValue(args []string, flag, value string) bool {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
		if arg == flag+"="+value {
			return true
		}
	}
	return false
}
