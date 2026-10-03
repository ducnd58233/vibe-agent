package reviewscan

import (
	"regexp"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// Rule names, as reports print them.
const (
	ruleSelfComparison   = "self-comparison"
	ruleNaNComparison    = "nan-comparison"
	ruleIdenticalBranch  = "identical-branches"
	ruleDuplicateCond    = "duplicate-condition"
	ruleEmptyHandler     = "empty-error-handler"
	ruleBroadExcept      = "bare-except"
	ruleUnreachable      = "unreachable-code"
	ruleDuplicateKey     = "duplicate-key"
	ruleAssignInCond     = "assignment-in-condition"
	ruleConstantCond     = "constant-condition"
	ruleSelfAssignment   = "self-assignment"
	ruleMutableDefault   = "mutable-default"
	ruleIdentityLiteral  = "identity-literal"
	ruleAssertTuple      = "assert-tuple"
	ruleTypeofTypo       = "typeof-typo"
	ruleReturnInFinally  = "return-in-finally"
	ruleDeferInLoop      = "defer-in-loop"
	ruleDebugger         = "debugger"
	ruleRedefined        = "redefinition"
	ruleUnusedImport     = "unused-import"
	ruleUnreferenced     = "unreferenced"
	ruleTestOnly         = "test-only-reference"
	ruleOrphaned         = "orphaned-by-change"
	ruleRedundantOperand = "redundant-operand"
)

var (
	alwaysTrueOps  = map[string]bool{"==": true, "===": true, "<=": true, ">=": true, "is": true}
	alwaysFalseOps = map[string]bool{"!=": true, "!==": true, "<": true, ">": true}
	redundantOps   = map[string]bool{"&&": true, "||": true, "and": true, "or": true, "&": true, "|": true}
	equalityOps    = map[string]bool{"==": true, "!=": true, "===": true, "!==": true}
	nanOperands    = map[string]bool{
		"NaN": true, "Number.NaN": true, "math.nan": true, "np.nan": true, "numpy.nan": true,
		`float("nan")`: true, `float('nan')`: true, "Double.NaN": true, "Float.NaN": true,
		"f64::NAN": true, "f32::NAN": true, "math.NaN()": true,
	}
	typeofResults = map[string]bool{
		"undefined": true, "object": true, "boolean": true, "number": true,
		"bigint": true, "string": true, "symbol": true, "function": true,
	}
	terminators = map[string]bool{
		"return_statement": true, "throw_statement": true, "raise_statement": true,
		"break_statement": true, "continue_statement": true, "goto_statement": true,
	}
	jumpExpressions = map[string]bool{
		"return_expression": true, "throw_expression": true, "break_expression": true,
		"continue_expression": true, "jump_expression": true,
	}
	// reachableAfterJump are statements a jump does not cut off: labels and
	// case arms are entered from elsewhere, handlers run on their own, and
	// declarations are hoisted or are not executed statements at all.
	reachableAfterJump = []string{
		"label", "case", "default", "else", "elif", "rescue", "ensure", "when", "catch",
		"finally", "except", "function", "class", "method", "interface", "struct", "enum",
		"type", "import", "package", "use_", "preproc",
	}
	handlerTypes = map[string]bool{
		"catch_clause": true, "except_clause": true, "except_group_clause": true, "rescue": true, "catch_block": true,
	}
	blockTypes = map[string]bool{
		"block": true, "statement_block": true, "compound_statement": true, "then": true,
		"body": true, "statements": true, "control_structure_body": true,
	}
	mutableLiterals = map[string]bool{"list": true, "dictionary": true, "set": true}
	pyLiterals      = map[string]bool{
		"string": true, "integer": true, "float": true, "concatenated_string": true,
		"list": true, "dictionary": true, "tuple": true, "set": true,
	}
	boolLiterals = map[string]bool{"true": true, "false": true, "boolean_literal": true, "boolean": true}
	zeroOrOne    = regexp.MustCompile(`^[01]$`)
	errVariable  = regexp.MustCompile(`(?i)^[a-z_]*err[a-z_0-9]*\s*!=\s*nil$`)
)

func ifType(t string) bool { return t == "if" || strings.HasPrefix(t, "if_") }

func (w *walker) text(n *gotreesitter.Node) string { return n.Text(w.src) }

// norm compares code ignoring layout.
func (w *walker) norm(n *gotreesitter.Node) string {
	return strings.Join(strings.Fields(n.Text(w.src)), " ")
}

func (w *walker) typ(n *gotreesitter.Node) string { return n.Type(w.lang) }

func (w *walker) field(n *gotreesitter.Node, name string) *gotreesitter.Node {
	return n.ChildByFieldName(name, w.lang)
}

// hasCall: an operand with a call can differ between evaluations.
func (w *walker) hasCall(n *gotreesitter.Node) bool {
	t := w.typ(n)
	if strings.Contains(t, "call") || strings.Contains(t, "invocation") || strings.Contains(t, "new_expression") || strings.Contains(t, "creation") {
		return true
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if w.hasCall(n.NamedChild(i)) {
			return true
		}
	}
	return false
}

func (w *walker) unparen(n *gotreesitter.Node) *gotreesitter.Node {
	if n != nil && w.typ(n) == "parenthesized_expression" && n.NamedChildCount() == 1 {
		return n.NamedChild(0)
	}
	return n
}

// isEmpty: a body with no statements, or only `pass` / `...`, and no comment
// explaining why.
func (w *walker) isEmpty(body *gotreesitter.Node) bool {
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if !child.IsNamed() {
			continue
		}
		t := w.typ(child)
		if strings.Contains(t, "comment") {
			return false
		}
		if t == "pass_statement" || t == "ellipsis" || (t == "expression_statement" && w.text(child) == "...") {
			continue
		}
		if t == "statement_list" && w.isEmpty(child) {
			continue
		}
		return false
	}
	return true
}

