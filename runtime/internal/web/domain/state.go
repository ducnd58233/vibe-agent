package domain

import "time"

// State is written to .agent-state/web.json while the server runs.
type State struct {
	URL       string    `json:"url"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}
