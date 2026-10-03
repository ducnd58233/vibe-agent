// Package architecture holds checks on the shape of the runtime itself: who may
// import whom, and where a supported agent may be named. They exist because both
// rules lived only in prose (runtime/AGENTS.md), and prose is not enforced.
package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/ducnd58233/vibe-agent/runtime/"

// runtimeRoot is the directory holding go.mod, found from this file's location
// so the test does not depend on the working directory.
func runtimeRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// sources returns every non-test Go file under root, slash-separated and
// relative to it.
func sources(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// importsOf returns the in-module packages a file imports, relative to the module.
func importsOf(t *testing.T, root, rel string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(rel)), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	var out []string
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if strings.HasPrefix(path, modulePath) {
			out = append(out, strings.TrimPrefix(path, modulePath))
		}
	}
	return out
}

// module is the top-level unit a package belongs to: internal/<name>, or the
// first path element for anything outside internal.
func module(pkg string) string {
	parts := strings.Split(pkg, "/")
	if parts[0] == "internal" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

func isInfra(pkg string) bool { return strings.Contains("/"+pkg+"/", "/infra/") }

func isApp(pkg string) bool { return strings.Contains("/"+pkg+"/", "/app/") }

// compositionRoots wire concrete adapters together, so they alone may import a
// module's infra. Everything else goes through the module's own façade.
func compositionRoot(pkg string) bool {
	return pkg == "cmd" || pkg == "internal/web/app"
}

// The dependency rules in runtime/AGENTS.md, as code.
func TestDependencyDirection(t *testing.T) {
	root := runtimeRoot(t)
	var violations []string
	for _, rel := range sources(t, root) {
		pkg := filepath.ToSlash(filepath.Dir(rel))
		for _, imp := range importsOf(t, root, rel) {
			switch {
			case strings.HasPrefix(pkg, "internal/shared") && !strings.HasPrefix(imp, "internal/shared") && imp != "migrations":
				violations = append(violations, rel+" (shared) imports "+imp+": shared depends on nothing above it")
			case strings.HasSuffix(pkg, "/domain") && (isInfra(imp) || isApp(imp)):
				violations = append(violations, rel+" (domain) imports "+imp+": a domain package imports neither its app nor its infra")
			case isApp(pkg) && !compositionRoot(pkg) && isInfra(imp) && module(imp) == module(pkg):
				violations = append(violations, rel+" (app) imports "+imp+": an application package depends on ports, not infra")
			case isInfra(imp) && !strings.HasPrefix(imp, "internal/shared") && module(imp) != module(pkg) && !compositionRoot(pkg):
				violations = append(violations, rel+" imports "+imp+": another module's infra is reached through its façade")
			case strings.HasPrefix(imp, "internal/legacy") && pkg != "cmd" && !strings.HasPrefix(pkg, "internal/legacy"):
				violations = append(violations, rel+" imports "+imp+": only the migrate command reaches the legacy layouts; modules store state one way")
			case strings.HasPrefix(imp, "internal/testutil"):
				violations = append(violations, rel+" imports "+imp+": test helpers are for tests only")
			}
		}
	}
	for _, v := range violations {
		t.Error(v)
	}
}

// adapterFiles may name a supported agent, because describing one agent's wire
// format, files, or commands is what they are for. Everything else is neutral and
// reads the host tables (internal/hosts, harness.HostContract) instead.
var adapterFiles = []string{
	"internal/hosts/",                     // the catalog: how to run each agent headless and where it keeps files
	"internal/harness/contracts.go",       // each host's hook API, as data
	"internal/harness/contracts_doc.go",   // renders that data
	"internal/harness/hook.go",            // the Client identifiers
	"internal/harness/payload.go",         // inbound payload spellings per host
	"internal/harness/dialect.go",         // outbound envelope shapes
	"cmd/hooks_wiring.go",                 // doctor's reading of each host's config
	"cmd/hooks_host_half.go",              // host-side half of that wiring
	"cmd/hooks_machine_path.go",           // per-host command spelling
	"internal/agent/infra/anthropic/",     // a model provider's API, not a host
	"internal/verifier/reviewbots.go",     // names of third-party review bots
	"internal/fetch/infra/httpx/httpx.go", // names documentation sites in a comment
}

// agentName matches the supported agents. "cursor" alone is a common identifier
// (a position in a stream), so only the product spellings count.
var agentName = regexp.MustCompile(`(?i:claude|codex|opencode|antigravity|kimi|\bmuse\b)|Cursor\b|"cursor"|cursor-agent|\.cursor/`)

func TestNoSupportedAgentIsNamedOutsideTheAdapterLayer(t *testing.T) {
	root := runtimeRoot(t)
	for _, rel := range sources(t, root) {
		if allowed(rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if agentName.MatchString(line) {
				t.Errorf("%s:%d names a supported agent outside the adapter layer: %s\n"+
					"\tread the host tables instead (internal/hosts, harness.HostContract.Dialect), "+
					"or add the file to adapterFiles with a reason", rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func allowed(rel string) bool {
	for _, prefix := range adapterFiles {
		if rel == prefix || (strings.HasSuffix(prefix, "/") && strings.HasPrefix(rel, prefix)) {
			return true
		}
	}
	return false
}

// A host is a row of the contract table, not a Go identifier. Naming one in
// code, as a typed constant or by comparing a client to a string, is how the
// per-host differences used to be scattered; they are Dialect and Tools fields
// now, read from the row.
func TestNoCodeNamesAHostOutsideTheContractTable(t *testing.T) {
	root := runtimeRoot(t)
	hostLiteral := regexp.MustCompile(`Client\("[a-z]|Client = "[a-z]|[Cc]lient [!=]= "[a-z]`)
	allowedFiles := map[string]bool{
		"internal/harness/contracts.go": true, // each row names its own host
		"internal/harness/hook.go":      true, // DefaultClient
	}
	for _, rel := range sources(t, root) {
		if allowedFiles[rel] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if hostLiteral.MatchString(line) {
				t.Errorf("%s:%d names a host in code: %s\n\tput the difference on harness.Dialect or HostContract instead", rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}
