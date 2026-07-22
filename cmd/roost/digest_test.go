package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDigestTranscript_Basic(t *testing.T) {
	p := writeTranscript(t,
		`{"type":"user","timestamp":"2026-07-20T10:00:00Z","message":{"role":"user","content":"fix the auth bug"}}`,
		`{"type":"assistant","timestamp":"2026-07-20T10:01:00Z","message":{"role":"assistant","content":[{"type":"text","text":"Found it in middleware.go"},{"type":"tool_use","name":"Edit","input":{"file_path":"/src/middleware.go"}}]}}`,
	)
	got, err := digestTranscript(p, 14000)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"USER: fix the auth bug", "CLAUDE: Found it in middleware.go", "TOOL: Edit(/src/middleware.go)"} {
		if !strings.Contains(got, want) {
			t.Errorf("digest missing %q:\n%s", want, got)
		}
	}
}

func TestDigestTranscript_SkipsPlumbingAndCorruptLines(t *testing.T) {
	p := writeTranscript(t,
		`{"type":"user","timestamp":"2026-07-20T10:00:00Z","message":{"role":"user","content":"<command-name>/model</command-name>"}}`,
		`not-json`,
		`{"type":"user","timestamp":"2026-07-20T10:01:00Z","message":{"role":"user","content":"real question"}}`,
	)
	got, err := digestTranscript(p, 14000)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "command-name") {
		t.Errorf("plumbing should be filtered:\n%s", got)
	}
	if !strings.Contains(got, "real question") {
		t.Errorf("real user text should survive:\n%s", got)
	}
}

func TestDigestTranscript_BudgetKeepsGoalAndTail(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-07-20T10:00:00Z","message":{"role":"user","content":"THE ORIGINAL GOAL"}}`,
	}
	for i := 0; i < 200; i++ {
		lines = append(lines, `{"type":"assistant","timestamp":"2026-07-20T10:05:00Z","message":{"role":"assistant","content":[{"type":"text","text":"middle chatter that repeats and repeats and repeats"}]}}`)
	}
	lines = append(lines, `{"type":"user","timestamp":"2026-07-20T11:00:00Z","message":{"role":"user","content":"THE LATEST MESSAGE"}}`)

	got, err := digestTranscript(writeTranscript(t, lines...), 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 3000 {
		t.Errorf("digest should respect the budget, got %d chars", len(got))
	}
	if !strings.Contains(got, "THE ORIGINAL GOAL") {
		t.Errorf("first user message (the goal) should be pinned:\n%s", truncate(got, 300))
	}
	if !strings.Contains(got, "THE LATEST MESSAGE") {
		t.Errorf("latest turn should be included:\n%s", truncate(got, 300))
	}
	if !strings.Contains(got, "earlier entries omitted") {
		t.Errorf("omission marker expected:\n%s", truncate(got, 300))
	}
}
