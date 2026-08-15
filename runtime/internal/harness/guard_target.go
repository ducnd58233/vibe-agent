package harness

import (
	"os"
	"path/filepath"
	"strings"

	enry "github.com/go-enry/go-enry/v2"
)

// Targeting asks one question - "is this guard interested in this file?" - and
// the answer used to be a list of extensions copied into every guard. Four
// lists drift, and each was wrong in the same direction: a stack nobody thought
// of was simply unguarded, and a monorepo has several of those by definition.
//
// enry answers it from the file's name *and* its content, which matters more
// than it sounds. By extension alone it calls .php Hack, .rs RenderScript, and
// .md a GCC Machine Description, because those extensions are genuinely
// ambiguous and the first candidate wins. Given the bytes it resolves all three
// correctly, and resolves .tf to HCL, .ino to C++, .m to Objective-C, and .dart
// to Dart without any of them being named here.
//
// It also separates Prose from code, which is the fix for a whole class of
// noise: a design document discussing a password is not a leak, and the
// extension lists had no way to say so.

// Language types, named rather than numbered so a config file and a Go rule
// spell the same thing. These are enry's own categories.
const (
	typeProgramming = "Programming"
	typeData        = "Data"
	typeMarkup      = "Markup"
	typeProse       = "Prose"
	typeUnknown     = "Unknown"
)

var enryTypeNames = map[enry.Type]string{
	enry.Programming: typeProgramming,
	enry.Data:        typeData,
	enry.Markup:      typeMarkup,
	enry.Prose:       typeProse,
	enry.Unknown:     typeUnknown,
}

// A subject is the file a tool call just wrote, resolved once so every guard
// reads the same facts about it rather than re-deciding them.
type subject struct {
	Path     string
	Text     string
	Language string
	Type     string
}

// A selector says which files a guard reads.
//
// Three ways to say it, most general first. Types cover a stack nobody listed;
// Languages pin a guard to one; Extensions are the escape hatch for files enry
// cannot name at all, such as `Dockerfile.prod`, which has no extension and no
// exact-name match.
type selector struct {
	Types      []string `yaml:"types"`
	Languages  []string `yaml:"languages"`
	Extensions []string `yaml:"extensions"`
}

// matches reports whether a guard with this selector should read this file.
//
// The three fields are an OR: naming a language does not narrow a type that
// already included it, it adds one enry may have typed differently.
func (s selector) matches(file subject) bool {
	for _, want := range s.Types {
		if strings.EqualFold(want, file.Type) {
			return true
		}
	}
	for _, want := range s.Languages {
		if strings.EqualFold(want, file.Language) {
			return true
		}
	}
	if len(s.Extensions) > 0 && hasSuffixFold(file.Path, s.Extensions) {
		return true
	}
	return false
}

// empty reports a selector that names nothing, which would silently match no
// file. Reported by doctor rather than treated as "match everything": a guard
// that guards nothing is the failure this whole port exists to make visible.
func (s selector) empty() bool {
	return len(s.Types) == 0 && len(s.Languages) == 0 && len(s.Extensions) == 0
}

// The categories the built-in plan selects with are written in
// guards-default.yaml rather than here, so that a repository extending a guard
// reads the same vocabulary the defaults do:
//
//	types: [Programming, Data]   code and configuration - what ships, or what
//	                             decides what ships. Reaches HCL, Dart, Ruby,
//	                             PHP, Swift, Objective-C, Rust, and Kotlin
//	                             without naming any of them.
//	types: [Markup]              what renders: CSS, SCSS, Less, HTML, Vue,
//	                             Svelte, Astro. The UI guards add the JS family
//	                             by name, because a Tailwind class lives in a
//	                             .tsx as readily as in a .css.
//	types: [Prose]               documentation, which no code rule selects.

// resolveSubject reads the file a write-like tool just changed.
//
// The text comes from disk rather than from the payload because the guards
// judge the file that now exists: an Edit sends only its replacement half, and
// a heredoc writes a file with no tool_input at all.
func resolveSubject(req Request, body payload) (subject, bool) {
	if !isFileWrite(body.ToolName) {
		return subject{}, false
	}
	target := body.writeTarget()
	if target == "" {
		return subject{}, false
	}

	full := target
	if !filepath.IsAbs(full) {
		full = filepath.Join(req.WorkspaceRoot, full)
	}
	data, err := os.ReadFile(filepath.Clean(full))
	if err != nil {
		return subject{}, false
	}

	// enry's vendor and documentation rules are patterns over a repository-
	// relative, slash-separated path: `^vendor/`, `^node_modules/`, `^docs/`.
	// Hosts send an absolute path, and on Windows they send it with
	// backslashes, so handing that straight over matches none of them and every
	// vendored file reads as the author's own code.
	name := relativeToWorkspace(req.WorkspaceRoot, full)

	// Vendored and generated files are nobody's authored code. Skipping them
	// here rather than in each guard means a rule added later inherits it.
	if enry.IsVendor(name) || enry.IsGenerated(name, data) {
		return subject{}, false
	}

	language := enry.GetLanguage(name, data)
	return subject{
		Path:     name,
		Text:     string(data),
		Language: language,
		Type:     languageType(language),
	}, true
}

// relativeToWorkspace renders a path the way enry's rules and a reader both
// expect it: relative to the repository, with forward slashes.
//
// Falls back to the base name when the file sits outside the workspace, which
// keeps an absolute temporary path out of a message meant to name a file
// someone can open.
func relativeToWorkspace(root, full string) string {
	if root == "" {
		return filepath.ToSlash(full)
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(full)
	}
	return filepath.ToSlash(rel)
}

// languageType names enry's category for a language.
func languageType(language string) string {
	if name, ok := enryTypeNames[enry.GetLanguageType(language)]; ok {
		return name
	}
	return typeUnknown
}
