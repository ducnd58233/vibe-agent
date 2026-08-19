package domain

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrBadHostPath means a browse path is relative, escaped, or not a directory.
var ErrBadHostPath = errors.New("bad path")

// ResolveHostDir maps a user-supplied absolute directory to a cleaned path.
// Relative values and any ".." segment are rejected before the filesystem is consulted.
func ResolveHostDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || strings.Contains(dir, "..") {
		return "", ErrBadHostPath
	}
	slash := filepath.ToSlash(dir)
	if strings.HasPrefix(slash, "//") {
		return "", ErrBadHostPath
	}
	clean := filepath.Clean(filepath.FromSlash(dir))
	if !filepath.IsAbs(clean) {
		return "", ErrBadHostPath
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", ErrBadHostPath
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", ErrBadHostPath
	}
	return abs, nil
}

// HostRoots is the first browse view: the user home, plus drives on Windows.
func HostRoots() []FileRow {
	var rows []FileRow
	if home, err := os.UserHomeDir(); err == nil {
		rows = append(rows, FileRow{
			Name:  "Home",
			Path:  filepath.ToSlash(home),
			IsDir: true,
		})
	}
	if runtime.GOOS == "windows" {
		for c := 'A'; c <= 'Z'; c++ {
			p := string(c) + `:\`
			info, err := os.Stat(p)
			if err != nil || !info.IsDir() {
				continue
			}
			rows = append(rows, FileRow{
				Name:  string(c) + ":",
				Path:  filepath.ToSlash(p),
				IsDir: true,
			})
		}
		return rows
	}
	rows = append(rows, FileRow{Name: "/", Path: "/", IsDir: true})
	return rows
}

// HostParent is the parent directory, or "" at a volume root (back to HostRoots).
func HostParent(abs string) string {
	abs = filepath.Clean(abs)
	parent := filepath.Dir(abs)
	if parent == abs {
		return ""
	}
	return filepath.ToSlash(parent)
}

// ListHostEntries lists a host directory, omitting names that start with a dot.
func ListHostEntries(abs string) ([]FileRow, error) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, ErrBadHostPath
	}
	var rows []FileRow
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		rows = append(rows, FileRow{
			Name:  entry.Name(),
			Path:  filepath.ToSlash(filepath.Join(abs, entry.Name())),
			IsDir: entry.IsDir(),
		})
	}
	return rows, nil
}
