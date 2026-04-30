// Package state classifies a stream of CC hook events into per-session Agent state.
package state

import "time"

// Status of a CC session at the time of classification.
//
// Semantics (matches what CC's hooks actually fire, validated empirically
// against ~400 events):
//
//   Blocked — Notification(permission_prompt) was the most recent state.
//             Claude is asking for explicit approval. You should look at it.
//   Running — A tool is mid-flight or Claude is processing output. Don't bother it.
//   Idle    — Stop or Notification(idle_prompt) — turn finished, available
//             for your next prompt. Most sessions live here.
//   Stale   — No event in the staleness window; probably forgotten.
type Status int

const (
	Running Status = iota
	Idle
	Stale
	Blocked
)

func (s Status) String() string {
	return [...]string{"running", "idle", "stale", "blocked"}[s]
}

// Agent is the classified view of one CC session.
type Agent struct {
	Name         string // basename(cwd) — short, recognizable
	Status       Status
	LastEvent    time.Time
	RepoRoot     string
	WorktreePath string
	GitBranch    string
	SessionID    string
}
