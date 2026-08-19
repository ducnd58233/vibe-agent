package main

import (
	"context"
	"runtime"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/app"
)

func webCommand(args []string) error {
	flags := newFlagSet("web")
	paths := addRootFlags(flags)
	port := flags.Int("port", 3080, "listen port (loopback only)")
	open := flags.Bool("open", false, "open the URL in a browser")
	workspaces := flags.String("workspaces", "", "comma-separated extra workspace roots")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	var extra []string
	if strings.TrimSpace(*workspaces) != "" {
		for _, part := range strings.Split(*workspaces, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				extra = append(extra, part)
			}
		}
	}
	if *open {
		_ = openBrowser(context.Background(), app.Addr(*port))
	}
	log, closer, err := openServiceLogger("web")
	if err != nil {
		return err
	}
	defer closeLogger(closer)
	return app.Run(app.Config{
		WorkspaceRoot:   workspaceRoot,
		ToolkitRoot:     toolkitRoot,
		Port:            *port,
		ExtraWorkspaces: extra,
		Logger:          log,
	})
}

func openBrowser(ctx context.Context, addr string) error {
	url := "http://" + addr + "/"
	var (
		name string
		args []string
	)
	switch runtime.GOOS {
	case "windows":
		name = "cmd"
		args = []string{"/c", "start", "", url}
	case "darwin":
		name = "open"
		args = []string{url}
	default:
		name = "xdg-open"
		args = []string{url}
	}
	cmd, err := safexec.CommandContext(ctx, name, args...)
	if err != nil {
		return err
	}
	return cmd.Start()
}
