// Package sourcefiles lists the files in a workspace that are the project's
// own text: what a scan should read. It is the one inventory every scanner
// (slop audit, repo map, review scan) walks, so a dependency directory skipped
// for one is skipped for all.
//
// Inside a git work tree the list comes from git itself (tracked plus
// untracked-but-not-ignored files), which applies nested and negated
// .gitignore rules exactly. Elsewhere a walk applies the root .gitignore.
// Either way, installed dependencies, build output, tool caches, virtual
// environments, and go-enry's vendor patterns are dropped by path, and
// Readable drops binary, generated, oversized, and secret-bearing content.
package sourcefiles

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	enry "github.com/go-enry/go-enry/v2"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// MaxFileBytes caps what a scan reads. Larger files are almost always
// generated, minified, or data.
const MaxFileBytes = 1024 * 1024

// File is one listed file: Rel is slash-separated and relative to the root.
type File struct {
	Rel string
	Abs string
}

// dependencyDirs are directory names that hold installed packages, build
// output, or tool caches in some ecosystem. go-enry's vendor list covers
// node_modules, vendor, dist, and friends; these are the ones it leaves out.
var dependencyDirs = map[string]bool{
	// Version control and editor state.
	".git": true, ".hg": true, ".svn": true, ".bzr": true, ".idea": true,
	// Python.
	"venv": true, ".venv": true, "virtualenv": true, ".virtualenv": true, "__pycache__": true,
	"site-packages": true, ".tox": true, ".nox": true, ".eggs": true, ".mypy_cache": true,
	".pytest_cache": true, ".ruff_cache": true, ".hypothesis": true, ".ipynb_checkpoints": true,
	// JavaScript and TypeScript.
	"jspm_packages": true, ".next": true, ".nuxt": true, ".svelte-kit": true, ".output": true,
	".vercel": true, ".turbo": true, ".parcel-cache": true, ".angular": true, ".expo": true,
	"coverage": true, ".nyc_output": true, ".yarn": true, ".pnpm-store": true,
	// JVM, .NET, native, and mobile.
	"build": true, "target": true, "out": true, "obj": true, ".gradle": true, ".m2": true,
	"Pods": true, "DerivedData": true, ".build": true, ".swiftpm": true, ".cxx": true,
	// Other ecosystems.
	"_build": true, ".dart_tool": true, ".pub-cache": true, "elm-stuff": true, ".stack-work": true,
	"dist-newstyle": true, ".cabal-sandbox": true, ".terraform": true, ".serverless": true,
	".cargo": true, ".bundle": true, ".cache": true,
	// This toolkit's own derived state.
	workspace.StateDirName: true,
}

// dependencyDirPrefixes catch generated directory families named per project.
var dependencyDirPrefixes = []string{"bazel-", "cmake-build-"}

// DependencyDir reports whether a directory name is installed or generated
// content rather than the project's own source.
func DependencyDir(name string) bool {
	if dependencyDirs[name] || strings.HasSuffix(name, ".egg-info") {
		return true
	}
	for _, prefix := range dependencyDirPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// List returns the project's own files under root, sorted by Rel.
func List(ctx context.Context, root string) ([]File, error) {
	abs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	rels, ok := gitFiles(ctx, abs)
	if !ok {
		if rels, err = walkFiles(abs); err != nil {
			return nil, err
		}
	}
	venvs := venvCache{}
	out := make([]File, 0, len(rels))
	for _, rel := range rels {
		if skipPath(rel) || venvs.inside(abs, rel) {
			continue
		}
		full := filepath.Join(abs, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, File{Rel: rel, Abs: full})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// skipPath applies every path-only rule to a slash-separated relative path.
func skipPath(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, dir := range parts[:len(parts)-1] {
		if DependencyDir(dir) {
			return true
		}
	}
	return Sensitive(parts[len(parts)-1]) || enry.IsVendor(rel)
}

// Readable reports whether content belongs in a scan: not oversized, binary,
// an image, or generated.
func Readable(rel string, data []byte) bool {
	return len(data) <= MaxFileBytes &&
		!enry.IsBinary(data) &&
		!enry.IsImage(rel) &&
		!enry.IsGenerated(rel, data)
}

// Sensitive reports file names that conventionally hold local secrets.
func Sensitive(name string) bool {
	lower := strings.ToLower(name)
	return lower == ".env" ||
		strings.HasPrefix(lower, ".env.") ||
		strings.HasSuffix(lower, ".local") ||
		strings.HasSuffix(lower, ".local.json")
}

// gitFiles asks git for tracked and untracked-but-not-ignored files. ok is
// false outside a work tree or without git, and the caller walks instead.
func gitFiles(ctx context.Context, root string) ([]string, bool) {
	cmd, err := safexec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, false
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	var rels []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		rel := string(raw)
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		rels = append(rels, rel)
	}
	return rels, true
}

func walkFiles(root string) ([]string, error) {
	ignore := loadGitignore(root)
	var rels []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			if entry != nil && entry.IsDir() {
				// Junctions and symlinked dirs on Windows can make ReadDir fail.
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if DependencyDir(entry.Name()) || ignore.skipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore.skipFile(path) {
			return nil
		}
		if rel, relErr := filepath.Rel(root, path); relErr == nil {
			rels = append(rels, filepath.ToSlash(rel))
		}
		return nil
	})
	return rels, err
}

// venvCache remembers which directories are Python virtual environments.
// A venv can have any name; pyvenv.cfg at its top is what makes it one.
type venvCache map[string]bool

func (c venvCache) inside(root, rel string) bool {
	dir := pathDir(rel)
	for dir != "" {
		isVenv, known := c[dir]
		if !known {
			_, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), "pyvenv.cfg"))
			isVenv = err == nil
			c[dir] = isVenv
		}
		if isVenv {
			return true
		}
		dir = pathDir(dir)
	}
	return false
}

func pathDir(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}
