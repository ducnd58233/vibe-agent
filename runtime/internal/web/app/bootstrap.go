// Package app is the web server composition root: loopback HTTP only.
package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	httpservermiddleware "github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver/middleware"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

const (
	// DefaultPort is the loopback web UI listen port.
	DefaultPort = 3080

	// ListenHost is always loopback.
	ListenHost = "127.0.0.1"
)

// Config holds loopback server settings.
type Config struct {
	WorkspaceRoot   string
	ToolkitRoot     string
	Port            int
	ExtraWorkspaces []string
	Logger          observability.Logger
}

// ValidateListenHost refuses non-loopback binds.
func ValidateListenHost(host string) error {
	switch host {
	case "127.0.0.1", "localhost":
		return nil
	case "0.0.0.0", "::", "[::]":
		return fmt.Errorf("refusing to bind %q: loopback only", host)
	default:
		ip := net.ParseIP(host)
		if ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("refusing to bind %q: loopback only", host)
	}
}

// Addr formats the listen address for a port.
func Addr(port int) string {
	if port <= 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("%s:%d", ListenHost, port)
}

// Run starts the loopback server and blocks until it stops.
func Run(cfg Config) error {
	if cfg.Port <= 0 {
		cfg.Port = DefaultPort
	}
	if err := ValidateListenHost(ListenHost); err != nil {
		return err
	}
	reg, err := loadRegistry(filepath.Clean(cfg.WorkspaceRoot), cfg.ExtraWorkspaces)
	if err != nil {
		return err
	}
	handler, err := mountHTTP(newHTTPDeps(reg, cfg.ToolkitRoot, cfg.Port))
	if err != nil {
		return err
	}
	handler = httpservermiddleware.StandardStack(handler, cfg.Logger)

	addr := Addr(cfg.Port)
	if err := persistence.WriteState(cfg.WorkspaceRoot, domain.State{
		URL:       "http://" + addr + "/",
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	defer func() { _ = persistence.RemoveState(cfg.WorkspaceRoot) }()

	if _, err := fmt.Fprintf(os.Stdout, "vibe-agent web listening on http://%s/ (Ctrl+C to stop)\n", addr); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		_, _ = fmt.Fprintf(os.Stderr, "\nshutting down (%s)...\n", sig)
		cancel()
	}()

	if err := httpserver.Serve(ctx, addr, handler, cfg.Logger); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(os.Stdout, "stopped")
	return nil
}
