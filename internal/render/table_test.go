package render

import (
	"strings"
	"testing"
	"time"

	"github.com/Thomaspeel6/roost/internal/state"
)

func TestAgentTable_EmptyHelpfulMessage(t *testing.T) {
	got := AgentTable(nil, true)
	if !strings.Contains(got, "No agents") {
		t.Errorf("empty state should hint at running CC, got:\n%s", got)
	}
}

func TestAgentTable_RendersEachAgent(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	agents := []state.Agent{
		{Name: "auth", Status: state.Blocked, LastEvent: now.Add(-2 * time.Minute), GitBranch: "main"},
		{Name: "ui", Status: state.Running, LastEvent: now.Add(-30 * time.Second), GitBranch: "feature/ui-cleanup"},
	}
	got := AgentTable(agents, true)
	for _, want := range []string{"auth", "ui", "blocked", "running", "main"} {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q, got:\n%s", want, got)
		}
	}
}

func TestAgentTable_BranchTruncation(t *testing.T) {
	long := strings.Repeat("x", 50)
	got := branchOrDash(long)
	if len(got) > 30 {
		t.Errorf("branch should truncate to 30 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated branch should end in ..., got %q", got)
	}
}

func TestAgentTable_EmptyBranchShowsDash(t *testing.T) {
	if got := branchOrDash(""); got != "-" {
		t.Errorf("empty branch should be -, got %q", got)
	}
}
