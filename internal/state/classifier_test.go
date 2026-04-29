package state

import (
	"testing"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
)

func ev(hook, agent, session string, t time.Time) events.Event {
	return events.Event{
		Hook:      hook,
		AgentID:   agent,
		SessionID: session,
		RepoRoot:  "/r",
		Timestamp: t,
	}
}

func TestClassify_RunningThenDone(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		ev("PreToolUse", "auth", "s1", t0.Add(10*time.Second)),
		ev("PostToolUse", "auth", "s1", t0.Add(11*time.Second)),
		ev("Stop", "auth", "s1", t0.Add(60*time.Second)),
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if len(got) != 1 {
		t.Fatalf("want 1 agent, got %d", len(got))
	}
	if got[0].Status != Done {
		t.Errorf("want Done after Stop, got %v", got[0].Status)
	}
}

func TestClassify_BlockedOnNotification(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		{Hook: "Notification", AgentID: "auth", SessionID: "s1", RepoRoot: "/r", NotifType: "idle_prompt", Timestamp: t0.Add(30 * time.Second)},
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status != Blocked {
		t.Errorf("want Blocked after idle_prompt notification, got %v", got[0].Status)
	}
}

func TestClassify_NotificationOtherTypeNotBlocking(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		{Hook: "Notification", AgentID: "auth", SessionID: "s1", RepoRoot: "/r", NotifType: "permission", Timestamp: t0.Add(30 * time.Second)},
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status == Blocked {
		t.Errorf("non-idle_prompt notification should not flip to Blocked, got %v", got[0].Status)
	}
}

func TestClassify_BlockedStaysBlockedThroughLaterEvents(t *testing.T) {
	// In the real flow, additional hook events shouldn't fire while CC is
	// genuinely blocked, but if a stale ordering sneaks in we should not
	// silently downgrade.
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		{Hook: "Notification", AgentID: "auth", SessionID: "s1", RepoRoot: "/r", NotifType: "idle_prompt", Timestamp: t0.Add(30 * time.Second)},
		ev("PreToolUse", "auth", "s1", t0.Add(31*time.Second)), // out-of-band; should not unblock
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status != Blocked {
		t.Errorf("stale event after Blocked should not unblock, got %v", got[0].Status)
	}
}

func TestClassify_IdleAfterSilence(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		ev("PostToolUse", "auth", "s1", t0.Add(30*time.Second)),
	}
	// 10 minutes of silence after the last event.
	got := Classify(in, t0.Add(15*time.Minute))
	if got[0].Status != Idle {
		t.Errorf("want Idle after silence, got %v", got[0].Status)
	}
}

func TestClassify_RunningRecent(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("PostToolUse", "auth", "s1", t0),
	}
	got := Classify(in, t0.Add(30*time.Second))
	if got[0].Status != Running {
		t.Errorf("recent activity should be Running, got %v", got[0].Status)
	}
}

func TestClassify_MultipleAgentsSorted(t *testing.T) {
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	now := t0.Add(10 * time.Minute)
	in := []events.Event{
		// running: very recent activity (30s before now)
		ev("PostToolUse", "running", "s1", now.Add(-30*time.Second)),
		// blocked: any time, blocked never times out
		{Hook: "Notification", AgentID: "blocked", SessionID: "s2", RepoRoot: "/r", NotifType: "idle_prompt", Timestamp: t0.Add(45 * time.Second)},
		// done: any time, done never times out
		ev("Stop", "done", "s3", t0.Add(20*time.Second)),
		// idle: SessionStart at t0, now is 10 min later → > IdleAfter (5m)
		ev("SessionStart", "idle", "s4", t0),
	}
	got := Classify(in, now)
	if len(got) != 4 {
		t.Fatalf("want 4 agents, got %d", len(got))
	}
	// Expected order: Blocked > Done > Running > Idle
	wantOrder := []Status{Blocked, Done, Running, Idle}
	for i, want := range wantOrder {
		if got[i].Status != want {
			t.Errorf("idx %d: want %v, got %v (name=%s)", i, want, got[i].Status, got[i].Name)
		}
	}
}

func TestClassify_DifferentSessionsSameAgent(t *testing.T) {
	// Two sessions running in the same project should be tracked separately.
	t0 := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "session-A", t0),
		ev("SessionStart", "auth", "session-B", t0.Add(10*time.Second)),
		ev("Stop", "auth", "session-A", t0.Add(20*time.Second)),
	}
	got := Classify(in, t0.Add(time.Minute))
	if len(got) != 2 {
		t.Fatalf("two sessions should produce two agents, got %d", len(got))
	}
}
