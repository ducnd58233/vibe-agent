package reviewscan

import (
	"regexp"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// spec is what differs per grammar. Everything else in this package reads node
// shapes that every grammar shares, so a language missing here still gets
// blocks, references, and the shape rules; it only loses the checks below.
type spec struct {
	// importTypes are the statements that bind imported names, and binder
	// returns those names. A language whose compiler already rejects or warns
	// on an unused import (Go, Rust, C#), or whose imports bind a module rather
	// than names (C, Ruby, Swift), has none: a guess there is only noise.
	importTypes map[string]bool
	binder      func(w *walker, n *gotreesitter.Node, t string) []binding
	// jsxImplies names an import that JSX uses without spelling it.
	jsxImplies string
	// requireCall binds `const x = require("x")` as an import.
	requireCall bool

	// How a definition is kept private to its file or package.
	exportByCase  bool // an identifier starting in lower case is unexported
	exportByPub   bool // a definition without `pub` is private to its module
	staticIsLocal bool // a top-level `static` function is private to its file
	esModule      bool // a top-level definition not under `export` is module-private

	// redefines: a second definition of a name in the same scope silently
	// replaces the first (no overloads, no compile error).
	redefines bool
	// sharedDefaults: default argument values are evaluated once and shared
	// between calls, so a mutable literal default leaks state.
	sharedDefaults bool
	// deferAtReturn: deferred calls run when the function returns, not when
	// the enclosing block ends.
	deferAtReturn bool
	// ignoredOperators are import names a language calls implicitly
	// (operator and delegate conventions), so they are never "unused".
	ignoredOperators *regexp.Regexp
}

var ecmaImports = map[string]bool{"import_statement": true}

var specs = map[string]spec{
	"python": {
		importTypes: map[string]bool{"import_statement": true, "import_from_statement": true},
		binder:      pythonBindings, redefines: true, sharedDefaults: true,
	},
	"javascript": {importTypes: ecmaImports, binder: ecmaBindings, jsxImplies: "React", requireCall: true, esModule: true, redefines: true},
	"typescript": {importTypes: ecmaImports, binder: ecmaBindings, requireCall: true, esModule: true, redefines: true},
	"tsx":        {importTypes: ecmaImports, binder: ecmaBindings, jsxImplies: "React", requireCall: true, esModule: true, redefines: true},
	"java":       {importTypes: map[string]bool{"import_declaration": true}, binder: lastNameBinding},
	"kotlin": {
		importTypes: map[string]bool{"import_header": true}, binder: lastNameBinding,
		ignoredOperators: regexp.MustCompile(`^(getValue|setValue|provideDelegate|plus|minus|times|div|rem|mod|rangeTo|rangeUntil|contains|get|set|invoke|iterator|next|hasNext|compareTo|equals|unaryPlus|unaryMinus|not|inc|dec|(plus|minus|times|div|rem)Assign|component\d+)$`),
	},
	"php":  {importTypes: map[string]bool{"namespace_use_declaration": true}, binder: phpBindings},
	"go":   {exportByCase: true, deferAtReturn: true},
	"rust": {exportByPub: true},
	"c":    {staticIsLocal: true},
	"cpp":  {staticIsLocal: true},
	"ruby": {redefines: true},
	"lua":  {redefines: true},
}

// binding is one name an import statement brings into scope.
type binding struct {
	name string
	line int
}

func (w *walker) bind(n *gotreesitter.Node) []binding {
	if n == nil {
		return nil
	}
	name := n.Text(w.src)
	if name == "" || name == "_" || name == "*" {
		return nil
	}
	w.markName(n)
	return []binding{{name: name, line: line(n)}}
}

// pythonBindings: `import a.b` binds a, `import a as b` binds b, and
// `from m import x, y as z` binds x and z. __future__ imports bind nothing.
func pythonBindings(w *walker, n *gotreesitter.Node, t string) []binding {
	var module *gotreesitter.Node
	if t == "import_from_statement" {
		module = n.ChildByFieldName("module_name", w.lang)
		if module != nil && module.Text(w.src) == "__future__" {
			return nil
		}
	}
	var out []binding
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if module != nil && child.StartByte() == module.StartByte() {
			continue
		}
		switch child.Type(w.lang) {
		case "dotted_name":
			if t == "import_statement" {
				out = append(out, w.bind(child.NamedChild(0))...)
			} else {
				out = append(out, w.bind(child)...)
			}
		case "aliased_import":
			out = append(out, w.bind(lastNamed(child))...)
		}
	}
	return out
}

