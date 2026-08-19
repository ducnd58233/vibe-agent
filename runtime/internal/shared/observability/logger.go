package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// Logger is the logging port. *slog.Logger satisfies it.
type Logger interface {
	Info(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	Warn(msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	Error(msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	Debug(msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
}

// Options configures dual-sink logging: tinted console and JSON file.
type Options struct {
	Service string
	Level   string
	Stdout  io.Writer
	Dir     string
}

// NewLogger writes tinted console output and a JSON log file.
func NewLogger(opt Options) (*slog.Logger, io.Closer, error) {
	service := strings.TrimSpace(opt.Service)
	if !validService(service) {
		return nil, nil, fmt.Errorf("observability: invalid service name %q", service)
	}

	level := new(slog.LevelVar)
	switch strings.ToLower(strings.TrimSpace(opt.Level)) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	stdout := opt.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	dir := strings.TrimSpace(opt.Dir)
	if dir == "" {
		var err error
		dir, err = ResolveLogDir()
		if err != nil {
			return nil, nil, err
		}
	}

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, nil, fmt.Errorf("observability: mkdir %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("observability: open log dir %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile(service+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, nil, fmt.Errorf("observability: open %s.log: %w", service, err)
	}

	fileHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	consoleHandler := tint.NewTextHandler(colorableWriter(stdout), &tint.Options{
		Level:      level,
		TimeFormat: time.Kitchen,
		NoColor:    false,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == "stack" {
				return slog.Attr{}
			}
			return attr
		},
	})

	h := slog.NewMultiHandler(consoleHandler, fileHandler)
	return slog.New(h).With(slog.String("name", service)), f, nil
}

// Discard drops all records.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func validService(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func colorableWriter(w io.Writer) io.Writer {
	if f, ok := w.(*os.File); ok {
		return colorable.NewColorable(f)
	}
	return w
}
