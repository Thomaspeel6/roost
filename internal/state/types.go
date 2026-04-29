// Package state classifies a stream of CC hook events into per-session Agent state.
package state

import "time"

// Status of a CC session at the time of classification.
type Status int

const (
	Running Status = iota
	Idle
	Done
	Blocked
)

func (s Status) String() string {
	return [...]string{"running", "idle", "done", "blocked"}[s]
}

// Agent is the classified view of one CC session.
type Agent struct {
	Name      string    // basename(cwd) — short, recognizable
	Status    Status
	LastEvent time.Time
	RepoRoot  string
	GitBranch string
	SessionID string
}
