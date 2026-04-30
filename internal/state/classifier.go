package state

import (
	"sort"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
)

// StaleAfter is how long a session can go without any event before we mark
// it Stale. Five minutes is conservative; users with always-on terminals
// running multiple sessions for hours wouldn't want a session disappearing
// just because they haven't touched it for a few minutes.
const StaleAfter = 5 * time.Minute

// Classify reduces an event stream to one Agent per (RepoRoot, SessionID).
//
// Decision order, applied to the LAST event seen for each session:
//
//   Notification(notification_type=permission_prompt) → Blocked
//   Notification(notification_type=idle_prompt)       → Idle
//   Stop                                              → Idle
//   Pre/PostToolUse / UserPromptSubmit / SessionStart → Running
//   then: Running with last event > StaleAfter ago    → Stale
//
// Agents are returned sorted Blocked > Running > Idle > Stale, then
// most-recent-first within each bucket.
func Classify(in []events.Event, now time.Time) []Agent {
	type key struct{ repo, session string }
	agg := map[key]*Agent{}

	for _, e := range in {
		k := key{e.RepoRoot, e.SessionID}
		if k.repo == "" && k.session == "" {
			k = key{"", e.AgentID}
		}
		a, ok := agg[k]
		if !ok {
			a = &Agent{
				Name:         e.AgentID,
				RepoRoot:     e.RepoRoot,
				WorktreePath: e.WorktreePath,
				SessionID:    e.SessionID,
				GitBranch:    e.GitBranch,
				Status:       Running,
			}
			agg[k] = a
		}
		if e.Timestamp.After(a.LastEvent) {
			a.LastEvent = e.Timestamp
		}
		if e.GitBranch != "" {
			a.GitBranch = e.GitBranch
		}
		if e.WorktreePath != "" {
			a.WorktreePath = e.WorktreePath
		}

		// Apply this event's status implication. Each new event wins;
		// the LAST event in the stream determines the live status.
		switch e.Hook {
		case "Notification":
			switch e.NotifType {
			case "permission_prompt":
				a.Status = Blocked
			case "idle_prompt":
				a.Status = Idle
			}
		case "Stop":
			a.Status = Idle
		default:
			// Any tool-use or prompt event means active work in progress.
			a.Status = Running
		}
	}

	out := make([]Agent, 0, len(agg))
	for _, a := range agg {
		// Downgrade Running → Stale if we haven't seen a heartbeat in a while.
		// Idle and Blocked are explicit terminal states from the user's POV;
		// don't downgrade them just because time passed.
		if a.Status == Running && now.Sub(a.LastEvent) > StaleAfter {
			a.Status = Stale
		}
		out = append(out, *a)
	}

	rank := map[Status]int{Blocked: 0, Running: 1, Idle: 2, Stale: 3}
	sort.Slice(out, func(i, j int) bool {
		if rank[out[i].Status] != rank[out[j].Status] {
			return rank[out[i].Status] < rank[out[j].Status]
		}
		return out[i].LastEvent.After(out[j].LastEvent)
	})
	return out
}
