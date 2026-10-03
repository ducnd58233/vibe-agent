package reviewscan

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	enry "github.com/go-enry/go-enry/v2"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const kindTopLevel = "top-level"

// analysis is everything one file contributes: its blocks and local findings
// when it is code, and the names it mentions either way.
type analysis struct {
	file     File
	code     bool
	parsed   bool
	findings []Finding
	lines    []string

	// idents counts identifier occurrences outside imports and outside the
	// names that definitions and imports introduce. importRefs counts the
	// identifiers inside imports: an import of a name is a use of whatever
	// defines it, though not a use inside this file.
	idents     map[string]int
	importRefs map[string]int
	// mentions are words inside string literals, or anywhere in a non-code
	// text file (a template, a config): a name there may be used by lookup.
	mentions map[string]bool
	defs     []definition
}

// definition is a named block that the cross-file pass checks for references.
type definition struct {
	name     string
	block    int
	line     int
	private  bool
	exempt   bool
	selfRefs int
}

func (a *analysis) suppressed(lineNo int) bool {
	for _, n := range []int{lineNo, lineNo - 1} {
		if n < 1 || n > len(a.lines) {
			continue
		}
		for _, marker := range suppressMarkers {
			if strings.Contains(a.lines[n-1], marker) {
				return true
			}
		}
	}
	return false
}

func (a *analysis) hasOrphanFinding() bool {
	for _, f := range a.findings {
		if f.Rule == ruleOrphaned {
			return true
		}
	}
	return false
}

func (a *analysis) add(lineNo int, rule string, severity Severity, format string, args ...any) {
	code := ""
	if lineNo >= 1 && lineNo <= len(a.lines) {
		code = strings.TrimSpace(a.lines[lineNo-1])
		if len(code) > maxCodeChars {
			code = strings.ToValidUTF8(code[:maxCodeChars], "") + "..."
		}
	}
	a.findings = append(a.findings, Finding{
		Path: a.file.Path, Line: lineNo, Rule: rule, Severity: severity,
		Message: fmt.Sprintf(format, args...), Code: code,
	})
}

const maxCodeChars = 160

var wordPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func addWords(set map[string]bool, text string) {
	for _, word := range wordPattern.FindAllString(text, -1) {
		set[word] = true
	}
}

// parserCache keeps one parser per language for one worker goroutine.
type parserCache map[string]*gotreesitter.Parser

func (c parserCache) parse(entry *grammars.LangEntry, src []byte) (*gotreesitter.Tree, *gotreesitter.Language, error) {
	lang := entry.Language()
	parser, ok := c[entry.Name]
	if !ok {
		parser = gotreesitter.NewParser(lang)
		c[entry.Name] = parser
	}
	if entry.TokenSourceFactory != nil {
		tree, err := parser.ParseWithTokenSource(src, entry.TokenSourceFactory(src, lang))
		return tree, lang, err
	}
	tree, err := parser.Parse(src)
	return tree, lang, err
}

func analyze(rel string, data []byte, parsers parserCache) *analysis {
	text := string(data)
	a := &analysis{
		lines:      strings.Split(text, "\n"),
		idents:     map[string]int{},
		importRefs: map[string]int{},
		mentions:   map[string]bool{},
	}
	language := enry.GetLanguage(rel, data)
	a.file = File{Path: rel, Language: language, Lines: len(a.lines), Test: testPath.MatchString(rel)}
	switch enry.GetLanguageType(language) {
	case enry.Prose:
		return a
	case enry.Programming:
		a.code = true
	default:
		addWords(a.mentions, text)
		return a
	}

	entry := grammars.DetectLanguage(rel)
	if entry == nil {
		entry = grammars.DetectLanguageByShebang(a.lines[0])
	}
	var tree *gotreesitter.Tree
	var lang *gotreesitter.Language
	if entry != nil {
		var err error
		if tree, lang, err = parsers.parse(entry, data); err != nil {
			tree = nil
		}
	}
	if tree == nil || tree.RootNode() == nil {
		// No grammar: still index the words so its references count, and
		// hand the reviewer the whole file as one block.
		addWords(a.mentions, text)
		a.file.Blocks = topLevelBlocks(a.lines, nil)
		return a
	}
	defer tree.Release()
	a.parsed = true
	a.file.Parsed = true
	if root := tree.RootNode(); root.HasError() {
		a.file.SyntaxError = firstError(root)
	}

	w := &walker{
		a: a, lang: lang, src: data, spec: specs[entry.Name],
		nameLeaves: map[uint32]bool{}, occ: map[string][]int{},
		chained: map[uint32]bool{}, comments: map[string]bool{}, commentLines: map[int]bool{},
	}
	w.visit(tree.RootNode())
	w.finish()
	return a
}

