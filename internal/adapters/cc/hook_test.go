package cc

import (
	"strings"
	"testing"
)

const samplePostToolUse = `{
  "hook_event_name":"PostToolUse",
  "session_id":"abc-123",
  "cwd":"/Users/x/code/proj/wt-auth",
  "transcript_path":"/Users/x/.claude/transcript.jsonl",
  "tool_name":"Bash"
}`

const sampleNotificationIdle = `{
  "hook_event_name":"Notification",
  "session_id":"abc-123",
  "cwd":"/Users/x/code/proj",
  "notification_type":"idle_prompt",
  "message":"Claude is waiting for your input"
}`

func TestParse_PostToolUse(t *testing.T) {
	e, err := Parse(strings.NewReader(samplePostToolUse))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if e.Hook != "PostToolUse" {
		t.Errorf("hook = %q", e.Hook)
	}
	if e.SessionID != "abc-123" {
		t.Errorf("session = %q", e.SessionID)
	}
	if e.WorktreePath != "/Users/x/code/proj/wt-auth" {
		t.Errorf("worktree = %q", e.WorktreePath)
	}
	if e.ToolName != "Bash" {
		t.Errorf("tool = %q", e.ToolName)
	}
}

func TestParse_AgentIDFromCWDBasename(t *testing.T) {
	e, err := Parse(strings.NewReader(samplePostToolUse))
	if err != nil {
		t.Fatal(err)
	}
	if e.AgentID != "wt-auth" {
		t.Errorf("AgentID should be basename(cwd), got %q", e.AgentID)
	}
}

func TestParse_NotificationCarriesType(t *testing.T) {
	e, err := Parse(strings.NewReader(sampleNotificationIdle))
	if err != nil {
		t.Fatal(err)
	}
	if e.Hook != "Notification" {
		t.Errorf("hook = %q", e.Hook)
	}
	if e.NotifType != "idle_prompt" {
		t.Errorf("NotifType should be %q, got %q", "idle_prompt", e.NotifType)
	}
}

func TestParse_RejectsMalformed(t *testing.T) {
	if _, err := Parse(strings.NewReader("not-json")); err == nil {
		t.Error("want error on malformed input")
	}
}

func TestParse_StampsSchemaVersion(t *testing.T) {
	e, err := Parse(strings.NewReader(samplePostToolUse))
	if err != nil {
		t.Fatal(err)
	}
	if e.Schema == 0 {
		t.Error("Parse should stamp the schema version")
	}
}

func TestParse_TimestampIsRecent(t *testing.T) {
	e, err := Parse(strings.NewReader(samplePostToolUse))
	if err != nil {
		t.Fatal(err)
	}
	if e.Timestamp.IsZero() {
		t.Error("timestamp should be set to wall clock at parse time")
	}
}
