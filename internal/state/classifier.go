package state

import (
	"sort"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
)

// IdleAfter is how long a session can go without any event before we mark it Idle.
const IdleAfter = 5 * time.Minute

// Classify reduces an event stream to one Agent per (RepoRoot, SessionID).
// State machine (validated by the Day-1 hook semantics gate):
//
//	Notification(notification_type=idle_prompt)        → Blocked  (highest priority once seen)
//	Stop with no later Notification                    → Done     (turn finished, idle until next prompt)
//	PreToolUse / PostToolUse / SessionStart / etc      → Running  (tool in progress or Claude processing)
//	No event in IdleAfter                              → Idle     (downgraded from Running at end)
//
// Agents are returned sorted Blocked > Done > Running > Idle, then most-recent-first.
func Classify(in []events.Event, now time.Time) []Agent {
	type key struct{ repo, session string }
	agg := map[key]*Agent{}

	for _, e := range in {
		k := key{e.RepoRoot, e.SessionID}
		// Allow events without RepoRoot/SessionID to still classify under
		// AgentID alone (degraded mode but better than dropping).
		if k.repo == "" && k.session == "" {
			k = key{"", e.AgentID}
		}
		a, ok := agg[k]
		if !ok {
			a = &Agent{
				Name:      e.AgentID,
				RepoRoot:  e.RepoRoot,
				SessionID: e.SessionID,
				GitBranch: e.GitBranch,
				Status:    Running,
			}
			agg[k] = a
		}
		if e.Timestamp.After(a.LastEvent) {
			a.LastEvent = e.Timestamp
		}
		if e.GitBranch != "" {
			a.GitBranch = e.GitBranch
		}
		switch e.Hook {
		case "Stop":
			// Promoting to Done unless we've subsequently seen a Notification.
			if a.Status != Blocked {
				a.Status = Done
			}
		case "Notification":
			// Only the idle_prompt notification means BLOCKED. Other notification
			// types (permission, etc) don't unambiguously signal user-input-required.
			if e.NotifType == "idle_prompt" {
				a.Status = Blocked
			}
		default:
			// Any non-terminal event resets to Running unless we've already gone Blocked.
			if a.Status != Blocked {
				a.Status = Running
			}
		}
	}

	out := make([]Agent, 0, len(agg))
	for _, a := range agg {
		// Downgrade Running → Idle if it's been too long since the last event.
		if a.Status == Running && now.Sub(a.LastEvent) > IdleAfter {
			a.Status = Idle
		}
		out = append(out, *a)
	}

	rank := map[Status]int{Blocked: 0, Done: 1, Running: 2, Idle: 3}
	sort.Slice(out, func(i, j int) bool {
		if rank[out[i].Status] != rank[out[j].Status] {
			return rank[out[i].Status] < rank[out[j].Status]
		}
		return out[i].LastEvent.After(out[j].LastEvent)
	})
	return out
}
