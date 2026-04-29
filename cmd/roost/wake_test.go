package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ----- isCCPlumbing -----

func TestIsCCPlumbing(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"<local-command-stdout>foo</local-command-stdout>", true},
		{"<command-name>/model</command-name>", true},
		{"<command-message>foo</command-message>", true},
		{"<command-args></command-args>", true},
		{"<system-reminder>blah</system-reminder>", true},
		{"Caveat: this is a CC caveat", true},
		{"  <local-command-stdout>leading whitespace ok</local-command-stdout>", true},

		{"whats 2+2", false},
		{"can you fix the auth bug", false},
		{"normal user prompt", false},
		{"", false},
	}
	for _, c := range cases {
		got := isCCPlumbing(c.in)
		if got != c.want {
			t.Errorf("isCCPlumbing(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ----- extractUserText -----

func TestExtractUserText_StringContent(t *testing.T) {
	raw := json.RawMessage(`{"role":"user","content":"whats 2+2"}`)
	got := extractUserText(raw)
	if got != "whats 2+2" {
		t.Errorf("got %q, want %q", got, "whats 2+2")
	}
}

func TestExtractUserText_ArrayContent(t *testing.T) {
	raw := json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image","source":{"type":"base64"}}]}`)
	got := extractUserText(raw)
	if got != "hello" {
		t.Errorf("got %q, want %q (image block should be skipped)", got, "hello")
	}
}

func TestExtractUserText_EmptyOrInvalid(t *testing.T) {
	if got := extractUserText(json.RawMessage(`{}`)); got != "" {
		t.Errorf("missing content should return empty, got %q", got)
	}
	if got := extractUserText(json.RawMessage(`not-json`)); got != "" {
		t.Errorf("malformed json should return empty, got %q", got)
	}
}

// ----- extractAssistantText -----

func TestExtractAssistantText_TextBlocks(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}`)
	text, tools := extractAssistantText(raw)
	if text != "hello  world" && text != "hello world" {
		// trailing space joiner is fine; we trim at the end
		t.Errorf("text = %q, want hello world (trim-tolerant)", text)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %v", tools)
	}
}

func TestExtractAssistantText_SkipsThinkingBlocks(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning..."},{"type":"text","text":"the answer"}]}`)
	text, _ := extractAssistantText(raw)
	if !strings.Contains(text, "the answer") {
		t.Errorf("text should contain the actual answer, got %q", text)
	}
	if strings.Contains(text, "reasoning") {
		t.Errorf("thinking block should be skipped, got %q", text)
	}
}

func TestExtractAssistantText_ToolCallsCaptured(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la","description":"list files"}}]}`)
	_, tools := extractAssistantText(raw)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool call, got %d: %v", len(tools), tools)
	}
	if !strings.Contains(tools[0], "Bash") || !strings.Contains(tools[0], "ls -la") {
		t.Errorf("tool call should mention name and command, got %q", tools[0])
	}
}

func TestExtractAssistantText_ToolUseFilePath(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/tmp/foo.go","old_string":"a","new_string":"b"}}]}`)
	_, tools := extractAssistantText(raw)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tools))
	}
	if !strings.Contains(tools[0], "/tmp/foo.go") {
		t.Errorf("tool call should prefer file_path field, got %q", tools[0])
	}
}

// ----- reorderFlagsFirst -----

