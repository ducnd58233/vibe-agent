package graphroute

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Detection is the result of reading an objective that did not name a workflow.
//
// It is deterministic on purpose: the same words give the same graph on every
// run and on every host, and the words that decided it are reported back, so a
// person can see why and override. A classifier a model runs in its head would
// be neither of those.
type Detection struct {
	Workflow Workflow
	// Flags are run flags the objective implies. Today that is task_required,
	// for an objective that is a code change and a non-code deliverable at once.
	Flags map[string]bool
	// Reason names the words that decided it, or says none did.
	Reason string
	// Strong is true when the winning workflow had at least two distinct
	// signals and no other workflow had any. An unattended run acts only on a
	// strong detection, because nobody is at intake to correct a wrong guess.
	Strong bool
	// Ambiguous lists the workflows that tied, when the objective cannot be
	// placed. The caller must ask which one is meant.
	Ambiguous []Workflow
}

// signal is one piece of evidence: a pattern, and the label printed back when
// it matches.
type signal struct {
	label string
	re    *regexp.Regexp
}

func terms(label string, alternatives ...string) signal {
	return signal{label: label, re: regexp.MustCompile(`(?i)\b(?:` + strings.Join(alternatives, "|") + `)\b`)}
}

var (
	// Phrases that say the person wants to learn something. These win outright:
	// an objective that says "teach me to write an API" is about learning, even
	// though it mentions code.
	tutorSignals = []signal{
		terms("teach me", `teach me`, `tutor me`, `be my tutor`),
		terms("help me learn", `help me (?:to )?(?:learn|study|understand)`, `i want to (?:learn|study)`, `i(?:'m| am) trying to learn`),
		terms("quiz me", `quiz me`, `test me on`, `flashcards?`),
		terms("study plan", `study plan`, `self[- ]?study`, `learning plan`, `lesson plan for me`),
	}

	researchSignals = []signal{
		terms("research", `research(?:ing)?`, `literature review`, `systematic review`, `survey of`),
		terms("state of the art", `state[- ]of[- ]the[- ]art`, `what does the evidence say`, `what do the studies say`),
		terms("compare studies", `compare (?:\w+ ){0,3}(?:papers|studies|approaches|methods|models)`),
		terms("investigate", `investigate`),
	}

	experimentSignals = []signal{
		terms("experiment", `experiments?`, `ablations?`),
		terms("benchmark", `benchmark(?:s|ing)?`),
		terms("hypothesis", `hypothes[ie]s`, `hyperparameters?`),
		terms("a/b test", `a/b test(?:s|ing)?`),
		terms("train and evaluate", `train and evaluate`, `fine[- ]?tun(?:e|ing)`),
	}

	// A non-code deliverable is a verb that produces something for a reader,
	// followed within a few words by what it produces. The verb alone proves
	// nothing ("send" is also a code word), and the noun alone proves nothing
	// ("email" is also a database column).
	taskSignals = []signal{
		{label: "a document to write", re: regexp.MustCompile(`(?i)\b(?:write|draft|compose|prepare|proofread|summari[sz]e|translate|rewrite)\b(?:\W+\w+){0,6}?\W+(?:email|e-mail|report|memo|letter|summary|proposal|presentation|slides|deck|invoice|contract|agenda|newsletter|announcement|resume|cover letter|spreadsheet|policy|minutes|notes|essay|article|brief)\b`)},
		{label: "a message to send", re: regexp.MustCompile(`(?i)\b(?:send|reply to|respond to|email|message|notify|announce to)\b(?:\W+\w+){0,5}?\W+(?:customer|client|team|manager|vendor|supplier|account manager|everyone|stakeholders?|recipients?)\b`)},
		{label: "data to reconcile", re: regexp.MustCompile(`(?i)\b(?:reconcile|clean(?: up)?|deduplicate|merge)\b(?:\W+\w+){0,4}?\W+(?:spreadsheet|csv|invoices?|expenses|ledger|contacts|records|rows)\b`)},
		terms("a meeting or schedule", `meeting notes`, `agenda for`, `schedule a`, `itinerary`),
		terms("a plan or budget", `budget for`, `plan (?:a|my|our) (?:trip|event|launch|move)`, `forecast for`),
	}

	// Words that say the objective changes software. Their presence keeps an
	// objective on the delivery graph, and next to a non-code signal it marks a
	// mixed objective.
	codeSignals = []signal{
		terms("code", `code`, `codebase`, `repo`, `repository`, `pull request`, `refactor(?:ing)?`, `bugs?`),
		terms("software term", `endpoint`, `api`, `function`, `module`, `library`, `sdk`, `cli`, `runtime`, `backend`, `frontend`, `webhook`, `schema`, `migration`, `database`, `sql`),
		terms("tests or build", `unit tests?`, `integration tests?`, `test suite`, `compile`, `lint`, `deploy(?:ment)?`, `docker`, `kubernetes`, `ci pipeline`),
		terms("a language", `typescript`, `javascript`, `python`, `golang`, `rust`, `kotlin`, `react`, `node(?:\.js)?`),
		terms("implement", `implement(?:ing|ation)?`, `fix(?:ing)? (?:the |a )?(?:bug|crash|error|failure)`),
	}
)

