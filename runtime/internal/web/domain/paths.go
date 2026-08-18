package domain

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrOutsideWorkspace means a path escapes the workspace root.
var ErrOutsideWorkspace = errors.New("path outside workspace")

// ResolveWorkspacePath maps a workspace-relative path to an absolute path.
func ResolveWorkspacePath(workspaceRoot, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return filepath.Clean(workspaceRoot), nil
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrOutsideWorkspace
	}
	root := filepath.Clean(workspaceRoot)
	abs := filepath.Join(root, clean)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) {
		return "", ErrOutsideWorkspace
	}
	return abs, nil
}

// RelWorkspacePath returns slash-separated path relative to workspace root.
func RelWorkspacePath(workspaceRoot, abs string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideWorkspace
	}
	return filepath.ToSlash(rel), nil
}

// FileRow is one entry in the workspace file browser.
type FileRow struct {
	Name   string
	Path   string
	IsDir  bool
	Attach string
}

// FormatAttach inserts an @path reference for the composer.
func FormatAttach(relPath string) string {
	return "@" + strings.TrimPrefix(filepath.ToSlash(relPath), "/")
}
