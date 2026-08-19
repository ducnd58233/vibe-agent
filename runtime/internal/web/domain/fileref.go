package domain

import (
	"path/filepath"
	"strings"

	enry "github.com/go-enry/go-enry/v2"
)

// LooksLikeFileRef reports whether inline code in markdown likely names a
// workspace file rather than a shell snippet or identifier.
//
// Resolution still happens server-side in handleWorkspaceFileView; this only
// decides whether to emit a data-file-view link in rendered HTML.
func LooksLikeFileRef(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, " \t\n`<>|*?") {
		return false
	}
	if strings.Contains(s, "://") || strings.Contains(s, "...") {
		return false
	}

	name := filepath.ToSlash(s)
	if idx := strings.IndexAny(name, "#?"); idx >= 0 {
		name = name[:idx]
	}
	if name == "" {
		return false
	}
	if enry.IsImage(name) {
		return false
	}

	slashName := filepath.ToSlash(name)
	if languages := enry.GetLanguagesByFilename(slashName, nil, nil); len(languages) > 0 {
		return true
	}

	ext := filepath.Ext(name)
	if ext == "" || ext == "." {
		return strings.Contains(name, "/") || strings.HasPrefix(name, ".")
	}
	if filepath.Base(name) == ext {
		return false
	}
	if lang, _ := enry.GetLanguageByExtension(name); lang != "" {
		return true
	}

	return strings.Contains(name, "/") || strings.HasPrefix(name, ".")
}