func match(set []signal, text string) []string {
	var hit []string
	seen := map[string]bool{}
	for _, s := range set {
		for _, m := range s.re.FindAllString(text, -1) {
			term := strings.ToLower(strings.TrimSpace(m))
			if !seen[term] {
				seen[term] = true
				hit = append(hit, term)
			}
		}
	}
	return hit
}

func quote(hits []string) string {
	q := make([]string, len(hits))
	for i, h := range hits {
		q[i] = `"` + h + `"`
	}
	return strings.Join(q, ", ")
}

// Detect reads an objective and says which workflow it is, with the words that
// decided it.
//
// The order of the rules is the design:
//
//  1. A learning phrase wins. The person wants to learn, whatever the subject.
//  2. Code words with a non-code deliverable is a mixed objective: the delivery
//     graph, with the task_required flag so the non-code part is delivered last.
//  3. Code words alone, or no words at all, is delivery, the default and the
//     behaviour before routing existed.
//  4. Research and experiment words without code words go to the researcher
//     graph. A task word beside them is a tie, and the person is asked.
//  5. Task words alone are task work.
func Detect(goal string) Detection {
	tutor := match(tutorSignals, goal)
	research := append(match(researchSignals, goal), match(experimentSignals, goal)...)
	task := match(taskSignals, goal)
	code := match(codeSignals, goal)

	switch {
	case len(tutor) > 0:
		return Detection{Workflow: WorkflowTutor, Strong: len(tutor) >= 2 && len(code)+len(research)+len(task) == 0,
			Reason: "learning words " + quote(tutor)}

	case len(code) > 0 && len(task) > 0:
		return Detection{Workflow: WorkflowDelivery, Flags: map[string]bool{"task_required": true}, Strong: true,
			Reason: "code words " + quote(code) + " and a non-code deliverable " + quote(task) +
				", so the non-code part runs after the last code task"}

	case len(code) > 0:
		return Detection{Workflow: WorkflowDelivery, Strong: true, Reason: "code words " + quote(code)}

	case len(research) > 0 && len(task) > 0:
		return Detection{Ambiguous: []Workflow{WorkflowResearch, WorkflowTask},
			Reason: "research words " + quote(research) + " and a non-code deliverable " + quote(task)}

	case len(research) > 0:
		wf := WorkflowResearch
		if len(match(experimentSignals, goal)) > 0 {
			wf = WorkflowExperiment
		}
		return Detection{Workflow: wf, Strong: len(research) >= 2, Reason: "research words " + quote(research)}

	case len(task) > 0:
		return Detection{Workflow: WorkflowTask, Strong: len(task) >= 2, Reason: "a non-code deliverable " + quote(task)}
	}
	return Detection{Workflow: WorkflowDelivery, Strong: false, Reason: "no routing words found, so the default delivery graph"}
}

// ambiguityError tells the caller how to say which workflow is meant.
func ambiguityError(command Command, d Detection) error {
	names := make([]string, len(d.Ambiguous))
	for i, w := range d.Ambiguous {
		names[i] = string(w)
	}
	sort.Strings(names)
	verb := string(command)
	return fmt.Errorf("the objective could be more than one kind of work (%s). Say which: %s.\n"+
		"  for example: vibe-agent %s %s \"<objective>\"",
		d.Reason, strings.Join(names, " or "), verb, names[0])
}
