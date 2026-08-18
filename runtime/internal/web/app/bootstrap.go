// Package app is the web server composition root: loopback HTTP only.
package app

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

const defaultPort = 3080

// ListenHost is always loopback.
const ListenHost = "127.0.0.1"

// Config holds loopback server settings.
type Config struct {
	WorkspaceRoot string
	Port          int
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
		port = defaultPort
	}
	return fmt.Sprintf("%s:%d", ListenHost, port)
}

// Run starts the loopback server and blocks until it stops.
func Run(cfg Config) error {
	if cfg.Port <= 0 {
		cfg.Port = defaultPort
	}
	if err := ValidateListenHost(ListenHost); err != nil {
		return err
	}
	handler, err := NewHandlerWithPort(cfg.WorkspaceRoot, cfg.Port)
	if err != nil {
		return err
	}
	addr := Addr(cfg.Port)
	if err := persistence.WriteState(cfg.WorkspaceRoot, domain.State{
		URL:       "http://" + addr + "/",
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if _, err := fmt.Fprintf(os.Stdout, "vibe-agent web listening on http://%s/\n", addr); err != nil {
		return err
	}
	return srv.ListenAndServe()
}