// walker is one pre-order pass over a file's syntax tree.
type walker struct {
	a    *analysis
	lang *gotreesitter.Language
	src  []byte
	spec spec

	stack []*gotreesitter.Node
	types []string

	// nameLeaves are identifier leaves that introduce a name (a definition's
	// name, an import's binding) rather than use one.
	nameLeaves map[uint32]bool
	occ        map[string][]int
	chained    map[uint32]bool
	comments   map[string]bool
	// commentLines are lines holding any part of a comment.
	commentLines  map[int]bool
	emptyHandlers []emptyHandler
	bindings      []binding
	raw           []rawBlock
	open          []int

	importDepth int
	errorDepth  int
	module      bool
	sawJSX      bool
}

type rawBlock struct {
	kind, name string
	start, end int
	depth      int
	parent     int
	scopeKey   string
	exported   bool
	decorated  bool
	broken     bool // the node holds a parse error, so its extent is a guess
	private    bool
	topLevel   bool
}

func (w *walker) visit(n *gotreesitter.Node) {
	t := n.Type(w.lang)
	named := n.IsNamed()
	if named && strings.Contains(t, "comment") {
		addWords(w.comments, n.Text(w.src))
		for l := line(n); l <= endLine(n); l++ {
			w.commentLines[l] = true
		}
		return
	}
	if n.IsError() {
		w.errorDepth++
		defer func() { w.errorDepth-- }()
	}
	opened := false
	if named {
		if strings.HasPrefix(t, "jsx") {
			w.sawJSX = true
		}
		if w.spec.importTypes[t] || t == "export_statement" {
			w.module = true
		}
		if w.spec.importTypes[t] && w.spec.binder != nil {
			w.bindings = append(w.bindings, w.spec.binder(w, n, t)...)
			w.importDepth++
			defer func() { w.importDepth-- }()
		}
		if w.spec.requireCall && t == "variable_declarator" && w.atTopLevel() {
			if bound := requireBindings(w, n); len(bound) > 0 {
				w.bindings = append(w.bindings, bound...)
				w.importDepth++
				defer func() { w.importDepth-- }()
			}
		}
		// A module path is not a use of a name, so `import lodash from
		// "lodash"` must not count its own string as using lodash.
		if strings.Contains(t, "string") && !w.parentIs("string") && w.importDepth == 0 {
			addWords(w.a.mentions, n.Text(w.src))
		}
		if w.errorDepth == 0 {
			w.rules(n, t)
		}
		opened = w.openBlock(n, t)
	}
	count := n.ChildCount()
	if count == 0 {
		if named && identifierType(t) {
			w.ident(n)
		}
	} else {
		w.stack = append(w.stack, n)
		w.types = append(w.types, t)
		for i := 0; i < count; i++ {
			w.visit(n.Child(i))
		}
		w.stack = w.stack[:len(w.stack)-1]
		w.types = w.types[:len(w.types)-1]
	}
	if opened {
		w.open = w.open[:len(w.open)-1]
	}
}

// firstError is the line of the first parse error under n.
func firstError(n *gotreesitter.Node) int {
	if n.IsError() || n.IsMissing() {
		return line(n)
	}
	for i := 0; i < n.ChildCount(); i++ {
		if child := n.Child(i); child.HasError() || child.IsMissing() {
			return firstError(child)
		}
	}
	return line(n)
}

func identifierType(t string) bool {
	switch t {
	case "identifier", "name", "constant", "word":
		return true
	}
	return strings.HasSuffix(t, "_identifier")
}

func (w *walker) ident(n *gotreesitter.Node) {
	if w.nameLeaves[n.StartByte()] {
		return
	}
	name := strings.TrimPrefix(n.Text(w.src), "$")
	if name == "" {
		return
	}
	if w.importDepth > 0 {
		w.a.importRefs[name]++
		return
	}
	w.occ[name] = append(w.occ[name], line(n))
}

// markName records every identifier leaf under n as introducing a name.
func (w *walker) markName(n *gotreesitter.Node) {
	if n.ChildCount() == 0 {
		w.nameLeaves[n.StartByte()] = true
		return
	}
	for i := 0; i < n.ChildCount(); i++ {
		w.markName(n.Child(i))
	}
}

