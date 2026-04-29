// Package events defines the on-disk event log used by Roost.
// Each line in ~/.roost/events.jsonl is one Event record. Hooks append.
// CLI commands read.
package events

import "time"

// SchemaVersion is bumped when the on-disk format changes incompatibly.
const SchemaVersion = 1

// Event captures one Claude Code lifecycle hook fire. The classifier
// reduces a stream of these into per-session Agent state.
type Event struct {
	Schema       int       `json:"schema"`
	Hook         string    `json:"hook"`         // SessionStart, PreToolUse, PostToolUse, UserPromptSubmit, Stop, Notification
	AgentID      string    `json:"agent_id"`     // basename(cwd) by default
	RepoRoot     string    `json:"repo_root,omitempty"`
	WorktreePath string    `json:"worktree_path,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	GitBranch    string    `json:"git_branch,omitempty"`
	NotifType    string    `json:"notification_type,omitempty"` // for Notification, e.g. "idle_prompt"
	ToolName     string    `json:"tool_name,omitempty"`         // for Pre/PostToolUse
	Timestamp    time.Time `json:"ts"`
}
