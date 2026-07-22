package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// recapMeta is the session context handed to the recap engine alongside the
// transcript digest.
type recapMeta struct {
	Name   string
	CWD    string
	Branch string
	Status string // classifier output from hook events, "" if unknown
}

// recapAvailable reports whether any LLM recap engine can run: the claude
// CLI (which carries the user's Claude Code login, so no key is required),
// or a direct API key.
func recapAvailable() bool {
	if _, err := exec.LookPath("claude"); err == nil {
		return true
	}
	return os.Getenv("ANTHROPIC_API_KEY") != ""
}

// liveRecap produces the structured recap using the best available engine:
//
//  1. claude CLI on PATH   → `claude -p` headless call; reuses the user's
//     existing Claude Code auth (subscription), so roost needs no key and
//     recaps cost nothing extra.
//  2. ANTHROPIC_API_KEY set → direct Messages API call (stdlib HTTP, no SDK).
//
// Returns ("", nil) if no engine is available (caller falls back to the raw
// transcript reader).
func liveRecap(ctx context.Context, meta recapMeta, digest string) (string, error) {
	prompt := buildRecapPrompt(meta, digest)

	if _, err := exec.LookPath("claude"); err == nil {
		out, err := cliRecap(ctx, prompt)
		if err == nil {
			return out, nil
		}
		// CLI present but failed (logged-out, update in progress, …):
		// fall through to the API key if one is set.
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return "", err
		}
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return apiRecap(ctx, key, prompt)
	}
	return "", nil // no engine; fall back to raw transcript
}

func buildRecapPrompt(meta recapMeta, digest string) string {
	status := meta.Status
	if status == "" {
		status = "(unknown — live hooks not installed)"
	}
	return fmt.Sprintf(`You are re-grounding a developer who runs several Claude Code sessions in parallel and just came back to this one. Write the recap a good teammate would give: concrete, plain language, no filler.

SESSION: %s
DIRECTORY: %s
BRANCH: %s
LIVE STATUS FROM LIFECYCLE HOOKS: %s

Below is a digest of the session transcript, oldest first. TOOL lines are tool calls Claude made.

Respond in EXACTLY this format and nothing else — no preamble, no markdown:

WAS DOING: <the overall task, one plain sentence>
PROGRESS:
- <a concrete thing that got done — name real files/tests/branches>
- <up to 3 bullets total, oldest first; use 1 bullet if that is all that happened>
LAST FINISHED: <the most recent completed step>
STATUS: <running|blocked|idle|done> — <why, one line; if Claude asked a question or is waiting on approval, quote it briefly>
NEXT: <the single most useful action for the developer right now>

TRANSCRIPT DIGEST:
%s`, meta.Name, meta.CWD, meta.Branch, status, digest)
}

// apiRecap calls the Anthropic Messages API directly. Stdlib net/http on
// purpose — roost stays dependency-light and this is its only network call.
// Model is Claude Haiku 4.5 by default (fast, ~half a cent per recap);
// override with ROOST_MODEL.
func apiRecap(ctx context.Context, key, prompt string) (string, error) {
	model := os.Getenv("ROOST_MODEL")
	if model == "" {
		model = "claude-haiku-4-5"
	}

	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 700,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("anthropic api: status %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			out.WriteString(c.Text)
		}
	}
	return strings.TrimSpace(out.String()), nil
}

// cliRecap runs the prompt through `claude -p` in headless mode. This rides
// the user's existing Claude Code authentication (subscription or key), so
// roost itself needs no credentials. ROOST_SUPPRESS tells roost-hook to
// ignore lifecycle events from this helper session, keeping it out of
// `roost ls`.
func cliRecap(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(os.Environ(), "ROOST_SUPPRESS=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("claude cli: %s", truncate(msg, 200))
	}
	return strings.TrimSpace(string(out)), nil
}
