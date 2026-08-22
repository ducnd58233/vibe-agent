// Package graphroute maps toolkit commands to workflow graphs.
//
// Users invoke slash commands; host agents derive slug and graph from the
// command name and the user's objective. Graph ids stay an implementation
// detail rather than a CLI flag non-developers would have to learn.
package graphroute

import (
	"fmt"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

const (
	// GraphDelivery is the default delivery pipeline with human gates.
	GraphDelivery = "goal-delivery"
	// GraphResearcher is the literature → experiment → findings loop.
	GraphResearcher = "researcher-delivery"
)

// Command is a toolkit entry surface (/goal, /research, /auto, ...).
type Command string

const (
	CmdGoal       Command = "goal"
	CmdAuto       Command = "auto"
	CmdResearch   Command = "research"
	CmdExperiment Command = "experiment"
	CmdFindings   Command = "findings"
)

// Workflow selects a graph when no explicit graph override is set.
type Workflow string

const (
	WorkflowDelivery Workflow = "delivery"
	WorkflowResearch Workflow = "research"
)

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
}

// Resolved is start input after graph and slug selection.
type Resolved struct {
	GraphID string
	Slug    string
	Goal    string
}

// GraphFor returns the graph id for a toolkit command.
func GraphFor(cmd Command) string {
	switch cmd {
	case CmdResearch, CmdExperiment, CmdFindings:
		return GraphResearcher
	default:
		return GraphDelivery
	}
}

// GraphForWorkflow returns the graph id for a named workflow.
func GraphForWorkflow(w Workflow) string {
	switch w {
	case WorkflowResearch:
		return GraphResearcher
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
	if graphID == "" {
		if p.Workflow != "" {
			graphID = GraphForWorkflow(p.Workflow)
		} else {
			graphID = GraphFor(p.Command)
		}
	}

	words := p.SlugWords
	if words <= 0 {
		words = DefaultSlugWords
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		slug = auto.Slugify(goal, words)
	}
	if !validate.Slug(slug) {
		return Resolved{}, fmt.Errorf("%q is not a usable slug; shorten or rephrase the objective", slug)
	}

	return Resolved{GraphID: graphID, Slug: slug, Goal: goal}, nil
}
