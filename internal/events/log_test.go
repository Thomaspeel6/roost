package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOST_EVENTS_PATH", filepath.Join(dir, "events.jsonl"))

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	e1 := Event{Hook: "SessionStart", AgentID: "auth", SessionID: "s1", RepoRoot: "/r1", Timestamp: now}
	e2 := Event{Hook: "Stop", AgentID: "auth", SessionID: "s1", RepoRoot: "/r1", Timestamp: now.Add(time.Minute)}

	if err := Append(e1); err != nil {
		t.Fatalf("append e1: %v", err)
	}
	if err := Append(e2); err != nil {
		t.Fatalf("append e2: %v", err)
	}

	got, err := Replay(100)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].Hook != "SessionStart" || got[1].Hook != "Stop" {
		t.Fatalf("wrong order: %+v", got)
	}
	if got[0].Schema != SchemaVersion {
		t.Errorf("schema not auto-stamped: %d", got[0].Schema)
	}
}

func TestReplay_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOST_EVENTS_PATH", filepath.Join(dir, "missing.jsonl"))

	got, err := Replay(100)
	if err != nil {
		t.Fatalf("replay missing should be nil error, got %v", err)
	}
	if got != nil {
		t.Errorf("want nil slice, got %v", got)
	}
}

func TestReplay_SkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	t.Setenv("ROOST_EVENTS_PATH", path)

	// Write good, bad, good directly.
	good := `{"schema":1,"hook":"SessionStart","agent_id":"x","ts":"2026-04-29T10:00:00Z"}`
	bad := "not-json"
	good2 := `{"schema":1,"hook":"Stop","agent_id":"x","ts":"2026-04-29T10:01:00Z"}`
	if err := writeFile(path, good+"\n"+bad+"\n"+good2+"\n"); err != nil {
		t.Fatal(err)
	}

	got, err := Replay(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 valid events, got %d", len(got))
	}
}

func TestAppend_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOST_EVENTS_PATH", filepath.Join(dir, "events.jsonl"))

	const n = 200
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			errs <- Append(Event{Hook: "PostToolUse", AgentID: "x", Timestamp: time.Now()})
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent append %d: %v", i, err)
		}
	}
	got, err := Replay(n + 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("want %d events after concurrent append, got %d (write race?)", n, len(got))
	}
}

func TestReplay_RespectsMaxEvents(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOST_EVENTS_PATH", filepath.Join(dir, "events.jsonl"))

	for i := 0; i < 50; i++ {
		if err := Append(Event{Hook: "PostToolUse", AgentID: "x", Timestamp: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Replay(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("max=10 but got %d", len(got))
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