func (w *walker) bodyOf(n *gotreesitter.Node) *gotreesitter.Node {
	if body := w.field(n, "body"); body != nil {
		return body
	}
	for i := n.NamedChildCount() - 1; i >= 0; i-- {
		if child := n.NamedChild(i); blockTypes[w.typ(child)] {
			return child
		}
	}
	return nil
}

// rules runs every shape check on one node. Each check returns early on the
// node types it does not apply to, so the per-node cost is a few comparisons.
func (w *walker) rules(n *gotreesitter.Node, t string) {
	w.binaryRules(n)
	if ifType(t) || t == "conditional_expression" || t == "ternary_expression" {
		w.branchRules(n, t)
	}
	if handlerTypes[t] {
		w.handlerRules(n, t)
	}
	w.unreachable(n)
	w.duplicateKeys(n)
	switch t {
	case "assignment", "assignment_expression", "assignment_statement":
		w.selfAssignment(n)
	case "default_parameter", "typed_default_parameter":
		w.mutableDefault(n)
	case "assert_statement":
		if first := n.NamedChild(0); first != nil && w.typ(first) == "tuple" && first.NamedChildCount() > 0 {
			w.a.add(line(n), ruleAssertTuple, SeverityHigh, "asserting a non-empty tuple is always true; drop the parentheses around the condition and message")
		}
	case "finally_clause":
		w.returnInFinally(n)
	case "defer_statement":
		w.deferInLoop(n)
	case "debugger_statement":
		w.a.add(line(n), ruleDebugger, SeverityMedium, "`debugger` statement left in code")
	case "call", "call_expression", "macro_invocation", "function_call_expression":
		w.debugCall(n)
	}
	if t == "if_statement" {
		w.goEmptyErrCheck(n)
	}
}