func (w *walker) parentIs(fragment string) bool {
	return len(w.types) > 0 && strings.Contains(w.types[len(w.types)-1], fragment)
}

func (w *walker) atTopLevel() bool {
	for _, t := range w.types {
		if strings.Contains(t, "function") || strings.Contains(t, "method") || strings.Contains(t, "class") || strings.Contains(t, "arrow") {
			return false
		}
	}
	return true
}

func (w *walker) inside(fragment string) bool {
	for _, t := range w.types {
		if strings.Contains(t, fragment) {
			return true
		}
	}
	return false
}

func line(n *gotreesitter.Node) int { return int(n.StartPoint().Row) + 1 }

// endLine is the last line a node occupies: a node ending at column 0 ends
// on the line before.
func endLine(n *gotreesitter.Node) int {
	end := n.EndPoint()
	if end.Column == 0 && end.Row > n.StartPoint().Row {
		return int(end.Row)
	}
	return int(end.Row) + 1
}

// containerKinds are definition kinds whose direct function members are methods.
var containerKinds = map[string]bool{
	"class": true, "struct": true, "interface": true, "trait": true, "impl": true, "object": true, "enum": true, "record": true,
}

// blockWords map a definition node type to its kind. A type qualifies when it
// is one of these words followed by a definition suffix, so the same table
// reads function_definition (Python, C), function_declaration (JS, Go,
// Kotlin), function_item (Rust), and class_specifier (C++).
var blockWords = []struct{ word, kind string }{
	{"constructor", "constructor"}, {"method", "method"}, {"function", "function"},
	{"interface", "interface"}, {"class", "class"}, {"struct", "struct"}, {"enum", "enum"},
	{"trait", "trait"}, {"impl", "impl"}, {"object", "object"}, {"record", "record"},
	{"protocol", "protocol"}, {"module", "module"}, {"namespace", "namespace"}, {"union", "union"},
}

var blockSuffixes = []string{"_definition", "_declaration", "_item", "_specifier"}

func blockKind(w *walker, n *gotreesitter.Node, t string) string {
	switch t {
	case "method", "singleton_method":
		return "method"
	case "class", "module":
		if len(w.stack) > 0 { // a root node named "module" is a whole Python file
			return t
		}
		return ""
	case "type_spec":
		return "type"
	case "variable_declarator", "public_field_definition", "field_definition":
		if value := n.ChildByFieldName("value", w.lang); value != nil && functionValue(value.Type(w.lang)) {
			if t == "variable_declarator" {
				return "function"
			}
			return "method"
		}
		return ""
	}
	for _, suffix := range blockSuffixes {
		if !strings.HasSuffix(t, suffix) {
			continue
		}
		head := strings.TrimSuffix(t, suffix)
		for _, bw := range blockWords {
			// The word must end the head: generator_function and
			// abstract_class qualify, namespace_use (an import) does not.
			if head != bw.word && !strings.HasSuffix(head, "_"+bw.word) {
				continue
			}
			// A C struct or enum specifier names a type at every use; only
			// the one with a body defines it.
			if suffix == "_specifier" && n.ChildByFieldName("body", w.lang) == nil {
				return ""
			}
			return bw.kind
		}
	}
	return ""
}

func functionValue(t string) bool {
	return t == "arrow_function" || strings.Contains(t, "function_expression") || t == "function" || t == "generator_function"
}

func (w *walker) openBlock(n *gotreesitter.Node, t string) bool {
	kind := blockKind(w, n, t)
	if kind == "" {
		return false
	}
	nameNode := w.nameNode(n)
	name := ""
	prefix := ""
	if nameNode != nil {
		name = strings.TrimSpace(nameNode.Text(w.src))
		prefix = string(w.src[n.StartByte():nameNode.StartByte()])
		w.markName(nameNode)
	}
	parent := -1
	if len(w.open) > 0 {
		parent = w.open[len(w.open)-1]
	}
	if kind == "function" && parent >= 0 && containerKinds[w.raw[parent].kind] {
		kind = "method"
	}
	if receiver := n.ChildByFieldName("receiver", w.lang); receiver != nil && name != "" {
		if typ := firstOfType(w, receiver, "type_identifier"); typ != "" {
			name = typ + "." + name
		}
	}
	scopeKey := ""
	if len(w.stack) > 0 {
		p := w.stack[len(w.stack)-1]
		scopeKey = fmt.Sprintf("%d:%s", p.StartByte(), strings.Join(strings.Fields(prefix), " "))
	}
	block := rawBlock{
		kind: kind, name: name, start: line(n), end: endLine(n), broken: n.HasError(),
		depth: len(w.open), parent: parent, scopeKey: scopeKey,
		exported:  w.inside("export"),
		decorated: w.decorated(n),
		topLevel:  parent < 0,
	}
	if nameNode != nil {
		block.private = w.private(n, t, name, prefix, parent)
	}
	w.raw = append(w.raw, block)
	w.open = append(w.open, len(w.raw)-1)
	return true
}

