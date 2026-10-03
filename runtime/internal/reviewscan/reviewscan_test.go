package reviewscan

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// workspace writes files under a fresh root. A temp dir is outside any git
// work tree, so the inventory walks it.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan(t *testing.T, root string, options Options) Report {
	t.Helper()
	if options.MinSeverity == "" {
		options.MinSeverity = SeverityInfo
	}
	report, err := Scan(t.Context(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// found lists "path:line rule" for every finding.
func found(report Report) []string {
	var out []string
	for _, f := range report.Findings {
		out = append(out, f.Path+":"+strconv.Itoa(f.Line)+" "+f.Rule)
	}
	return out
}

func has(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// Each case is a snippet that must raise a rule (want) and a near miss that
// must not (clean). The near misses are the idioms a naive rule flags.
func TestShapeRulesFireOnBugsAndStayQuietOnIdioms(t *testing.T) {
	cases := []struct {
		name, rule, path, bug, clean string
		line                         int
	}{
		{"self comparison", ruleSelfComparison, "a.py", "def f(x):\n    return x == x\n", "def f(x, y):\n    return x == y\n", 2},
		{"self comparison ignores calls", ruleSelfComparison, "a.js", "export function f(x) { return x.a === x.a; }\n", "export function f() { return rand() === rand(); }\n", 1},
		{"nan comparison", ruleNaNComparison, "a.js", "export function f(x) { return x === NaN; }\n", "export function f(x) { return Number.isNaN(x); }\n", 1},
		{"identical branches", ruleIdenticalBranch, "a.go", "package p\n\nfunc F(a bool) int {\n\tif a {\n\t\treturn 1\n\t} else {\n\t\treturn 1\n\t}\n}\n", "package p\n\nfunc F(a bool) int {\n\tif a {\n\t\treturn 1\n\t} else {\n\t\treturn 2\n\t}\n}\n", 4},
		{"identical ternary", ruleIdenticalBranch, "a.py", "def f(c):\n    return 1 if c else 1\n", "def f(c):\n    return 1 if c else 2\n", 2},
		{"duplicate elif", ruleDuplicateCond, "a.py", "def f(x):\n    if x > 1:\n        return 1\n    elif x > 1:\n        return 2\n", "def f(x):\n    if x > 1:\n        return 1\n    elif x > 2:\n        return 2\n", 4},
		{"duplicate else-if", ruleDuplicateCond, "a.js", "export function f(x) {\n  if (x) { return 1; } else if (x) { return 2; }\n}\n", "export function f(x, y) {\n  if (x) { return 1; } else if (y) { return 2; }\n}\n", 2},
		{"duplicate condition with different init", ruleDuplicateCond, "a.go", "package p\n\nfunc F(x int) int {\n\tif x > 1 {\n\t\treturn 1\n\t} else if x > 1 {\n\t\treturn 2\n\t}\n\treturn 0\n}\n", "package p\n\nfunc F() int {\n\tif v, ok := f(); ok {\n\t\treturn v\n\t} else if v, ok := g(); ok {\n\t\treturn v\n\t}\n\treturn 0\n}\n", 6},
		{"empty catch", ruleEmptyHandler, "a.js", "export function f() {\n  try { g(); } catch (e) {}\n}\n", "export function f() {\n  try { g(); } catch (e) { /* storage may be blocked */ }\n}\n", 2},
		{"empty catch explained above", ruleEmptyHandler, "a.js", "export function f() {\n  try { g(); } catch (e) {}\n}\n", "export function f() {\n  // g throws when storage is blocked; the default stands\n  try { g(); } catch (e) {}\n}\n", 2},
		{"empty except", ruleEmptyHandler, "a.py", "def f():\n    try:\n        g()\n    except ValueError:\n        pass\n", "def f():\n    try:\n        g()\n    except ValueError:\n        log()\n", 4},
		{"ignored go error", ruleEmptyHandler, "a.go", "package p\n\nfunc F() {\n\tif err := g(); err != nil {\n\t}\n}\n", "package p\n\nfunc F() error {\n\tif err := g(); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n", 4},
		{"bare except", ruleBroadExcept, "a.py", "def f():\n    try:\n        g()\n    except:\n        log()\n", "def f():\n    try:\n        g()\n    except:\n        log()\n        raise\n", 4},
		{"unreachable", ruleUnreachable, "a.c", "int f(int x) {\n  return x;\n  x++;\n}\n", "int f(int x) {\n  switch (x) {\n  case 1:\n    return 1;\n  case 2:\n    return 2;\n  }\n  return 0;\n}\n", 3},
		{"unreachable keeps hoisted functions", ruleUnreachable, "a.js", "export function f() {\n  return 1;\n  g();\n}\n", "export function f() {\n  return g();\n  function g() { return 1; }\n}\n", 3},
		{"duplicate key", ruleDuplicateKey, "a.py", "X = {'a': 1, \"a\": 2}\n", "X = {'a': 1, 'b': 2}\n", 1},
		{"duplicate key after spread is an override", ruleDuplicateKey, "a.js", "export const o = {a: 1, a: 2};\n", "export const o = {a: 1, ...base, a: 2};\n", 1},
		{"assignment in condition", ruleAssignInCond, "a.c", "int f(int x) {\n  if (x = 1) { return 1; }\n  return 0;\n}\n", "int f(int x) {\n  if ((x = next())) { return 1; }\n  return 0;\n}\n", 2},
		{"constant condition", ruleConstantCond, "a.py", "def f():\n    if False:\n        g()\n", "def f(debug):\n    if debug:\n        g()\n", 2},
		{"self assignment", ruleSelfAssignment, "a.js", "export function f(x) { x = x; return x; }\n", "export function f(x) { x += x; return x; }\n", 1},
		{"mutable default", ruleMutableDefault, "a.py", "def f(x=[]):\n    return x\n", "def f(x=None):\n    return x or []\n", 1},
		{"identity with literal", ruleIdentityLiteral, "a.py", "def f(x):\n    return x is 'a'\n", "def f(x):\n    return x is None\n", 2},
		{"assert tuple", ruleAssertTuple, "a.py", "def f(x):\n    assert (x, 'message')\n", "def f(x):\n    assert x, 'message'\n", 2},
		{"typeof typo", ruleTypeofTypo, "a.js", "export function f(x) { return typeof x === 'undefnied'; }\n", "export function f(x) { return typeof x === 'undefined'; }\n", 1},
		{"return in finally", ruleReturnInFinally, "a.java", "class A {\n  int f() {\n    try { return g(); } finally {\n      return 0;\n    }\n  }\n}\n", "class A {\n  int f() {\n    try { return g(); } finally {\n      close();\n    }\n  }\n}\n", 4},
		{"defer in loop", ruleDeferInLoop, "a.go", "package p\n\nfunc F(fs []F) {\n\tfor _, f := range fs {\n\t\tdefer f.Close()\n\t}\n}\n", "package p\n\nfunc F(fs []F) {\n\tfor _, f := range fs {\n\t\tfunc() {\n\t\t\tdefer f.Close()\n\t\t}()\n\t}\n}\n", 5},
		{"debugger", ruleDebugger, "a.py", "def f():\n    breakpoint()\n", "def f():\n    log()\n", 2},
		{"redefinition", ruleRedefined, "a.py", "def f():\n    return 1\n\ndef f():\n    return 2\n", "class A:\n    @property\n    def f(self):\n        return 1\n\n    @f.setter\n    def f(self, v):\n        pass\n", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bug := found(scan(t, workspace(t, map[string]string{tc.path: tc.bug}), Options{}))
			if want := tc.path + ":" + strconv.Itoa(tc.line) + " " + tc.rule; !has(bug, want) {
				t.Errorf("want %q, got %v", want, bug)
			}
			for _, f := range found(scan(t, workspace(t, map[string]string{tc.path: tc.clean}), Options{})) {
				if strings.HasSuffix(f, " "+tc.rule) {
					t.Errorf("the idiom raised %s", f)
				}
			}
		})
	}
}

func TestUnusedImportsPerLanguage(t *testing.T) {
	root := workspace(t, map[string]string{
		"a.py":        "import os, sys as system\nfrom collections import OrderedDict, defaultdict as dd\nfrom __future__ import annotations\nimport json  # noqa\n\n__all__ = ['OrderedDict']\n\ndef f():\n    return os.getcwd()\n",
		"b.ts":        "import React, { a, b as c } from 'r';\nimport lodash from 'lodash';\nimport type { T } from 't';\nimport './side-effect';\nconst q = require('q');\nexport function f(x: T) { return c(x); }\n",
		"c.jsx":       "import React from 'react';\nexport const App = () => <div />;\n",
		"D.java":      "import java.util.List;\nimport java.util.Map;\nimport java.util.*;\nclass D { List<String> xs; }\n",
		"e.kt":        "import a.b.C\nimport a.b.D as E\nimport kotlin.properties.getValue\nfun f(): C = C()\n",
		"f.php":       "<?php\nuse A\\B;\nuse C\\D as E;\nfunction f() { return new B(); }\n",
		"__init__.py": "from .a import f\n",
	})
	got := found(scan(t, root, Options{}))
	for _, want := range []string{
		"a.py:1 unused-import", "a.py:2 unused-import", // system, dd
		"b.ts:1 unused-import", "b.ts:2 unused-import", "b.ts:5 unused-import", // a/React, lodash, q
		"D.java:2 unused-import", "e.kt:2 unused-import", "f.php:3 unused-import",
	} {
		if !has(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, unwanted := range []string{
		"a.py:4 unused-import", "a.py:3 unused-import", "b.ts:3 unused-import", "b.ts:4 unused-import",
		"c.jsx:1 unused-import", "D.java:1 unused-import", "D.java:3 unused-import",
		"e.kt:1 unused-import", "e.kt:3 unused-import", "f.php:2 unused-import", "__init__.py:1 unused-import",
	} {
		if has(got, unwanted) {
			t.Errorf("false positive %s", unwanted)
		}
	}
}

func TestUnreferencedDefinitionsAcrossFiles(t *testing.T) {
	root := workspace(t, map[string]string{
		"lib.py":      "def used_elsewhere():\n    return 1\n\ndef _dead():\n    return 2\n\ndef _recursive(n):\n    return _recursive(n - 1)\n\ndef _for_tests():\n    return 3\n\ndef by_name():\n    return 4\n\n@app.route('/')\ndef handler():\n    return 5\n\ndef PublicApi():\n    return 6\n",
		"main.py":     "from lib import used_elsewhere\n\nused_elsewhere()\nREGISTRY = ['by_name']\n",
		"test_lib.py": "from lib import _for_tests\n\ndef test_it():\n    assert _for_tests() == 3\n",
		"dispatch.py": "class G:\n    def run(self, kind):\n        return getattr(self, '_handle_' + kind)()\n\n    def _handle_text(self):\n        return 1\n",
		"svc.go":      "package svc\n\ntype T struct{}\n\nfunc (T) String() string { return \"\" }\n\nfunc helper() {}\n\nfunc Exported() {}\n",
	})
	report := scan(t, root, Options{})
	got := found(report)
	for _, want := range []string{
		"lib.py:4 unreferenced", "lib.py:7 unreferenced", "lib.py:10 test-only-reference",
		"lib.py:20 unreferenced", "svc.go:7 unreferenced", "svc.go:9 unreferenced",
	} {
		if !has(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, unwanted := range []string{"lib.py:1 unreferenced", "lib.py:13 unreferenced", "lib.py:17 unreferenced", "svc.go:5 unreferenced", "test_lib.py:3 unreferenced"} {
		if has(got, unwanted) {
			t.Errorf("false positive %s", unwanted)
		}
	}
	for _, f := range report.Findings {
		if f.Path == "dispatch.py" && f.Severity != SeverityInfo {
			t.Errorf("a method reachable through a built name is reported as dead: %+v", f)
		}
		if f.Path == "lib.py" && f.Line == 20 && f.Severity != SeverityInfo {
			t.Errorf("a public definition with no in-repo caller is info, got %s", f.Severity)
		}
		if f.Path == "svc.go" && f.Line == 7 && f.Severity != SeverityMedium {
			t.Errorf("an unexported Go function with no caller is medium, got %s", f.Severity)
		}
	}
}

// The blocks of a file must cover every non-blank line, so a reviewer who
// reads every block has read the whole file.
func TestBlocksCoverEveryNonBlankLine(t *testing.T) {
	src := "import os\n\nX = 1\n\nclass A:\n    y = 2\n\n    def m(self):\n        return os\n\n\ndef f():\n    return A()\n\nif __name__ == '__main__':\n    f()\n"
	report := scan(t, workspace(t, map[string]string{"a.py": src}), Options{})
	if len(report.Files) != 1 {
		t.Fatalf("files = %v", report.Files)
	}
	file := report.Files[0]
	for i, text := range strings.Split(src, "\n") {
		if strings.TrimSpace(text) == "" {
			continue
		}
		if innermost(file.Blocks, i+1) < 0 {
			t.Errorf("line %d %q is in no block: %+v", i+1, text, file.Blocks)
		}
	}
	var labels []string
	for _, b := range file.Blocks {
		labels = append(labels, b.Label())
	}
	want := []string{"top-level L1-L3", "class A L5-L9", "method m L8-L9", "function f L12-L13", "top-level L15-L16"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("blocks = %v, want %v", labels, want)
	}
}

func TestInstalledAndGeneratedFilesAreNotScanned(t *testing.T) {
	root := workspace(t, map[string]string{
		"src/app.py":                "import os\n",
		"node_modules/dep/index.js": "import x from 'x';\n",
		".venv/lib/site.py":         "import os\n",
		"myenv/pyvenv.cfg":          "home = /usr\n",
		"myenv/lib/thing.py":        "import os\n",
		"build/out.py":              "import os\n",
		"target/classes/A.java":     "import a.B;\nclass A {}\n",
		"vendor/lib/x.go":           "package x\n",
		"__pycache__/a.py":          "import os\n",
		"dist/bundle.min.js":        "import a from 'a';\n",
		"src/generated.pb.go":       "// Code generated by protoc-gen-go. DO NOT EDIT.\npackage x\n",
	})
	report := scan(t, root, Options{})
	var paths []string
	for _, f := range report.Files {
		paths = append(paths, f.Path)
	}
	if strings.Join(paths, ",") != "src/app.py" {
		t.Errorf("scanned %v, want only src/app.py", paths)
	}
}

func TestSuppressionCommentsSilenceALine(t *testing.T) {
	root := workspace(t, map[string]string{"a.py": "def f(x):\n    return x == x  # noqa\n\ndef g(x):\n    # review:ignore deliberate NaN probe\n    return x != x\n"})
	if got := found(scan(t, root, Options{})); len(got) != 0 {
		t.Errorf("suppressed findings still reported: %v", got)
	}
}

func TestBlockViewFoldsNestedBlocksAndListsCallers(t *testing.T) {
	root := workspace(t, map[string]string{
		"a.py": "class A:\n    def m(self):\n        return 1\n\n    def n(self):\n        return 2\n",
		"b.py": "from a import A\nA().m()\n",
	})
	report := scan(t, root, Options{})
	view, err := report.Block(filepath.Join(root, "a.py") + ":1")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteBlock(&out, view); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"class A L1-L6", "references: 1 in b.py", "... method m L2-L3 (folded", "... method n L5-L6 (folded"} {
		if !strings.Contains(text, want) {
			t.Errorf("block view lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "return 1") {
		t.Errorf("a nested block was printed instead of folded:\n%s", text)
	}
	byName, err := report.Block(filepath.Join(root, "a.py") + ":n")
	if err != nil || byName.Block.Name != "n" {
		t.Errorf("lookup by name = %+v, %v", byName.Block, err)
	}
}

func TestChangedModeReportsOnlyTheChangeAndOrphans(t *testing.T) {
	if _, err := safexec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := workspace(t, map[string]string{
		"m.py":     "def helper():\n    return 1\n\ndef caller():\n    return helper()\n",
		"other.py": "def old(x):\n    return x == x\n",
	})
	run := func(args ...string) {
		t.Helper()
		cmd, err := safexec.CommandContext(t.Context(), "git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if err != nil {
			t.Fatalf("git command: %v", err)
		}
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("add", ".")
	run("commit", "-qm", "base")
	run("checkout", "-qb", "feature")
	if err := os.WriteFile(filepath.Join(root, "m.py"), []byte("def helper():\n    return 1\n\ndef caller(y):\n    if y == y:\n        return 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := scan(t, root, Options{Changed: true, MinSeverity: SeverityLow})
	got := found(report)
	if !has(got, "m.py:5 self-comparison") || !has(got, "m.py:1 orphaned-by-change") {
		t.Errorf("missing the change's findings: %v", got)
	}
	if has(got, "other.py:2 self-comparison") {
		t.Errorf("an unchanged file's finding was reported: %v", got)
	}
	if report.Base != "main" {
		t.Errorf("base = %q, want main", report.Base)
	}
	var changed []string
	for _, b := range report.Files[0].Blocks {
		if b.Changed {
			changed = append(changed, b.Label())
		}
	}
	if strings.Join(changed, "|") != "function caller L4-L6" {
		t.Errorf("changed blocks = %v", changed)
	}
}