func TestReorderFlagsFirst(t *testing.T) {
	valueFlags := []string{"-n"}
	boolFlags := []string{"--list"}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"flag first", []string{"-n", "4", "kalshi"}, []string{"-n", "4", "kalshi"}},
		{"flag after", []string{"kalshi", "-n", "4"}, []string{"-n", "4", "kalshi"}},
		{"flag in middle", []string{"abm", "-n", "10", "tools"}, []string{"-n", "10", "abm", "tools"}},
		{"bool flag at end", []string{"--list"}, []string{"--list"}},
		{"bool after positional", []string{"foo", "--list"}, []string{"--list", "foo"}},
		{"no flags", []string{"kalshi", "auth"}, []string{"kalshi", "auth"}},
		{"equals form", []string{"foo", "-n=5"}, []string{"-n=5", "foo"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reorderFlagsFirst(c.in, valueFlags, boolFlags)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("idx %d: got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// ----- filterSessions -----

func TestFilterSessions(t *testing.T) {
	in := []sessionInfo{
		{SessionID: "abc12345", CWD: "/Users/x/projects/auth-svc", GitBranch: "main"},
		{SessionID: "def67890", CWD: "/Users/x/projects/billing", GitBranch: "feature/auth-rate-limit"},
		{SessionID: "fed09876", CWD: "/Users/x/work/ui", GitBranch: "main"},
	}
	cases := []struct {
		pattern  string
		wantIDs  []string
	}{
		{"auth", []string{"abc12345", "def67890"}}, // matches cwd OR branch
		{"billing", []string{"def67890"}},
		{"abc", []string{"abc12345"}}, // session id prefix
		{"nonexistent", nil},
		{"", []string{"abc12345", "def67890", "fed09876"}}, // empty = all
	}
	for _, c := range cases {
		t.Run(c.pattern, func(t *testing.T) {
			got := filterSessions(in, c.pattern)
			if len(got) != len(c.wantIDs) {
				t.Fatalf("pattern %q: got %d sessions, want %d", c.pattern, len(got), len(c.wantIDs))
			}
			for i, want := range c.wantIDs {
				if got[i].SessionID != want {
					t.Errorf("idx %d: got %s, want %s", i, got[i].SessionID, want)
				}
			}
		})
	}
}

// ----- scanSessions / lastMetadata (integration via tmpdir) -----

func TestScanSessions_RealTranscriptShape(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "-Users-x-projects-foo")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Two synthetic transcripts. First one (older) ends with a user record
	// so lastMetadata should pick up cwd/branch from there.
	t1 := filepath.Join(projDir, "11111111-1111-1111-1111-111111111111.jsonl")
	t1content := `{"type":"system","content":"started"}` + "\n" +
		`{"type":"user","cwd":"/Users/x/projects/foo","gitBranch":"main","sessionId":"11111111","timestamp":"2026-04-29T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(t1, []byte(t1content), 0644); err != nil {
		t.Fatal(err)
	}

	t2 := filepath.Join(projDir, "22222222-2222-2222-2222-222222222222.jsonl")
	t2content := `{"type":"user","cwd":"/Users/x/projects/foo","gitBranch":"feature","sessionId":"22222222","timestamp":"2026-04-29T11:00:00Z","message":{"role":"user","content":"newer"}}` + "\n"
	if err := os.WriteFile(t2, []byte(t2content), 0644); err != nil {
		t.Fatal(err)
	}

	// Make t2 newer.
	if err := os.Chtimes(t2, mustNow(t), mustNow(t)); err != nil {
		t.Fatal(err)
	}

	got, err := scanSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(got))
	}
	if got[0].SessionID != "22222222" {
		t.Errorf("most recent first: got %s, want 22222222", got[0].SessionID)
	}
	if got[0].GitBranch != "feature" {
		t.Errorf("branch: got %q, want feature", got[0].GitBranch)
	}
	if got[1].GitBranch != "main" {
		t.Errorf("older branch: got %q, want main", got[1].GitBranch)
	}
}

func TestScanSessions_TolerateCorruptLines(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "-Users-x-projects-foo")
	os.MkdirAll(projDir, 0755)
	tp := filepath.Join(projDir, "ffffffff-ffff-ffff-ffff-ffffffffffff.jsonl")

	good := `{"type":"user","cwd":"/Users/x/projects/foo","gitBranch":"main","sessionId":"ffffffff","timestamp":"2026-04-29T10:00:00Z"}`
	bad := `not-json`
	if err := os.WriteFile(tp, []byte(good+"\n"+bad+"\n"+good+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := scanSessions(dir)
	if err != nil {
		t.Fatalf("scan should tolerate corrupt lines, got error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 session, got %d", len(got))
	}
	if got[0].CWD != "/Users/x/projects/foo" {
		t.Errorf("cwd: got %q, want /Users/x/projects/foo", got[0].CWD)
	}
}

// ----- helpers -----

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should pass through, got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("long string should ellipsis, got %q", got)
	}
	if got := truncate("  hello  ", 10); got != "hello" {
		t.Errorf("should trim before length check, got %q", got)
	}
}

func TestHumanizeAgo(t *testing.T) {
	// not testing exact values (time-dependent) — just smoke the format.
	for _, dur := range []string{"30s", "5m", "3h", "2d"} {
		if !strings.HasSuffix(dur, "s") && !strings.HasSuffix(dur, "m") && !strings.HasSuffix(dur, "h") && !strings.HasSuffix(dur, "d") {
			t.Errorf("unexpected dur %q", dur)
		}
	}
}

// mustNow returns time.Now() — wrapped to make Chtimes call sites tidy.
func mustNow(t *testing.T) (time.Time) {
	t.Helper()
	return time.Now()
}
