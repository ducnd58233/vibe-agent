package repomap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, relative, body string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
	return path
}

// gitInit makes a directory a repository, so discovery can ask git what the
// repository keeps rather than guessing from directory names.
func gitInit(t *testing.T, root string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "init", "--quiet")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git unavailable, so gitignore behaviour cannot be tested: %v\n%s", err, out)
	}
}

func build(t *testing.T, root string) Result {
	t.Helper()
	result, err := Build(t.Context(), root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return result
}

func find(result Result, path string) *File {
	for i := range result.Files {
		if filepath.ToSlash(result.Files[i].Path) == path {
			return &result.Files[i]
		}
	}
	return nil
}

func TestBuildIndexesTheWorkspace(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n\ntype Order struct{}\n")
	write(t, root, "README.md", "# docs\n")

	result := build(t, root)

	orders := find(result, "api/orders.go")
	if orders == nil {
		t.Fatalf("api/orders.go missing from the map: %+v", result.Files)
	}
	if got := names(orders.Symbols); !equal(got, []string{"CreateOrder", "Order"}) {
		t.Errorf("symbols = %v", got)
	}
	// A file with no extractable symbols still belongs in the map. Knowing a
	// README is there costs one line and saves the agent looking for it.
	if find(result, "README.md") == nil {
		t.Error("a file with no symbols was dropped from the map entirely")
	}
}

// The whole point of the cache. An unchanged file must not be re-read, and the
// count is what proves it rather than a timing measurement.
func TestUnchangedFilesAreServedFromCache(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")

	first := build(t, root)
	if first.Read != 1 || first.Cached != 0 {
		t.Fatalf("first build read=%d cached=%d, want 1 and 0", first.Read, first.Cached)
	}

	second := build(t, root)
	if second.Read != 0 || second.Cached != 1 {
		t.Errorf("second build read=%d cached=%d, want 0 and 1", second.Read, second.Cached)
	}
}

// The failure that makes a cache worse than none: reporting symbols that are no
// longer in the file. Content hash, not mtime, because a checkout or a restored
// backup moves mtime without changing code and changes code without moving it.
func TestAChangedFileIsReindexed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	build(t, root)

	write(t, root, "api/orders.go", "package api\n\nfunc CancelOrder() {}\n")
	second := build(t, root)

	if second.Read != 1 {
		t.Errorf("a changed file was served from cache: read=%d", second.Read)
	}
	orders := find(second, "api/orders.go")
	if orders == nil {
		t.Fatal("api/orders.go missing")
	}
	if got := names(orders.Symbols); !equal(got, []string{"CancelOrder"}) {
		t.Errorf("symbols = %v, want the new declaration only", got)
	}
}

