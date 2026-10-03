package hosts

import "strings"

// PrintOptions are composer-only CLI flags. They never change catalog IDs.
type PrintOptions struct {
	Model string
	Mode  string
}

// agentMode is the composer's request to let a spawn act, not only answer.
const agentMode = "agent"

// AcceptsModel reports whether this host documents a --model flag.
func AcceptsModel(host Host) bool { return len(host.Models) > 0 }

// ModelSuggestions are the host's documented --model values for the composer.
func ModelSuggestions(host Host) []string { return host.Models }

// PrintArgv returns argv after the binary for a print-mode spawn: the host's
// eval command, its print flags, its read-only mode unless agent mode was asked
// for, and a model when the host takes one.
func PrintArgv(host Host, opts PrintOptions) []string {
	parts := strings.Fields(host.EvalCommand)
	if len(parts) < 2 {
		return nil
	}
	args := append([]string{}, parts[1:]...)
	if len(host.PrintFlags) > 0 {
		args = setAfterPrint(args, host.PrintFlags...)
	}
	if host.AskMode != nil {
		args = dropFlag(args, *host.AskMode)
		if !strings.EqualFold(strings.TrimSpace(opts.Mode), agentMode) {
			args = setAfterPrint(args, *host.AskMode)
		}
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" || strings.HasPrefix(model, "-") || !AcceptsModel(host) {
		return args
	}
	return append(args, "--model", model)
}

// setAfterPrint removes any existing occurrence of each flag and inserts them,
// in order, right after the print flag (or first, when there is none).
func setAfterPrint(args []string, flags ...Flag) []string {
	for _, flag := range flags {
		args = dropFlag(args, flag)
	}
	var inserted []string
	for _, flag := range flags {
		inserted = append(inserted, flag.Name)
		if flag.Value != "" {
			inserted = append(inserted, flag.Value)
		}
	}
	for i, arg := range args {
		if arg == "--print" || arg == "-p" {
			out := append([]string{}, args[:i+1]...)
			out = append(out, inserted...)
			return append(out, args[i+1:]...)
		}
	}
	return append(inserted, args...)
}

// dropFlag removes a flag, with its value when it takes one, in either the
// "--flag value" or "--flag=value" spelling.
func dropFlag(args []string, flag Flag) []string {
	out := make([]string, 0, len(args))
	skipValue := false
	for _, arg := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if arg == flag.Name {
			skipValue = flag.Value != ""
			continue
		}
		if strings.HasPrefix(arg, flag.Name+"=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}
