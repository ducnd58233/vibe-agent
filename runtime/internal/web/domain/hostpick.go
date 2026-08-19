package domain

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// PickKind is the OS dialog the operator asked for.
type PickKind string

const (
	PickFile   PickKind = "file"
	PickFolder PickKind = "folder"
)

var (
	// ErrPickCancelled means the operator dismissed the OS dialog.
	ErrPickCancelled = errors.New("cancelled")
	// ErrPickUnavailable means this OS has no working dialog helper.
	ErrPickUnavailable = errors.New("picker unavailable")
	// ErrBadPickKind means kind was not file or folder.
	ErrBadPickKind = errors.New("bad kind")
)

// HostPicker shows a host file or folder dialog and returns the chosen path.
type HostPicker interface {
	Pick(ctx context.Context, kind PickKind) (string, error)
}

// ParsePickKind maps a query value to file or folder.
func ParsePickKind(raw string) (PickKind, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(PickFile):
		return PickFile, nil
	case string(PickFolder):
		return PickFolder, nil
	default:
		return "", ErrBadPickKind
	}
}

// CleanPickPath accepts an absolute file or directory path and rejects relative
// values and any ".." segment before the path is used.
func CleanPickPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "..") {
		return "", ErrBadHostPath
	}
	slash := filepath.ToSlash(path)
	if strings.HasPrefix(slash, "//") {
		return "", ErrBadHostPath
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(clean) {
		return "", ErrBadHostPath
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", ErrBadHostPath
	}
	return abs, nil
}