// ecmaBindings: default, namespace, and named imports (the alias when there
// is one). A side-effect import (`import "x"`) binds nothing.
func ecmaBindings(w *walker, n *gotreesitter.Node, _ string) []binding {
	var out []binding
	for i := 0; i < n.NamedChildCount(); i++ {
		clause := n.NamedChild(i)
		if clause.Type(w.lang) != "import_clause" {
			continue
		}
		for j := 0; j < clause.NamedChildCount(); j++ {
			part := clause.NamedChild(j)
			switch part.Type(w.lang) {
			case "identifier":
				out = append(out, w.bind(part)...)
			case "namespace_import":
				out = append(out, w.bind(lastNamed(part))...)
			case "named_imports":
				for k := 0; k < part.NamedChildCount(); k++ {
					spec := part.NamedChild(k)
					if spec.Type(w.lang) != "import_specifier" {
						continue
					}
					if alias := spec.ChildByFieldName("alias", w.lang); alias != nil {
						out = append(out, w.bind(alias)...)
					} else {
						out = append(out, w.bind(lastNamed(spec))...)
					}
				}
			}
		}
	}
	return out
}

// requireBindings: `const x = require("x")` and `const { a, b: c } = require("x")`
// at the top of a file.
func requireBindings(w *walker, declarator *gotreesitter.Node) []binding {
	value := declarator.ChildByFieldName("value", w.lang)
	if value == nil || !strings.Contains(value.Type(w.lang), "call") {
		return nil
	}
	callee := value.ChildByFieldName("function", w.lang)
	if callee == nil || callee.Text(w.src) != "require" {
		return nil
	}
	name := declarator.ChildByFieldName("name", w.lang)
	if name == nil {
		return nil
	}
	if name.Type(w.lang) == "identifier" {
		return w.bind(name)
	}
	var out []binding
	for i := 0; i < name.NamedChildCount(); i++ {
		part := name.NamedChild(i)
		switch part.Type(w.lang) {
		case "shorthand_property_identifier_pattern":
			out = append(out, w.bind(part)...)
		case "pair_pattern":
			out = append(out, w.bind(part.ChildByFieldName("value", w.lang))...)
		}
	}
	return out
}

// lastNameBinding: `import a.b.C` binds C and `import a.b.C as D` binds D.
// A wildcard import binds nothing a scan can name.
func lastNameBinding(w *walker, n *gotreesitter.Node, _ string) []binding {
	var alias *gotreesitter.Node
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type(w.lang) {
		case "asterisk", "wildcard_import", "*":
			return nil
		case "import_alias":
			alias = lastLeaf(child)
		}
	}
	if alias != nil {
		return w.bind(alias)
	}
	name := lastLeaf(n)
	if name == nil {
		return nil
	}
	if ops := w.spec.ignoredOperators; ops != nil && ops.MatchString(name.Text(w.src)) {
		return nil
	}
	return w.bind(name)
}

// phpBindings: `use A\B;` binds B, `use A\B as C;` binds C, and grouped uses
// bind each member.
func phpBindings(w *walker, n *gotreesitter.Node, _ string) []binding {
	var out []binding
	var visit func(node *gotreesitter.Node)
	visit = func(node *gotreesitter.Node) {
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			switch child.Type(w.lang) {
			case "namespace_use_clause", "namespace_use_group_clause":
				var alias *gotreesitter.Node
				for j := 0; j < child.NamedChildCount(); j++ {
					if part := child.NamedChild(j); part.Type(w.lang) == "name" && j > 0 {
						alias = part
					}
				}
				if alias == nil {
					alias = lastLeaf(child)
				}
				out = append(out, w.bind(alias)...)
			default:
				visit(child)
			}
		}
	}
	visit(n)
	return out
}

