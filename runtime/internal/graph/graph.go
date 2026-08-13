// Package graph assembles the workflow-graph module.
//
// A graph is the executable form of a delivery loop: which node runs next, and
// on what evidence. Edge conditions are guard names rather than expressions, and
// guards resolve from run state, so a transition happens because something was
// recorded and never because a model said so.
package graph

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph/infra"
)

// The names a caller uses. One definition each, in the layer that owns it.
type (
	Graph          = domain.Graph
	Spec           = domain.Spec
	Metadata       = domain.Metadata
	Node           = domain.Node
	Edge           = domain.Edge
	Guard          = domain.Guard
	NodeType       = domain.NodeType
	GuardSource    = domain.GuardSource
	VerifierKind   = domain.VerifierKind
	TerminalStatus = domain.TerminalStatus
	ProblemsError  = domain.ProblemsError
)

// The node kinds, guard sources, verifier kinds and terminal statuses, each
// defined once in the domain that gives them meaning.
const (
	NodeAgent     = domain.NodeAgent
	NodeArtifact  = domain.NodeArtifact
	NodeVerifier  = domain.NodeVerifier
	NodeHumanGate = domain.NodeHumanGate
	NodeTerminal  = domain.NodeTerminal

	VerifierCommand = domain.VerifierCommand
	VerifierFiles   = domain.VerifierFiles
	VerifierGit     = domain.VerifierGit
	VerifierScreen  = domain.VerifierScreen

	GuardFlag    = domain.GuardFlag
	GuardCheck   = domain.GuardCheck
	GuardResult  = domain.GuardResult
	GuardRuntime = domain.GuardRuntime

	TerminalDone      = domain.TerminalDone
	TerminalFailed    = domain.TerminalFailed
	TerminalCancelled = domain.TerminalCancelled
)

// Loading a graph is I/O, so it lives in infra and is reached through here.
var (
	Parse      = infra.Parse
	Load       = infra.Load
	LoadDir    = infra.LoadDir
	LoadByID   = infra.LoadByID
	DefaultDir = infra.DefaultDir
)