// A deleted file must leave the map, or the agent is sent to a path that is not
// there.
func TestADeletedFileLeavesTheMap(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	write(t, root, "api/users.go", "package api\n\nfunc CreateUser() {}\n")
	build(t, root)

	if err := os.Remove(filepath.Join(root, "api", "users.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	second := build(t, root)

	if find(second, "api/users.go") != nil {
		t.Error("a deleted file is still in the map")
	}
}

// Changing how extraction works invalidates every cached row, hash or no hash:
// the content is the same and what this package makes of it is not.
func TestABumpedCacheVersionDiscardsEveryRow(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	build(t, root)

	cache, err := openCache(t.Context(), root, cacheVersion+1)
	if err != nil {
		t.Fatalf("openCache: %v", err)
	}
	_, hit := cache.lookup(t.Context(), "api/orders.go", "any-hash")
	_ = cache.Close()
	if hit {
		t.Error("a row survived a cache version bump")
	}
}

// The repository's own .gitignore decides what is noise, not a list in this
// package. A hardcoded list is wrong in both directions: it skips a dist/ that
// some repositories commit, and it indexes a generated directory it has never
// heard of.
func TestGitignoreDecidesWhatIsIndexed(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	write(t, root, ".gitignore", "node_modules/\n/tmp/\n*.log\n.env\n")
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	write(t, root, "node_modules/left-pad/index.js", "export function leftPad() {}\n")
	write(t, root, "tmp/demo/events.ndjson", "{}\n")
	write(t, root, "build.log", "noise\n")
	write(t, root, ".env", "API_KEY=x\n")
	// Not ignored, and a hardcoded skip list would have dropped it.
	write(t, root, "dist/bundle.mjs", "export function boot() {}\n")

	result := build(t, root)

	for _, gone := range []string{"node_modules/left-pad/index.js", "tmp/demo/events.ndjson",
		"build.log", ".env"} {
		if find(result, gone) != nil {
			t.Errorf("%s is gitignored and should not be indexed", gone)
		}
	}
	if find(result, "dist/bundle.mjs") == nil {
		t.Error("dist/bundle.mjs is tracked and was dropped by a list this package should not have")
	}
	if find(result, ".git/config") != nil {
		t.Error(".git is never content")
	}
}

// Everything the repository keeps belongs in the map, whether or not this
// package can read declarations out of it. A frontend is .mjs and .scss and
// .yaml, and an agent told those files do not exist will go looking elsewhere.
func TestEveryTrackedFileAppears(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	paths := []string{
		"web/app.mjs", "web/theme.scss", "web/styles.css",
		"deploy/values.yaml", "docs/guide.md", "scripts/release.sh",
		"db/schema.sql", "Dockerfile", "config.toml",
	}
	for _, p := range paths {
		write(t, root, p, "placeholder\n")
	}

	result := build(t, root)

	for _, p := range paths {
		if find(result, p) == nil {
			t.Errorf("%s is missing from the map", p)
		}
	}
}

// The cache is workspace state, so it belongs beside the memory database and
// never in the toolkit or the user's home.
func TestTheCacheLivesInTheWorkspaceStateDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	build(t, root)

	if _, err := os.Stat(CachePath(root)); err != nil {
		t.Fatalf("no cache at %s: %v", CachePath(root), err)
	}
	if !strings.Contains(filepath.ToSlash(CachePath(root)), "/.agent-state/") {
		t.Errorf("cache path %s is not under the workspace state directory", CachePath(root))
	}
}

// A diagnostic asking about the index must not create one, or running doctor in
// an unrelated directory leaves a database behind.
func TestStatDoesNotCreateAnIndex(t *testing.T) {
	root := t.TempDir()

	files, exists, err := Stat(t.Context(), root)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if exists || files != 0 {
		t.Errorf("Stat reported exists=%v files=%d for a workspace with no index", exists, files)
	}
	if _, err := os.Stat(CachePath(root)); err == nil {
		t.Errorf("Stat created %s", CachePath(root))
	}

	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	build(t, root)

	files, exists, err = Stat(t.Context(), root)
	if err != nil {
		t.Fatalf("Stat after build: %v", err)
	}
	if !exists || files != 1 {
		t.Errorf("Stat reported exists=%v files=%d after indexing one file", exists, files)
	}
}

// Ranking is what makes a budgeted map useful: the file everything imports has
// to survive the cut that drops the leaf nobody references.
func TestFilesAreRankedByHowOftenTheyAreReferenced(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/engine.go", "package core\n\nfunc Engine() {}\n")
	write(t, root, "leaf/unused.go", "package leaf\n\nfunc Unused() {}\n")
	write(t, root, "a/one.go", "package a\n\nfunc One() { core.Engine() }\n")
	write(t, root, "b/two.go", "package b\n\nfunc Two() { core.Engine() }\n")

	result := build(t, root)

	engine, unused := find(result, "core/engine.go"), find(result, "leaf/unused.go")
	if engine == nil || unused == nil {
		t.Fatal("expected both files in the map")
	}
	if engine.Inbound != 2 {
		t.Errorf("core/engine.go inbound = %d, want 2", engine.Inbound)
	}
	if unused.Inbound != 0 {
		t.Errorf("leaf/unused.go inbound = %d, want 0", unused.Inbound)
	}
	if result.Files[0].Path != engine.Path {
		t.Errorf("most-referenced file is not first: %s", result.Files[0].Path)
	}
}

// A name several files declare cannot attribute a mention to any of them.
// "main" appears in every script in a repository, and counting it hands the top
// of the ranking to whichever file happens to declare the most common words.
func TestAmbiguousNamesDoNotRank(t *testing.T) {
	root := t.TempDir()
	// Three files declare main; nothing else distinguishes them.
	for _, name := range []string{"one", "two", "three"} {
		write(t, root, "scripts/"+name+".py", "def main():\n    pass\n")
	}
	// Every one of them mentions main, so an unfiltered count would score each
	// of these at two inbound files apiece.
	write(t, root, "core/engine.py", "class Engine:\n    def start(self):\n        pass\n")
	write(t, root, "app.py", "from core import Engine\n\ndef run():\n    Engine().start()\n")

	result := build(t, root)

	for _, name := range []string{"one", "two", "three"} {
		script := find(result, "scripts/"+name+".py")
		if script == nil {
			t.Fatalf("scripts/%s.py missing", name)
		}
		if script.Inbound != 0 {
			t.Errorf("scripts/%s.py scored %d on a name three files declare", name, script.Inbound)
		}
	}
	engine := find(result, "core/engine.py")
	if engine == nil || engine.Inbound != 1 {
		t.Errorf("core/engine.py should score 1 on its unambiguous name: %+v", engine)
	}
}

// Knowing a package has tests is orientation. Knowing the name of all thirty of
// them is not, and at a few tokens each it is most of a budget spent on the one
// part of a repository nobody navigates by.
func TestTestFilesAppearByNameWithoutTheirCases(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	write(t, root, "api/orders_test.go",
		"package api\n\nfunc TestCreateOrderRejectsAnEmptyCart() {}\n")
	write(t, root, "web/orders.spec.ts", "export function setupHarness() {}\n")

	result := build(t, root)

	goTest := find(result, "api/orders_test.go")
	if goTest == nil {
		t.Fatal("a test file should still be in the map: its presence is the useful part")
	}
	if len(goTest.Symbols) != 0 {
		t.Errorf("test cases are listed in the map: %v", names(goTest.Symbols))
	}
	if spec := find(result, "web/orders.spec.ts"); spec == nil || len(spec.Symbols) != 0 {
		t.Errorf("a .spec file was treated as implementation: %+v", spec)
	}
	if impl := find(result, "api/orders.go"); impl == nil || len(impl.Symbols) != 1 {
		t.Errorf("the implementation lost its symbols: %+v", impl)
	}
}

// The map's silences are its most dangerous property. An unexported function, a
// test case, and a shell script all appear the same way as something that does
// not exist, and an agent that searched the map and found nothing will say so.
// Stating the coverage is what turns "absent" back into "not indexed".
func TestRenderStatesWhatItDoesNotIndex(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n\nfunc helper() {}\n")
	write(t, root, "scripts/deploy.sh", "deploy() { echo hi; }\n")

	rendered := Render(build(t, root), 4000)

	for _, want := range []string{"exported", "test"} {
		if !strings.Contains(strings.ToLower(rendered), want) {
			t.Errorf("the map does not say it omits %s declarations:\n%s", want, rendered)
		}
	}
	// The languages it can read have to be named, or a Ruby file with no symbols
	// reads as a Ruby file with no code.
	if !strings.Contains(rendered, "Go") || !strings.Contains(rendered, "Python") {
		t.Errorf("the map does not name the languages it can read:\n%s", rendered)
	}
}

// A budget that is quietly exceeded is not a budget. A budget that quietly
// drops files is worse, so the render says what it left out.
func TestRenderRespectsTheBudgetAndReportsWhatItDropped(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		write(t, root, "pkg/"+name+".go",
			"package pkg\n\nfunc "+strings.ToUpper(name)+"Handler() {}\n")
	}
	result := build(t, root)

	// Above the footer's own cost, and below the footer plus all eight files, so
	// the clip is what is being measured rather than whether anything fits.
	const budget = 80
	rendered := Render(result, budget)
	if estimateTokens(rendered) > budget {
		t.Errorf("render is %d tokens, over the %d asked for:\n%s",
			estimateTokens(rendered), budget, rendered)
	}
	if !strings.Contains(rendered, "omitted") {
		t.Errorf("a truncated map does not say so:\n%s", rendered)
	}
}

// The footer outranks the listing. A budget too small to hold anything must
// still say what the map does not cover, because that sentence is what stops a
// reader concluding something is missing from the code. Spending it on two more
// filenames buys nothing and costs the only safeguard there is.
func TestRenderKeepsTheCoverageNoteWhenTheBudgetIsTiny(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")

	rendered := Render(build(t, root), 1)

	if !strings.Contains(rendered, "Not indexed") {
		t.Errorf("a budget of 1 dropped the coverage note:\n%s", rendered)
	}
}

func TestRenderNamesEveryFileWhenTheBudgetAllows(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/orders.go", "package api\n\nfunc CreateOrder() {}\n")
	write(t, root, "api/users.go", "package api\n\nfunc CreateUser() {}\n")

	rendered := Render(build(t, root), 4000)

	for _, want := range []string{"api/", "orders.go", "CreateOrder", "users.go", "CreateUser"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("render omits %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "omitted") {
		t.Errorf("a complete map claims to have dropped something:\n%s", rendered)
	}
}
