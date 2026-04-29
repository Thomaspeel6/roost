package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// liveRecap calls the Anthropic Messages API with Claude Haiku 4.5 to summarize
// a live session into the structured 4-line format. Returns ("", nil) if no
// API key is configured (caller falls back to the raw transcript reader).
//
// The HTTP call uses stdlib net/http; no SDK dependency.
func liveRecap(ctx context.Context, agentName, transcriptTail string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", nil // signal: no key, fall back to raw transcript
	}

	prompt := fmt.Sprintf(`You are summarizing a live Claude Code session for a user who stepped away and just came back.

Read the transcript tail below. Answer in EXACTLY this format, four lines, no preamble:

WAS DOING: <one line>
LAST FINISHED: <one line>
STATUS: <blocked|running|idle and one-line reason>
NEXT: <one line — what the user likely needs to do>

AGENT: %s

TRANSCRIPT TAIL:
%s`, agentName, transcriptTail)

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 400,
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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("anthropic api: status %d: %s", resp.StatusCode, truncatePreview(string(raw), 200))
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

func truncatePreview(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
