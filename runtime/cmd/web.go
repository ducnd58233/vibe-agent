package main

import (
	"context"
	"runtime"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/web"
)

func webCommand(args []string) error {
	flags := newFlagSet("web")
	paths := addRootFlags(flags)
	port := flags.Int("port", 3080, "listen port (loopback only)")
	open := flags.Bool("open", false, "open the URL in a browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	if *open {
		_ = openBrowser(context.Background(), web.Addr(*port))
	}
	return web.Run(web.Config{
		WorkspaceRoot: workspaceRoot,
		Port:          *port,
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
