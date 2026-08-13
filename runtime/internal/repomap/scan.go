package repomap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// File is one indexed file and how central it is.
type File struct {
	Path    string   `json:"path"`
	Symbols []Symbol `json:"symbols,omitempty"`
	// Inbound is how many other files mention one of this file's declarations.
	// It is the ranking signal, and the reason a budgeted map keeps the module
	// everything imports and drops the leaf nobody does.
	Inbound int `json:"inbound"`
}

// Result is one pass over a workspace.
type Result struct {
	Files []File `json:"files"`
	// Read and Cached account for every file in the map. They are reported
	// rather than logged because "the cache is working" is a claim that should
	// come with a number.
	Read   int `json:"read"`
	Cached int `json:"cached"`
}

// skipDirs is the guess used only where there is no repository to ask.
//
// Popular conventions across ecosystems, nothing more. Where a `.gitignore`
// exists it wins, because it is the maintained statement of what this particular
// project treats as generated, and it will be right about the cases this list
// has never heard of. `.git` is the one entry that holds regardless: it is
// object storage, not content, and no `.gitignore` lists it.
var skipDirs = map[string]bool{
	".git": true, ".agent-state": true, ".hg": true, ".svn": true,
	"node_modules": true, "bower_components": true, "jspm_packages": true,
	"vendor": true, "Pods": true, ".bundle": true,
	"dist": true, "build": true, "out": true, "target": true, "bin": true, "obj": true,
	"tmp": true, "temp": true, ".cache": true, "coverage": true,
	".venv": true, "venv": true, "env": true, "__pycache__": true, ".tox": true,
	".mypy_cache": true, ".pytest_cache": true, ".ruff_cache": true,
	".next": true, ".nuxt": true, ".svelte-kit": true, ".parcel-cache": true,
	".gradle": true, ".terraform": true, ".idea": true, ".vscode": true, ".DS_Store": true,
}

// maxFileBytes skips a file too large to be source. A generated bundle or a
// checked-in dataset costs real time to scan and yields declarations no one
// wrote.
const maxFileBytes = 1 << 20

// identifier matches the tokens that could name a declaration elsewhere. Three
// characters minimum, because shorter names produce matches by coincidence.
var identifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)

// maxReferences caps what is remembered about one file's mentions of other code.
// A file naming more than this many distinct identifiers is generated, and the
// tail adds cache size rather than ranking signal.
const maxReferences = 500

// Build indexes a workspace, reading only the files whose contents changed.
func Build(ctx context.Context, workspaceRoot string) (Result, error) {
	paths, err := discover(ctx, workspaceRoot)
	if err != nil {
		return Result{}, err
	}

	index, err := openCache(ctx, workspaceRoot, cacheVersion)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = index.Close() }()

	var result Result
	entries := make(map[string]entry, len(paths))
	live := make(map[string]bool, len(paths))

	for _, path := range paths {
		content, err := os.ReadFile(filepath.Clean(filepath.Join(workspaceRoot, path)))
		if err != nil {
			continue // a file that vanished mid-scan is not an error worth failing on
		}
		live[path] = true
		hash := digest(content)

		if cached, hit := index.lookup(ctx, path, hash); hit {
			entries[path] = cached
			result.Cached++
			continue
		}
		found := entry{
			Symbols:    Extract(path, content),
			References: referencedNames(content),
		}
		if err := index.store(ctx, path, hash, found); err != nil {
			return Result{}, fmt.Errorf("cache %s: %w", path, err)
		}
		entries[path] = found
		result.Read++
	}

	if err := index.prune(ctx, live); err != nil {
		return Result{}, fmt.Errorf("prune index: %w", err)
	}

	result.Files = rank(paths, entries)
	return result, nil
}

