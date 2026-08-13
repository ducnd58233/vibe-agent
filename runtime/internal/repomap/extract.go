// Package repomap builds a structural index of a workspace so an agent can
// orient in it without reading it.
//
// The problem it solves is measured, not theoretical: answering "what calls
// this?" by opening files costs tens of thousands of tokens and tens of seconds,
// where the same answer off an index costs a few hundred and no time at all.
//
// Two rules shape it, both inherited from internal/memory:
//
//   - Nothing here is a summary. Every entry is a declaration that exists at a
//     named line, so the source type is file_content and no model wrote any of
//     it. A map that paraphrased the code would be model output wearing the
//     costume of an index.
//   - The cache is keyed by content hash. A file that has not changed is not
//     re-read, and a file that has is re-read before anything is reported about
//     it, so the map cannot describe code that is no longer there.
package repomap

import (
	"bytes"
	"regexp"
	"strings"
)

// Symbol is one declaration, at the line a reader can jump to.
type Symbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// binarySniff is how far in to look for a NUL before calling content binary.
// Text files do not carry one, and the alternative is running every pattern
// over a few megabytes of image for no symbols.
const binarySniff = 1024

// rule is one language's pattern for one kind of declaration.
//
// Regexes, and that is a known weakness rather than a defended choice. One
// pattern set per language means every language not written here is silent, and
// silence in a map reads as absence from the code. The dynamic answer is a
// tree-sitter runtime with per-grammar tag queries, which now exists in pure Go;
// until that lands, the coverage note in render.go is what keeps this honest.
type rule struct {
	pattern *regexp.Regexp
	kind    string
}

// language is how one file type is read.
type language struct {
	// comment starts a line that declares nothing, whatever it looks like.
	comment string
	rules   []rule
	// container, when set, names the pattern that opens a scope whose members
	// are qualified with its name. Python's classes are the only such scope
	// here; Go's methods carry their receiver in the declaration itself.
	container *regexp.Regexp
	// member matches a declaration indented inside a container.
	member *regexp.Regexp
}