// testPath marks files that hold tests, by the naming conventions test
// runners discover them with.
var testPath = regexp.MustCompile(`(^|/)(tests?|__tests__|spec|specs|testing)/|_test\.[A-Za-z]+$|(^|/)test_[^/]*\.py$|\.(test|spec)\.[A-Za-z0-9]+$|(Test|Tests|Spec|IT)\.(java|kt|cs|scala|swift|groovy)$|_spec\.rb$|(^|/)conftest\.py$`)

// testName marks definitions a test runner calls by convention.
var testName = regexp.MustCompile(`^(?i:test)|(?i:test|tests|spec)$|^(Benchmark|Example|Fuzz)|^(setUp|tearDown|setup|teardown)`)

// runtimeNames are called by a language runtime or framework protocol, never
// by name in the project: entry points, constructors, and the conversion,
// comparison, lifecycle, and I/O hooks common across ecosystems.
var runtimeNames = map[string]bool{
	"main": true, "init": true, "constructor": true, "initialize": true, "Main": true, "TestMain": true,
	// Go interfaces satisfied implicitly.
	"String": true, "Error": true, "Format": true, "GoString": true, "Unwrap": true, "Is": true, "As": true,
	"MarshalJSON": true, "UnmarshalJSON": true, "MarshalText": true, "UnmarshalText": true,
	"MarshalYAML": true, "UnmarshalYAML": true, "MarshalBinary": true, "UnmarshalBinary": true,
	"Scan": true, "Value": true, "ServeHTTP": true, "Len": true, "Less": true, "Swap": true,
	"Read": true, "Write": true, "Close": true, "Seek": true, "ReadFrom": true, "WriteTo": true, "Reset": true,
	// JVM, .NET, and JavaScript object protocols and UI lifecycles.
	"toString": true, "equals": true, "hashCode": true, "compareTo": true, "clone": true, "finalize": true,
	"close": true, "run": true, "call": true, "iterator": true, "toJSON": true, "valueOf": true,
	"ToString": true, "Equals": true, "GetHashCode": true, "Dispose": true, "GetEnumerator": true,
	"render": true, "componentDidMount": true, "componentDidUpdate": true, "componentWillUnmount": true,
	"shouldComponentUpdate": true, "getDerivedStateFromProps": true, "componentDidCatch": true,
	"connectedCallback": true, "disconnectedCallback": true, "attributeChangedCallback": true,
	"ngOnInit": true, "ngOnDestroy": true, "ngOnChanges": true, "ngAfterViewInit": true,
	// Ruby and Rust protocols.
	"to_s": true, "to_str": true, "inspect": true, "each": true, "method_missing": true,
	"respond_to_missing?": true, "drop": true, "fmt": true, "from": true, "deref": true,
	"eq": true, "partial_cmp": true, "cmp": true, "next": true, "default": true, "hash": true,
}

func exemptName(name string, inTestFile bool) bool {
	if runtimeNames[name] || (strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")) {
		return true
	}
	return inTestFile && testName.MatchString(name)
}

// debugCalls are calls that stop a program in a debugger or dump state; they
// are never meant to ship.
var debugCalls = map[string]bool{
	"breakpoint": true, "pdb.set_trace": true, "ipdb.set_trace": true, "pudb.set_trace": true,
	"binding.pry": true, "binding.irb": true, "byebug": true, "var_dump": true, "dd": true,
	"dbg": true, "xdebug_break": true, "runtime.Breakpoint": true, "debug.Break": true,
}

// suppressMarkers are linter directives that already record a deliberate
// exception on a line (or the line below one).
var suppressMarkers = []string{
	"noqa", "nolint", "NOLINT", "eslint-disable", "pylint: disable", "pylint:disable",
	"@SuppressWarnings", "rubocop:disable", "#[allow(", "lint:ignore", "review:ignore", "@ts-ignore", "@ts-expect-error",
}

func lastNamed(n *gotreesitter.Node) *gotreesitter.Node {
	if n == nil || n.NamedChildCount() == 0 {
		return nil
	}
	return n.NamedChild(n.NamedChildCount() - 1)
}

func lastLeaf(n *gotreesitter.Node) *gotreesitter.Node {
	for n != nil && n.ChildCount() > 0 {
		next := lastNamed(n)
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}