// rank counts, for every file, how many other files mention one of its
// declarations, and orders the map by it.
//
// This is the cheap half of what Aider does with PageRank over a symbol graph: a
// direct count rather than an eigenvector, so a file imported by one hub does
// not inherit the hub's weight. It is enough for the job here, which is deciding
// what survives a token budget, and it needs no iteration to explain.
func rank(paths []string, entries map[string]entry) []File {
	// Which files mention a given name at all. A set per name, so a file that
	// says "Engine" forty times still counts once.
	mentions := make(map[string]map[string]bool)
	for path, found := range entries {
		for _, name := range found.References {
			if mentions[name] == nil {
				mentions[name] = make(map[string]bool)
			}
			mentions[name][path] = true
		}
	}

	// How many files declare each name. A name more than one of them declares
	// cannot attribute a mention to any of them: every script in a repository
	// defines main, so counting it hands the top of the ranking to whichever
	// file declares the most ordinary words. Those names are dropped rather than
	// weighted, because a mention of an ambiguous name is not weak evidence, it
	// is no evidence about which declaration was meant.
	declarers := make(map[string]int)
	for _, found := range entries {
		for _, symbol := range found.Symbols {
			declarers[baseName(symbol.Name)]++
		}
	}

	files := make([]File, 0, len(paths))
	for _, path := range paths {
		found, ok := entries[path]
		if !ok {
			continue
		}
		// A file mentioning its own declarations says nothing about how central
		// they are, so the file itself never counts toward its own score.
		inbound := make(map[string]bool)
		for _, symbol := range found.Symbols {
			name := baseName(symbol.Name)
			if declarers[name] > 1 {
				continue
			}
			for other := range mentions[name] {
				if other != path {
					inbound[other] = true
				}
			}
		}
		files = append(files, File{Path: path, Symbols: found.Symbols, Inbound: len(inbound)})
	}

	// Path breaks ties so the same workspace always renders identically. A map
	// that reorders between runs makes every diff of it unreadable.
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Inbound != files[j].Inbound {
			return files[i].Inbound > files[j].Inbound
		}
		if len(files[i].Symbols) != len(files[j].Symbols) {
			return len(files[i].Symbols) > len(files[j].Symbols)
		}
		return files[i].Path < files[j].Path
	})
	return files
}

// baseName drops the receiver from a qualified name, so a mention of Propose
// counts toward Store.Propose.
func baseName(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

// referencedNames returns the distinct identifiers a file mentions.
func referencedNames(content []byte) []string {
	seen := make(map[string]bool)
	names := make([]string, 0, 64)
	for _, match := range identifier.FindAll(content, -1) {
		name := string(match)
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
		if len(names) >= maxReferences {
			break
		}
	}
	return names
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// discover lists the workspace's own files, as slash-separated relative paths.
//
// Git decides where there is a repository, because the repository has already
// answered this question. A hardcoded skip list is wrong in both directions: it
// drops a `dist/` that some projects commit, and it indexes whatever generated
// directory it has not heard of. `.gitignore` is the maintained, per-repository
// statement of what is not content, versioned alongside the code it describes.
//
// Without a repository there is nothing to consult, and the walk falls back to
// skipDirs. That list is a guess, and it is only ever reached when the better
// answer does not exist.
func discover(ctx context.Context, workspaceRoot string) ([]string, error) {
	if paths, err := gitFiles(ctx, workspaceRoot); err == nil {
		return paths, nil
	}
	return walkFiles(workspaceRoot)
}

// indexable reports whether a walked file belongs in the map, and its
// slash-separated path relative to the workspace.
//
// A bool rather than an error, because every reason to say no here is a reason
// to skip the file quietly: too large to be source, unreadable, or somewhere
// filepath cannot express relative to the root.
func indexable(workspaceRoot, path string, d fs.DirEntry) (string, bool) {
	info, err := d.Info()
	if err != nil || info.Size() > maxFileBytes {
		return "", false
	}
	relative, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// gitFiles asks the repository what it keeps.
//
// `--cached --others --exclude-standard` is tracked files plus untracked ones
// that are not ignored: everything the repository keeps, including work in
// progress that was never staged. It fails when there is no repository and no
// git, which is what sends the caller to the walk.
func gitFiles(ctx context.Context, workspaceRoot string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workspaceRoot,
		"ls-files", "--cached", "--others", "--exclude-standard", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range bytes.Split(out, []byte{0}) {
		name := string(entry)
		if name == "" {
			continue
		}
		// A submodule is listed as a single entry and belongs to another
		// repository, which keeps its own map.
		info, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(name)))
		if err != nil || info.IsDir() || info.Size() > maxFileBytes {
			continue
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths, nil
}

// walkFiles is the fallback for a directory that is not a repository.
func walkFiles(workspaceRoot string) ([]string, error) {
	var paths []string
	// The walk's own error is named "walkErr" and swallowed on purpose: a
	// directory this process cannot read is one to skip, not one to fail the
	// whole map over, and returning nil from the callback is how WalkDir is told
	// to carry on.
	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		// fs.SkipDir, not nil: an entry this process cannot read is a subtree
		// to step over, which is what WalkDir's contract calls that, and it
		// says so rather than reporting success on a failure just seen.
		if walkErr != nil || d == nil {
			return fs.SkipDir
		}
		if d.IsDir() {
			if path != workspaceRoot && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if relative, keep := indexable(workspaceRoot, path, d); keep {
			paths = append(paths, relative)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", workspaceRoot, err)
	}
	sort.Strings(paths)
	return paths, nil
}
