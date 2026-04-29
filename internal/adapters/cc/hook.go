// Package cc parses Claude Code lifecycle hook payloads into roost Events
// and manages the CC settings.json hook installation lifecycle.
package cc

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
)

// rawPayload mirrors the fields CC writes to a hook's stdin. Documented
// empirically from the Day-1 validation run on 2026-04-29.
type rawPayload struct {
	HookEventName    string `json:"hook_event_name"`
	SessionID        string `json:"session_id"`
	CWD              string `json:"cwd"`
	TranscriptPath   string `json:"transcript_path"`
	ToolName         string `json:"tool_name"`
	NotificationType string `json:"notification_type"`
}

// Parse converts a Claude Code hook stdin JSON payload into an Event.
// AgentID defaults to basename(cwd). RepoRoot is best-effort: if cwd is
// inside a git work tree, the work-tree root is used; otherwise we leave
// RepoRoot empty and let the classifier degrade to AgentID-only keying.
func Parse(r io.Reader) (events.Event, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return events.Event{}, err
	}
	var p rawPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return events.Event{}, err
	}

	repo, branch := repoMetadata(p.CWD)

	return events.Event{
		Schema:       events.SchemaVersion,
		Hook:         p.HookEventName,
		AgentID:      filepath.Base(p.CWD),
		RepoRoot:     repo,
		WorktreePath: p.CWD,
		SessionID:    p.SessionID,
		GitBranch:    branch,
		NotifType:    p.NotificationType,
		ToolName:     p.ToolName,
		Timestamp:    time.Now().UTC(),
	}, nil
}

// repoMetadata best-effort calls git from the cwd to learn the repo root and
// current branch. Failures are silent — non-git directories are valid hook
// sources too (Roost itself was used to debug class projects in Google Drive
// folders without git).
func repoMetadata(cwd string) (repoRoot, branch string) {
	if cwd == "" {
		return "", ""
	}
	if _, err := os.Stat(cwd); err != nil {
		return "", ""
	}

	repoRoot = runGit(cwd, "rev-parse", "--show-toplevel")
	branch = runGit(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "HEAD" {
		// Detached HEAD — show short SHA instead, more useful than literal "HEAD".
		if sha := runGit(cwd, "rev-parse", "--short", "HEAD"); sha != "" {
			branch = sha
		}
	}
	return repoRoot, branch
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