// binaryRules covers every binary shape [operand, operator, operand].
func (w *walker) binaryRules(n *gotreesitter.Node) {
	if n.ChildCount() != 3 {
		return
	}
	left, op, right := n.Child(0), n.Child(1), n.Child(2)
	if op.IsNamed() || !left.IsNamed() || !right.IsNamed() {
		return
	}
	operator := w.text(op)
	if equalityOps[operator] && (nanOperands[w.norm(left)] || nanOperands[w.norm(right)]) {
		w.a.add(line(n), ruleNaNComparison, SeverityHigh,
			"comparing with NaN with `%s` is always %v; use an is-NaN check", operator, operator == "!=" || operator == "!==")
		return
	}
	if equalityOps[operator] {
		w.typeofTypo(n, left, right)
	}
	if operator == "is" && (pyLiterals[w.typ(left)] || pyLiterals[w.typ(right)]) {
		w.a.add(line(n), ruleIdentityLiteral, SeverityHigh,
			"`is` compares identity, not value; use `==` to compare with a literal")
	}
	if w.norm(left) != w.norm(right) || w.hasCall(left) {
		return
	}
	expr := w.norm(left)
	switch {
	case alwaysTrueOps[operator]:
		w.a.add(line(n), ruleSelfComparison, SeverityHigh, "`%s %s %s` compares a value with itself and is always true; one side is probably a typo", expr, operator, expr)
	case alwaysFalseOps[operator]:
		w.a.add(line(n), ruleSelfComparison, SeverityMedium, "`%s %s %s` compares a value with itself and is false except for NaN; if this is a NaN check, say so with an is-NaN call", expr, operator, expr)
	case redundantOps[operator]:
		w.a.add(line(n), ruleRedundantOperand, SeverityMedium, "`%s %s %s` repeats the same operand; one side is probably meant to differ", expr, operator, expr)
	}
}

func (w *walker) typeofTypo(n, left, right *gotreesitter.Node) {
	for _, pair := range [][2]*gotreesitter.Node{{left, right}, {right, left}} {
		unary, str := pair[0], pair[1]
		if w.typ(unary) != "unary_expression" || unary.ChildCount() < 2 || w.text(unary.Child(0)) != "typeof" {
			continue
		}
		if !strings.Contains(w.typ(str), "string") {
			continue
		}
		value := strings.Trim(w.text(str), "'\"`")
		if !typeofResults[value] {
			w.a.add(line(n), ruleTypeofTypo, SeverityHigh, "`typeof` never returns %q, so this comparison never matches", value)
		}
	}
}

func (w *walker) branchRules(n *gotreesitter.Node, t string) {
	cons, alt := w.field(n, "consequence"), w.field(n, "alternative")
	if t == "conditional_expression" && cons == nil && n.NamedChildCount() == 3 {
		cons, alt = n.NamedChild(0), n.NamedChild(2) // value if cond else value
	}
	if cons != nil && alt != nil {
		other := alt
		if strings.Contains(w.typ(alt), "else") {
			other = w.bodyOf(alt)
			if other == nil {
				other = lastNamed(alt)
			}
		}
		if other != nil && w.field(other, "condition") == nil {
			a, b := w.norm(cons), w.norm(other)
			if a == b && a != "" && a != "{}" && a != "{ }" {
				w.a.add(line(n), ruleIdenticalBranch, SeverityHigh, "both branches are identical, so the condition decides nothing")
			}
		}
	}
	cond := w.field(n, "condition")
	if cond == nil || strings.Contains(t, "ternary") || t == "conditional_expression" {
		return
	}
	w.assignmentInCondition(n, cond)
	if inner := w.unparen(cond); inner != nil {
		it := w.typ(inner)
		if boolLiterals[it] || (strings.Contains(it, "integer") || strings.Contains(it, "number") || strings.Contains(it, "int_literal")) && zeroOrOne.MatchString(w.text(inner)) {
			w.a.add(line(n), ruleConstantCond, SeverityLow, "the condition is the constant `%s`; one branch is dead code", w.text(inner))
		}
	}
	w.duplicateConditions(n)
}

func (w *walker) assignmentInCondition(n, cond *gotreesitter.Node) {
	if w.typ(cond) != "parenthesized_expression" || cond.NamedChildCount() != 1 {
		return
	}
	inner := cond.NamedChild(0)
	if w.typ(inner) != "assignment_expression" {
		return // doubled parentheses mark an intended assignment
	}
	if right := w.field(inner, "right"); right == nil || w.hasCall(right) {
		return // `if (x = next())` is a deliberate idiom
	}
	w.a.add(line(n), ruleAssignInCond, SeverityHigh, "`%s` assigns inside the condition; did you mean a comparison?", w.norm(inner))
}