// nameNode finds what a definition is called: its name field, the identifier
// inside a C declarator, an impl's type, or its first identifier child.
func (w *walker) nameNode(n *gotreesitter.Node) *gotreesitter.Node {
	if name := n.ChildByFieldName("name", w.lang); name != nil {
		return name
	}
	for d := n.ChildByFieldName("declarator", w.lang); d != nil; d = d.ChildByFieldName("declarator", w.lang) {
		if identifierType(d.Type(w.lang)) || strings.Contains(d.Type(w.lang), "qualified") {
			return d
		}
	}
	if typ := n.ChildByFieldName("type", w.lang); typ != nil {
		return typ
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if child := n.NamedChild(i); identifierType(child.Type(w.lang)) {
			return child
		}
	}
	return nil
}

func firstOfType(w *walker, n *gotreesitter.Node, want string) string {
	if n.Type(w.lang) == want {
		return n.Text(w.src)
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if found := firstOfType(w, n.NamedChild(i), want); found != "" {
			return found
		}
	}
	return ""
}

// decoratorType names the nodes that attach framework metadata. It is a list,
// not a substring match: type_annotation (a return type) and attribute (Python
// member access) would otherwise read as decorators.
func decoratorType(t string) bool {
	switch t {
	case "annotation", "marker_annotation", "attribute_item", "attribute_list", "attributes", "attribute":
		return true
	}
	return strings.Contains(t, "decorator")
}

// decorated: a decorator, annotation, or attribute registers a definition
// with a framework, which then calls it without naming it.
func (w *walker) decorated(n *gotreesitter.Node) bool {
	if w.parentIs("decorated") {
		return true
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		ct := child.Type(w.lang)
		if ct != "attribute" && decoratorType(ct) {
			return true
		}
		if ct == "modifiers" {
			for j := 0; j < child.NamedChildCount(); j++ {
				if decoratorType(child.NamedChild(j).Type(w.lang)) {
					return true
				}
			}
		}
	}
	if len(w.stack) > 0 {
		parent := w.stack[len(w.stack)-1]
		var prev *gotreesitter.Node
		for i := 0; i < parent.NamedChildCount(); i++ {
			child := parent.NamedChild(i)
			if child.StartByte() == n.StartByte() {
				break
			}
			if !strings.Contains(child.Type(w.lang), "comment") {
				prev = child
			}
		}
		if prev != nil && prev.Type(w.lang) != "attribute" && decoratorType(prev.Type(w.lang)) {
			return true
		}
	}
	return false
}

var (
	privateWord = regexp.MustCompile(`\b(private|fileprivate)\b`)
	staticWord  = regexp.MustCompile(`\bstatic\b`)
)

// private reports a definition that nothing outside its file or package can
// reach, so no reference in this repository means none anywhere.
func (w *walker) private(n *gotreesitter.Node, t, name, prefix string, parent int) bool {
	bare := name
	if i := strings.LastIndexByte(bare, '.'); i >= 0 {
		bare = bare[i+1:]
	}
	switch {
	case strings.HasPrefix(bare, "__") && strings.HasSuffix(bare, "__"):
		return false
	case strings.HasPrefix(bare, "_") || strings.HasPrefix(bare, "#"):
		return true
	case privateWord.MatchString(prefix):
		return true
	case w.spec.exportByCase:
		r, _ := utf8.DecodeRuneInString(bare)
		return unicode.IsLower(r)
	case w.spec.exportByPub:
		if strings.Contains(prefix, "pub") {
			return false
		}
		// A method in an impl may implement a trait, which never says `pub`;
		// count it as reachable.
		for i := parent; i >= 0; i = w.raw[i].parent {
			if w.raw[i].kind == "impl" {
				return false
			}
		}
		return t == "function_item" || t == "struct_item" || t == "enum_item"
	case w.spec.staticIsLocal:
		return parent < 0 && staticWord.MatchString(prefix)
	}
	return false
}

