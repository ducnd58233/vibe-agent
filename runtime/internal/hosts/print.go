package hosts

import "strings"

// PrintOptions are composer-only CLI flags. They never change Catalog IDs.
type PrintOptions struct {
	Model string
	Mode  string
}

// AcceptsModel reports whether this host documents a --model flag.
func AcceptsModel(host Host) bool {
	switch host.Binary {
	case "claude", "cursor-agent":
		return true
	default:
		return false
	}
}

// PrintArgv returns argv after the binary for a print-mode spawn.
func PrintArgv(host Host, opts PrintOptions) []string {
	parts := strings.Fields(host.EvalCommand)
	if len(parts) < 2 {
		return nil
	}
	args := append([]string{}, parts[1:]...)
	if host.Binary == "claude" {
		args = applyClaudePrintStream(args)
	}
	if host.Binary == "cursor-agent" {
		args = applyCursorMode(args, opts.Mode)
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" || strings.HasPrefix(model, "-") || !AcceptsModel(host) {
		return args
	}
	return append(args, "--model", model)
}

func applyClaudePrintStream(args []string) []string {
	stripped := dropFlagAndValue(args, "--output-format")
	stripped = dropBareFlag(stripped, "--verbose")
	stripped = dropBareFlag(stripped, "--include-hook-events")
	flags := []string{"--output-format", "stream-json", "--verbose", "--include-hook-events"}
	for i, arg := range stripped {
		if arg == "--print" || arg == "-p" {
			out := append([]string{}, stripped[:i+1]...)
			out = append(out, flags...)
			return append(out, stripped[i+1:]...)
		}
	}
	return append(flags, stripped...)
}

func dropBareFlag(args []string, flag string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == flag {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func applyCursorMode(args []string, mode string) []string {
	stripped := dropFlagAndValue(args, "--mode")
	if strings.EqualFold(strings.TrimSpace(mode), "agent") {
		return stripped
	}
	return insertAfterPrint(stripped, "--mode", "ask")
}

func dropFlagAndValue(args []string, flag string) []string {
	out := make([]string, 0, len(args))
	skipValue := false
	prefix := flag + "="
	for _, arg := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if arg == flag {
			skipValue = true
			continue
		}
		if strings.HasPrefix(arg, prefix) {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func insertAfterPrint(args []string, flag, value string) []string {
	for i, arg := range args {
		if arg == "--print" || arg == "-p" {
			rest := append([]string{flag, value}, args[i+1:]...)
			return append(args[:i+1], rest...)
		}
	}
	return append([]string{flag, value}, args...)
}