// duplicateConditions walks an if / else-if chain from its head and reports a
// condition that repeats an earlier one: that branch can never run.
func (w *walker) duplicateConditions(head *gotreesitter.Node) {
	if w.chained[head.StartByte()] {
		return
	}
	seen := map[string]int{}
	check := func(link *gotreesitter.Node) {
		cond := w.field(link, "condition")
		if cond == nil {
			return
		}
		// `if v, ok := f(); ok` and `if v, ok := g(); ok` test different
		// things: the init statement is part of the condition.
		key := w.norm(w.unparen(cond))
		init := w.field(link, "initializer")
		if init != nil {
			key = w.norm(init) + "; " + key
		}
		if first, ok := seen[key]; ok && !w.hasCall(cond) && (init == nil || !w.hasCall(init)) {
			w.a.add(line(link), ruleDuplicateCond, SeverityHigh, "this condition repeats the one on line %d, so this branch never runs", first)
			return
		}
		seen[key] = line(link)
	}
	for link := head; link != nil; {
		w.chained[link.StartByte()] = true
		check(link)
		var next *gotreesitter.Node
		for i := 0; i < link.NamedChildCount(); i++ {
			child := link.NamedChild(i)
			ct := w.typ(child)
			switch ct {
			case "elif_clause":
				check(child)
			case "elsif": // checked when the walk reaches it
				next = child
			}
		}
		if alt := w.field(link, "alternative"); alt != nil && next == nil {
			if w.field(alt, "condition") != nil && w.typ(alt) != "elif_clause" {
				next = alt
			} else if body := lastNamed(alt); body != nil && ifType(w.typ(body)) {
				next = body
			}
		}
		if next == link {
			break
		}
		link = next
	}
}

func (w *walker) handlerRules(n *gotreesitter.Node, t string) {
	body := w.bodyOf(n)
	if t == "except_clause" && n.NamedChildCount() == 1 && body != nil && !strings.Contains(w.norm(body), "raise") {
		w.a.add(line(n), ruleBroadExcept, SeverityMedium, "a bare `except:` also catches KeyboardInterrupt and SystemExit; name the exceptions")
	}
	if (body == nil && t == "rescue" && n.NamedChildCount() <= 1) || (body != nil && w.isEmpty(body)) {
		w.emptyHandlers = append(w.emptyHandlers, emptyHandler{line(n), endLine(n), "the error is caught and silently dropped; handle it, log it, or comment why ignoring it is safe"})
	}
}

// emptyHandler waits for the end of the pass: a comment on the line before it
// or after it on the same line (visited later) explains it just as one inside does.
type emptyHandler struct {
	start, end int
	message    string
}

func (w *walker) reportEmptyHandlers() {
	for _, h := range w.emptyHandlers {
		if w.commentLines[h.start-1] || w.commentLines[h.start] || w.commentLines[h.end] {
			continue
		}
		w.a.add(h.start, ruleEmptyHandler, SeverityMedium, "%s", h.message)
	}
}

// goEmptyErrCheck: `if err != nil {}` drops the error just as an empty catch does.
func (w *walker) goEmptyErrCheck(n *gotreesitter.Node) {
	cond, cons := w.field(n, "condition"), w.field(n, "consequence")
	if cond == nil || cons == nil || !errVariable.MatchString(w.norm(cond)) {
		return
	}
	if w.isEmpty(cons) {
		w.emptyHandlers = append(w.emptyHandlers, emptyHandler{line(n), endLine(n), "the error is checked and then ignored; handle it or return it"})
	}
}

func (w *walker) jumps(n *gotreesitter.Node) bool {
	t := w.typ(n)
	if terminators[t] || jumpExpressions[t] {
		return true
	}
	return t == "expression_statement" && n.NamedChildCount() == 1 && jumpExpressions[w.typ(n.NamedChild(0))]
}

// unreachable: a statement after a return, throw, break, or continue in the
// same statement list never runs.
func (w *walker) unreachable(n *gotreesitter.Node) {
	jumped := false
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		t := w.typ(child)
		if strings.Contains(t, "comment") {
			continue
		}
		if jumped {
			for _, fragment := range reachableAfterJump {
				if strings.Contains(t, fragment) {
					return
				}
			}
			w.a.add(line(child), ruleUnreachable, SeverityHigh, "this statement follows a return, throw, break, or continue and never runs")
			return
		}
		jumped = w.jumps(child)
	}
}

func (w *walker) keyOf(child *gotreesitter.Node) string {
	if key := w.field(child, "key"); key != nil {
		return w.norm(key)
	}
	switch w.typ(child) {
	case "shorthand_property_identifier":
		return w.text(child)
	case "keyed_element", "array_element_initializer":
		if child.NamedChildCount() == 2 {
			return w.norm(child.NamedChild(0))
		}
	}
	return ""
}

