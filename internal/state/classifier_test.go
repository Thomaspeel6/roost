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

func notif(notifType, agent, session string, t time.Time) events.Event {
	return events.Event{
		Hook:      "Notification",
		NotifType: notifType,
		AgentID:   agent,
		SessionID: session,
		RepoRoot:  "/r",
		Timestamp: t,
	}
}

func TestClassify_StopMeansIdle(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		ev("PreToolUse", "auth", "s1", t0.Add(10*time.Second)),
		ev("PostToolUse", "auth", "s1", t0.Add(11*time.Second)),
		ev("Stop", "auth", "s1", t0.Add(60*time.Second)),
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status != Idle {
		t.Errorf("Stop should mean Idle (between turns), got %v", got[0].Status)
	}
}

func TestClassify_IdlePromptMeansIdleNotBlocked(t *testing.T) {
	// This is the bug v0.2 had: idle_prompt was treated as Blocked.
	// idle_prompt fires every turn end, same as Stop. It is NOT user-attention-required.
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		notif("idle_prompt", "auth", "s1", t0.Add(30*time.Second)),
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status != Idle {
		t.Errorf("idle_prompt notification should mean Idle, got %v", got[0].Status)
	}
}

func TestClassify_PermissionPromptMeansBlocked(t *testing.T) {
	// The actual "needs you NOW" signal: permission_prompt.
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		notif("permission_prompt", "auth", "s1", t0.Add(30*time.Second)),
	}
	got := Classify(in, t0.Add(2*time.Minute))
	if got[0].Status != Blocked {
		t.Errorf("permission_prompt notification should mean Blocked, got %v", got[0].Status)
	}
}

func TestClassify_RunningMidToolCall(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "auth", "s1", t0),
		ev("PreToolUse", "auth", "s1", t0.Add(10*time.Second)),
	}
	got := Classify(in, t0.Add(15*time.Second))
	if got[0].Status != Running {
		t.Errorf("PreToolUse without PostToolUse should mean Running, got %v", got[0].Status)
	}
}

func TestClassify_StaleAfterSilence(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("PreToolUse", "auth", "s1", t0.Add(30*time.Second)),
	}
	got := Classify(in, t0.Add(15*time.Minute))
	if got[0].Status != Stale {
		t.Errorf("Running with old last-event should downgrade to Stale, got %v", got[0].Status)
	}
}

func TestClassify_IdleStaysIdleEvenAfterLongSilence(t *testing.T) {
	// User mental model: a session that finished a turn 2 hours ago is
	// still "idle, available to talk to" — not "stale." Don't downgrade
	// terminal states based on time alone.
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("Stop", "auth", "s1", t0),
	}
	got := Classify(in, t0.Add(2*time.Hour))
	if got[0].Status != Idle {
		t.Errorf("Idle should not auto-downgrade to Stale, got %v", got[0].Status)
	}
}

func TestClassify_BlockedStaysBlocked(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		notif("permission_prompt", "auth", "s1", t0),
	}
	got := Classify(in, t0.Add(2*time.Hour))
	if got[0].Status != Blocked {
		t.Errorf("Blocked should not auto-downgrade, got %v", got[0].Status)
	}
}

func TestClassify_LaterEventOverridesEarlier(t *testing.T) {
	// If the agent goes Blocked then the user approves and a new tool runs,
	// the latest event wins.
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		notif("permission_prompt", "auth", "s1", t0),
		ev("PreToolUse", "auth", "s1", t0.Add(5*time.Second)),
	}
	got := Classify(in, t0.Add(10*time.Second))
	if got[0].Status != Running {
		t.Errorf("After Blocked then PreToolUse, should be Running again, got %v", got[0].Status)
	}
}

func TestClassify_MultipleAgentsSorted(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	now := t0.Add(10 * time.Minute)
	in := []events.Event{
		ev("PostToolUse", "running", "s1", now.Add(-30*time.Second)),
		notif("permission_prompt", "blocked", "s2", t0.Add(45*time.Second)),
		ev("Stop", "idle", "s3", t0.Add(20*time.Second)),
		ev("PreToolUse", "stale", "s4", t0), // 10 min old, no Stop → Stale
	}
	got := Classify(in, now)
	if len(got) != 4 {
		t.Fatalf("want 4 agents, got %d", len(got))
	}
	wantOrder := []Status{Blocked, Running, Idle, Stale}
	for i, want := range wantOrder {
		if got[i].Status != want {
			t.Errorf("idx %d: want %v, got %v (name=%s)", i, want, got[i].Status, got[i].Name)
		}
	}
}

func TestClassify_DifferentSessionsSameAgent(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	in := []events.Event{
		ev("SessionStart", "moveo.ai", "session-A", t0),
		ev("SessionStart", "moveo.ai", "session-B", t0.Add(10*time.Second)),
		ev("Stop", "moveo.ai", "session-A", t0.Add(20*time.Second)),
	}
	got := Classify(in, t0.Add(time.Minute))
	if len(got) != 2 {
		t.Fatalf("two sessions in same project should produce two agents, got %d", len(got))
	}
}
