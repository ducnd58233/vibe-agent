package repomap

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	enry "github.com/go-enry/go-enry/v2"
)

// MaxFileBytes matches the slopaudit walk so oversized files stay out of both.
const MaxFileBytes = 1024 * 1024

func listSourceFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" {
				return filepath.SkipDir
			}
			// Classify vendor against the path relative to the walk root so a
			// test fixture rooted at testdata/ is still readable: enry treats
			// the string "testdata" as vendor, which would SkipDir the root.
			if path != root {
				relDir, relErr := filepath.Rel(root, path)
				if relErr == nil && enry.IsVendor(filepath.ToSlash(relDir)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if skipFile(rel, path) {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

func skipFile(rel, abs string) bool {
	base := filepath.Base(rel)
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	if strings.HasSuffix(base, ".local") || strings.HasSuffix(base, ".local.json") {
		return true
	}
	info, err := os.Stat(abs)
	if err != nil || info.Size() > MaxFileBytes {
		return true
	}
	data, err := os.ReadFile(filepath.Clean(abs)) //nolint:gosec // abs is joined from walked workspace paths only
	if err != nil {
		return true
	}
	if enry.IsBinary(data) || enry.IsImage(rel) || enry.IsGenerated(rel, data) {
		return true
	}
	return false
}