// duplicateKeys: a literal that gives one key twice keeps only the last value.
func (w *walker) duplicateKeys(n *gotreesitter.Node) {
	if n.NamedChildCount() < 2 {
		return
	}
	seen := map[string]int{}
	for i := 0; i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if strings.Contains(w.typ(child), "spread") {
			seen = map[string]int{} // keys after a spread override it on purpose
			continue
		}
		key := w.keyOf(child)
		if key == "" || strings.Contains(key, "(") || strings.HasPrefix(key, "...") {
			continue
		}
		key = strings.Trim(key, "'\"`")
		if first, ok := seen[key]; ok {
			w.a.add(line(child), ruleDuplicateKey, SeverityHigh, "key %q repeats line %d; the earlier value is silently overwritten", key, first)
			continue
		}
		seen[key] = line(child)
	}
}

func (w *walker) selfAssignment(n *gotreesitter.Node) {
	left, right := w.field(n, "left"), w.field(n, "right")
	if left == nil || right == nil || w.hasCall(left) {
		return
	}
	if op := w.field(n, "operator"); op != nil && w.text(op) != "=" {
		return
	}
	for i := 0; i < n.ChildCount(); i++ {
		if child := n.Child(i); !child.IsNamed() && w.text(child) != "=" && strings.HasSuffix(w.text(child), "=") {
			return // a compound assignment such as +=
		}
	}
	if w.norm(left) == w.norm(right) {
		w.a.add(line(n), ruleSelfAssignment, SeverityMedium, "`%s` is assigned to itself; the statement has no effect", w.norm(left))
	}
}

func (w *walker) mutableDefault(n *gotreesitter.Node) {
	if !w.spec.sharedDefaults {
		return
	}
	value := w.field(n, "value")
	if value != nil && mutableLiterals[w.typ(value)] {
		w.a.add(line(n), ruleMutableDefault, SeverityMedium, "the default `%s` is created once and shared by every call; default to None and build it inside", w.text(value))
	}
}

// returnInFinally: a return in a finally block replaces whatever the try
// returned or raised, so exceptions vanish.
func (w *walker) returnInFinally(n *gotreesitter.Node) {
	var find func(node *gotreesitter.Node) *gotreesitter.Node
	find = func(node *gotreesitter.Node) *gotreesitter.Node {
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			t := w.typ(child)
			if t == "return_statement" {
				return child
			}
			if strings.Contains(t, "function") || strings.Contains(t, "lambda") || strings.Contains(t, "class") || strings.Contains(t, "method") || strings.Contains(t, "arrow") {
				continue
			}
			if found := find(child); found != nil {
				return found
			}
		}
		return nil
	}
	if ret := find(n); ret != nil {
		w.a.add(line(ret), ruleReturnInFinally, SeverityHigh, "returning from finally discards any exception or earlier return from the try block")
	}
}

func (w *walker) deferInLoop(n *gotreesitter.Node) {
	if !w.spec.deferAtReturn {
		return
	}
	for i := len(w.types) - 1; i >= 0; i-- {
		t := w.types[i]
		if strings.Contains(t, "func") || strings.Contains(t, "method") {
			return
		}
		if strings.HasPrefix(t, "for") {
			w.a.add(line(n), ruleDeferInLoop, SeverityMedium, "deferred calls run when the function returns, not each iteration; resources pile up until then")
			return
		}
	}
}

func (w *walker) debugCall(n *gotreesitter.Node) {
	callee := w.field(n, "function")
	if callee == nil {
		callee = w.field(n, "macro")
	}
	name := ""
	switch {
	case callee != nil:
		name = w.norm(callee)
	case w.field(n, "receiver") != nil && w.field(n, "method") != nil:
		name = w.norm(w.field(n, "receiver")) + "." + w.norm(w.field(n, "method"))
	case w.field(n, "method") != nil:
		name = w.norm(w.field(n, "method"))
	}
	if debugCalls[name] {
		w.a.add(line(n), ruleDebugger, SeverityMedium, "`%s` stops in a debugger or dumps state; remove it before shipping", name)
	}
}
