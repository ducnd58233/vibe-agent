// Package run assembles the run-state module and is the only place that knows
// all of its parts.
//
// A run is the record of one delivery loop: which node it sits at, what
// evidence has been recorded, and in what order things happened. The rule the
// module exists to hold is that a check is passed only when real evidence
// produced it, and CheckSource has no value for model assertion, so no code path
// lets model output mark its own work complete.
package run

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/run/infra/persistence"
)

// The names a caller uses. One definition each, in the layer that owns it.
type (
	Run         = domain.Run
	Check       = domain.Check
	CheckSource = domain.CheckSource
	Status      = domain.Status
	Blocker     = domain.Blocker
	Artifact    = domain.Artifact
	Event       = domain.Event
)

// The provenance a check may claim, and the states a run may hold. There is
// deliberately no source for model assertion.
const (
	SchemaVersion = domain.SchemaVersion

	SourceExitCode   = domain.SourceExitCode
	SourceFileAssert = domain.SourceFileAssert
	SourceCIAPI      = domain.SourceCIAPI
	SourceHumanEvent = domain.SourceHumanEvent

	StatusRunning        = domain.StatusRunning
	StatusAwaitingHuman  = domain.StatusAwaitingHuman
	StatusDone           = domain.StatusDone
	StatusFailed         = domain.StatusFailed
	StatusCancelled      = domain.StatusCancelled
	StatusBudgetExceeded = domain.StatusBudgetExceeded

	EventLogName = domain.EventLogName
	RunsDirName  = persistence.RunsDirName
)

// Creating a run is a domain act; reading and writing one is I/O.
var (
	NewRun = domain.NewRun

	Load         = persistence.Load
	Save         = persistence.Save
	RunDir       = persistence.RunDir
	RunsDir      = persistence.RunsDir
	ManifestPath = persistence.ManifestPath
	EventLogPath = persistence.EventLogPath
	AppendEvent  = persistence.AppendEvent
	ReadEvents   = persistence.ReadEvents
	List         = persistence.List
)
