// Package graphroute maps toolkit commands to workflow graphs.
//
// Users invoke slash commands; host agents derive slug and graph from the
// command name and the user's objective. Graph ids stay an implementation
// detail rather than a CLI flag non-developers would have to learn.
package graphroute

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

const (
	// GraphDelivery is the default delivery pipeline with human gates.
	GraphDelivery = "goal-delivery"
	// GraphResearcher is the literature → experiment → findings loop.
	GraphResearcher = "researcher-delivery"
	// GraphTask is a non-code task from request to an approved delivery. It has
	// no auto path: the delivery step is the one a person has to approve.
	GraphTask = "task-delivery"
	// GraphStudy is a self-learner's study, one session at a time, resumed when
	// the learner returns. It needs a person, so it has no auto path.
	GraphStudy = "study-delivery"
)

// Command is a toolkit entry surface (/goal, /research, /auto, ...).
type Command string

const (
	CmdGoal       Command = "goal"
	CmdAuto       Command = "auto"
	CmdResearch   Command = "research"
	CmdExperiment Command = "experiment"
	CmdFindings   Command = "findings"
	CmdTask       Command = "task"
	CmdTutor      Command = "tutor"
)

// Workflow selects a graph when no explicit graph override is set.
type Workflow string

const (
	WorkflowDelivery Workflow = "delivery"
	WorkflowResearch Workflow = "research"
	WorkflowTask     Workflow = "task"
	// WorkflowExperiment is the researcher graph entered through an experiment
	// objective. The graph is the same one; the word records what was asked.
	WorkflowExperiment Workflow = "experiment"
	WorkflowTutor      Workflow = "tutor"
)

// Workflows lists every workflow name a command may state, in the order shown
// to a person.
func Workflows() []Workflow {
	return []Workflow{WorkflowDelivery, WorkflowResearch, WorkflowExperiment, WorkflowTask, WorkflowTutor}
}

// ParseWorkflow reports whether word names a workflow.
func ParseWorkflow(word string) (Workflow, bool) {
	for _, w := range Workflows() {
		if strings.EqualFold(word, string(w)) {
			return w, true
		}
	}
	return "", false
}

// DefaultSlugWords bounds a derived slug.
const DefaultSlugWords = 4

// Params is unresolved start input from a command or MCP tool.
type Params struct {
	Command       Command
	Workflow      Workflow
	Goal          string
	Slug          string
	GraphOverride string
	SlugWords     int
	// WithTask marks a delivery objective as also carrying a non-code
	// deliverable, so the task_required flag is set at start.
	WithTask bool
}

// Resolved is start input after graph and slug selection.
type Resolved struct {
	GraphID string
	Slug    string
	Goal    string
	// Workflow is the workflow the graph was chosen for, which is empty when a
	// graph override named it directly.
	Workflow Workflow
	// Flags are run flags to set at start, such as task_required.
	Flags map[string]bool
	// Reason says how the graph was chosen: "named on the command line", or the
	// words that decided it. Shown to the person so a wrong guess is visible.
	Reason string
}

// GraphFor returns the graph id for a toolkit command.
func GraphFor(cmd Command) string {
	switch cmd {
	case CmdResearch, CmdExperiment, CmdFindings:
		return GraphResearcher
	case CmdTask:
		return GraphTask
	case CmdTutor:
		return GraphStudy
	default:
		return GraphDelivery
	}
}

// GraphForWorkflow returns the graph id for a named workflow.
func GraphForWorkflow(w Workflow) string {
	switch w {
	case WorkflowResearch, WorkflowExperiment:
		return GraphResearcher
	case WorkflowTask:
		return GraphTask
	case WorkflowTutor:
		return GraphStudy
	default:
		return GraphDelivery
	}
}

// Resolve picks graph and slug from command context and the objective text.
func (p Params) Resolve() (Resolved, error) {
	goal := strings.TrimSpace(p.Goal)
	if goal == "" {
		return Resolved{}, fmt.Errorf("need a one-line objective")
	}

	graphID := strings.TrimSpace(p.GraphOverride)
	workflow := p.Workflow
	flags := map[string]bool{}
	reason := ""
	switch {
	case graphID != "":
		reason = "named with --graph"
	case workflow != "":
		reason = "named on the command line"
	case p.Command == CmdGoal || p.Command == CmdAuto:
		// No workflow was named, so read the objective.
		detected := Detect(goal)
		if len(detected.Ambiguous) > 0 {
			return Resolved{}, ambiguityError(p.Command, detected)
		}
		// Nobody is at intake on an unattended run to correct a wrong guess,
		// so it acts only on a strong signal for anything but delivery.
		if p.Command == CmdAuto && detected.Workflow != WorkflowDelivery && !detected.Strong {
			return Resolved{}, fmt.Errorf("this looks like %s work (%s), but that is too little to start an unattended run on.\n"+
				"  say which: vibe-agent auto %s \"<objective>\" or vibe-agent auto delivery \"<objective>\"",
				detected.Workflow, detected.Reason, detected.Workflow)
		}
		workflow, flags, reason = detected.Workflow, detected.Flags, detected.Reason
		if flags == nil {
			flags = map[string]bool{}
		}
	}
	if graphID == "" {
		if workflow != "" {
			graphID = GraphForWorkflow(workflow)
		} else {
			graphID = GraphFor(p.Command)
		}
	}
	if p.WithTask {
		flags["task_required"] = true
	}
	if graphID != GraphDelivery {
		// The flag is a guard of the delivery graph only.
		delete(flags, "task_required")
	}
	if p.Command == CmdAuto && graphID == GraphStudy {
		return Resolved{}, fmt.Errorf("a tutor needs a learner present, so it has no unattended form.\n" +
			"  use: vibe-agent goal tutor \"<what to learn>\"")
	}

	words := p.SlugWords
	if words <= 0 {
		words = DefaultSlugWords
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		if r := firstNonASCIILetter(goal); r != 0 {
			return Resolved{}, fmt.Errorf(
				"objective contains %q, so a slug cannot be derived from it safely: "+
					"stripping accents from non-English text leaves bare consonant fragments "+
					"(observed in this repo: l-m-th-n, m-r-ng-repo). "+
					"Restate the objective as a short English gloss, or pass --slug explicitly", r)
		}
		slug = auto.Slugify(goal, words)
	}
	if !validate.Slug(slug) {
		return Resolved{}, fmt.Errorf("%q is not a usable slug; shorten or rephrase the objective", slug)
	}

	return Resolved{GraphID: graphID, Slug: slug, Goal: goal, Workflow: workflow, Flags: flags, Reason: reason}, nil
}

// firstNonASCIILetter returns the first non-ASCII letter in s, or 0 if none.
//
// A slug is derived by keeping only [a-z0-9] and treating everything else as
// a word break (auto.Slugify). Run on text that was never English, every
// accented letter is a break too, so a diacritic-stripped word like "kiểm"
// becomes "ki" and "m" as two separate fragments. The result still matches
// the kebab-case slug pattern, so nothing downstream catches it - this check
// looks at the objective text itself, before that damage happens, rather
// than guess at consonant fragments after the fact.
func firstNonASCIILetter(s string) rune {
	for _, r := range s {
		if r > unicode.MaxASCII && unicode.IsLetter(r) {
			return r
		}
	}
	return 0
}
