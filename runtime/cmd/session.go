package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func sessionCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session needs a subcommand: list, show")
	}
	switch args[0] {
	case "list":
		return sessionList(args[1:])
	case "show":
		return sessionShow(args[1:])
	default:
		return fmt.Errorf("unknown session subcommand %q; try list, show", args[0])
	}
}

func sessionList(args []string) error {
	flags := newFlagSet("session list")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		fmt.Println(slug)
	}
	if hasAmbientSession(workspaceRoot) {
		fmt.Println("ambient")
	}
	return nil
}

func sessionShow(args []string) error {
	flags := newFlagSet("session show")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "run slug, or ambient for the workspace journal")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("session show needs --slug")
	}

	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	var logPath string
	switch *slug {
	case "ambient":
		logPath = session.AmbientLogPath(workspaceRoot)
	default:
		logPath = session.LogPath(workspaceRoot, *slug)
	}
	return writeSessionShow(os.Stdout, logPath)
}

func writeSessionShow(out io.Writer, logPath string) error {
	events, err := session.Replay(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := fmt.Fprintln(out, "No session log yet.")
			return err
		}
		return err
	}
	if len(events) == 0 {
		_, err := fmt.Fprintln(out, "No session log yet.")
		return err
	}
	for _, ev := range events {
		if _, err := fmt.Fprintln(out, formatSessionLine(ev)); err != nil {
			return err
		}
	}
	return nil
}

func formatSessionLine(ev session.Event) string {
	kind := session.Kind(ev)
	var body session.Payload
	if len(ev.Payload) > 0 {
		_ = json.Unmarshal(ev.Payload, &body)
	}
	typeLabel := string(ev.Type)
	if role := firstNonEmpty(ev.Role, body.Role); role != "" {
		typeLabel += "/" + role
	}
	line := fmt.Sprintf("#%d %s %s %s", ev.Sequence, typeLabel, ev.Source, kind)
	if detail := singleLine(firstNonEmpty(body.Body, body.Command, body.Event, body.Tool)); detail != "" {
		line += " " + detail
	}
	return line
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasAmbientSession(workspaceRoot string) bool {
	path := session.AmbientLogPath(workspaceRoot)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