var (
	goLang = language{
		comment: "//",
		rules: []rule{
			{regexp.MustCompile(`^func\s+\(\s*\w+\s+\*?(\w+)\s*\)\s+([A-Z]\w*)`), "method"},
			{regexp.MustCompile(`^func\s+([A-Z]\w*)`), "func"},
			{regexp.MustCompile(`^type\s+([A-Z]\w*)`), "type"},
			{regexp.MustCompile(`^const\s+([A-Z]\w*)`), "const"},
			{regexp.MustCompile(`^var\s+([A-Z]\w*)`), "var"},
		},
	}

	pythonLang = language{
		comment:   "#",
		container: regexp.MustCompile(`^class\s+([A-Za-z]\w*)`),
		member:    regexp.MustCompile(`^\s+(?:async\s+)?def\s+([A-Za-z]\w*)`),
		rules: []rule{
			{regexp.MustCompile(`^class\s+([A-Za-z]\w*)`), "class"},
			{regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z]\w*)`), "func"},
			{regexp.MustCompile(`^([A-Z][A-Z0-9_]*)\s*(?::[^=]+)?=`), "const"},
		},
	}

	// Only exported declarations. An unexported one is not reachable from
	// anywhere the map is read from, so listing it adds tokens and no bearings.
	tsLang = language{
		comment: "//",
		rules: []rule{
			{regexp.MustCompile(`^export\s+default\s+(?:async\s+)?function\s+(\w+)`), "func"},
			{regexp.MustCompile(`^export\s+(?:async\s+)?function\s+(\w+)`), "func"},
			{regexp.MustCompile(`^export\s+(?:abstract\s+)?class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^export\s+interface\s+(\w+)`), "interface"},
			{regexp.MustCompile(`^export\s+type\s+(\w+)`), "type"},
			{regexp.MustCompile(`^export\s+enum\s+(\w+)`), "enum"},
			{regexp.MustCompile(`^export\s+(?:const|let|var)\s+(\w+)`), "const"},
		},
	}

	rustLang = language{
		comment: "//",
		rules: []rule{
			{regexp.MustCompile(`^pub\s+(?:async\s+)?fn\s+(\w+)`), "func"},
			{regexp.MustCompile(`^pub\s+struct\s+(\w+)`), "struct"},
			{regexp.MustCompile(`^pub\s+trait\s+(\w+)`), "trait"},
			{regexp.MustCompile(`^pub\s+enum\s+(\w+)`), "enum"},
			{regexp.MustCompile(`^pub\s+type\s+(\w+)`), "type"},
			{regexp.MustCompile(`^pub\s+const\s+(\w+)`), "const"},
		},
	}

	javaLang = language{
		comment: "//",
		rules: []rule{
			{regexp.MustCompile(`^\s*public\s+(?:final\s+|abstract\s+)?class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^\s*public\s+interface\s+(\w+)`), "interface"},
			{regexp.MustCompile(`^\s*public\s+enum\s+(\w+)`), "enum"},
			{regexp.MustCompile(`^\s*public\s+(?:static\s+)?(?:final\s+)?[\w<>\[\], ]+\s+(\w+)\s*\(`), "method"},
		},
	}
)

// byExtension maps a file suffix to how it is read. A suffix absent here yields
// no symbols, which is the honest answer: the file is in the map by path, and
// nothing is claimed about its contents.
var byExtension = map[string]*language{
	".go":   &goLang,
	".py":   &pythonLang,
	".ts":   &tsLang,
	".tsx":  &tsLang,
	".js":   &tsLang,
	".jsx":  &tsLang,
	".mjs":  &tsLang,
	".rs":   &rustLang,
	".java": &javaLang,
	".kt":   &javaLang,
}

// Extract returns the declarations in one file, in the order they appear.
//
// Order is source order rather than sorted, because a file's own sequence is
// information: the type comes before the constructor that returns it, and a
// reader scanning the map keeps that.
func Extract(path string, content []byte) []Symbol {
	if IsTest(path) {
		return nil
	}
	lang := byExtension[strings.ToLower(extension(path))]
	if lang == nil {
		return nil
	}
	head := content
	if len(head) > binarySniff {
		head = head[:binarySniff]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return nil
	}

	var symbols []Symbol
	var container string
	for index, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || (lang.comment != "" && strings.HasPrefix(trimmed, lang.comment)) {
			continue
		}
		number := index + 1

		// A container resets on any other unindented line, so a method defined
		// after a class has closed is not attributed to it.
		if lang.container != nil {
			if match := lang.container.FindStringSubmatch(line); match != nil {
				container = match[1]
			} else if line == strings.TrimLeft(line, " \t") {
				container = ""
			} else if container != "" && lang.member != nil {
				if match := lang.member.FindStringSubmatch(line); match != nil {
					if name := match[1]; !strings.HasPrefix(name, "_") {
						symbols = append(symbols, Symbol{Name: container + "." + name, Kind: "method", Line: number})
					}
				}
				continue
			}
		}

		for _, r := range lang.rules {
			match := r.pattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			name := match[1]
			if len(match) > 2 && match[2] != "" {
				// A receiver-qualified declaration: the type, then the method.
				name = match[1] + "." + match[2]
			}
			if strings.HasPrefix(name, "_") {
				break
			}
			symbols = append(symbols, Symbol{Name: name, Kind: r.kind, Line: number})
			break
		}
	}
	return symbols
}

// testMarkers are the naming conventions that make a file a test across the
// languages this package reads.
var testMarkers = []string{"_test.", "-test.", ".test.", ".spec.", "_spec."}

// IsTest reports whether a path names a test file.
//
// Such a file stays in the map by name, because knowing a package has tests is
// orientation. Its cases do not: thirty function names that each restate one
// assertion is most of a budget spent on the part of a repository nobody
// navigates by, and the names are already the best documentation of themselves
// for anyone who opens the file.
func IsTest(path string) bool {
	name := strings.ToLower(path)
	if slash := strings.LastIndexAny(name, `/\`); slash >= 0 {
		name = name[slash+1:]
	}
	if strings.HasPrefix(name, "test_") {
		return true
	}
	for _, marker := range testMarkers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

// extension returns the final suffix of a path, including the dot.
func extension(path string) string {
	if dot := strings.LastIndexByte(path, '.'); dot >= 0 {
		if slash := strings.LastIndexAny(path, `/\`); slash < dot {
			return path[dot:]
		}
	}
	return ""
}