// finish turns the pass's raw material into blocks, definitions, and the
// findings that need the whole file.
func (w *walker) finish() {
	a := w.a
	for name, lines := range w.occ {
		a.idents[name] = len(lines)
	}
	blocks := make([]Block, 0, len(w.raw))
	for i, raw := range w.raw {
		if w.spec.esModule && w.module && raw.topLevel && !raw.exported && raw.name != "" {
			raw.private = true
			w.raw[i] = raw
		}
		blocks = append(blocks, Block{Kind: raw.kind, Name: raw.name, Start: raw.start, End: raw.end, Depth: raw.depth})
	}
	// Definitions arrive in pre-order, which is already reading order; the
	// top-level stretches merge in around them, and position tracks where
	// each definition lands.
	position := make([]int, len(blocks))
	merged := make([]Block, 0, len(blocks)+4)
	gaps := topLevelBlocks(a.lines, blocks)
	next := 0
	for i, b := range blocks {
		for next < len(gaps) && gaps[next].Start < b.Start {
			merged = append(merged, gaps[next])
			next++
		}
		position[i] = len(merged)
		merged = append(merged, b)
	}
	a.file.Blocks = append(merged, gaps[next:]...)

	for i, raw := range w.raw {
		if raw.name == "" || raw.kind == "impl" || raw.kind == "namespace" || raw.kind == "module" {
			continue
		}
		bare := raw.name
		if dot := strings.LastIndexByte(bare, '.'); dot >= 0 {
			bare = bare[dot+1:]
		}
		self := 0
		for _, l := range w.occ[bare] {
			if raw.broken {
				break
			}
			if l >= raw.start && l <= raw.end {
				self++
			}
		}
		a.defs = append(a.defs, definition{
			name: bare, block: position[i], line: raw.start,
			private: raw.private, exempt: raw.decorated || raw.broken || exemptName(bare, a.file.Test), selfRefs: self,
		})
	}
	w.redefinitions()
	w.unusedImports()
	w.reportEmptyHandlers()
}

// topLevelBlocks covers the lines no top-level definition holds, one block per
// stretch that has any non-blank line, so the blocks together leave nothing
// unread.
func topLevelBlocks(lines []string, defs []Block) []Block {
	covered := make([]bool, len(lines)+2)
	for _, d := range defs {
		if d.Depth != 0 {
			continue
		}
		for l := d.Start; l <= d.End && l <= len(lines); l++ {
			covered[l] = true
		}
	}
	var out []Block
	start := 0
	flush := func(end int) {
		for start <= end && strings.TrimSpace(lines[start-1]) == "" {
			start++
		}
		for end >= start && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		if start > 0 && start <= end {
			out = append(out, Block{Kind: kindTopLevel, Start: start, End: end})
		}
		start = 0
	}
	for l := 1; l <= len(lines); l++ {
		if covered[l] {
			if start > 0 {
				flush(l - 1)
			}
			continue
		}
		if start == 0 {
			start = l
		}
	}
	if start > 0 {
		flush(len(lines))
	}
	return out
}

// redefinitions: in a language where a later definition replaces an earlier
// one, two undecorated definitions of a name in one scope leave the first dead.
func (w *walker) redefinitions() {
	if !w.spec.redefines {
		return
	}
	seen := map[string]int{}
	for _, raw := range w.raw {
		if raw.name == "" || raw.decorated || raw.scopeKey == "" {
			continue
		}
		key := raw.scopeKey + "\x00" + raw.name
		if first, ok := seen[key]; ok {
			w.a.add(raw.start, ruleRedefined, SeverityHigh,
				"`%s` is defined again here; the definition on line %d is replaced and never runs", raw.name, first)
			continue
		}
		seen[key] = raw.start
	}
}

func (w *walker) unusedImports() {
	if len(w.bindings) == 0 || strings.HasSuffix(w.a.file.Path, "__init__.py") {
		return
	}
	for _, b := range w.bindings {
		if len(w.occ[b.name]) > 0 || w.a.mentions[b.name] {
			continue
		}
		if w.sawJSX && b.name == w.spec.jsxImplies {
			continue
		}
		if w.comments[b.name] {
			w.a.add(b.line, ruleUnusedImport, SeverityLow,
				"`%s` is imported but only mentioned in comments or docs; remove it unless a doc tool resolves it", b.name)
			continue
		}
		w.a.add(b.line, ruleUnusedImport, SeverityMedium,
			"`%s` is imported but never used in this file; remove it unless the import is for its side effects", b.name)
	}
}
