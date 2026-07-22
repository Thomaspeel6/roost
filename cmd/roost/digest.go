package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// digestTranscript distills a CC transcript into a compact, human-readable
// conversation log suitable for an LLM prompt:
//
//	[15:04] USER: can you fix the auth bug
//	[15:05] CLAUDE: The middleware drops the token when ...
//	[15:05] TOOL: Edit(/src/middleware.go)
//
// This replaces the old approach of pasting raw JSONL lines, which drowned
// the model in metadata and multi-megabyte tool results. The digest keeps
// the first user message (the session's goal) and as many trailing turns as
// fit in charBudget.
func digestTranscript(path string, charBudget int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	firstUser := ""

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var r transcriptRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, r.Timestamp)
		stamp := ts.Local().Format("15:04")
		switch r.Type {
		case "user":
			text := extractUserText(r.Message)
			if text == "" || isCCPlumbing(text) {
				continue
			}
			line := fmt.Sprintf("[%s] USER: %s", stamp, truncate(text, 700))
			if firstUser == "" {
				firstUser = line
			}
			lines = append(lines, line)
		case "assistant":
			text, tools := extractAssistantText(r.Message)
			if text != "" {
				lines = append(lines, fmt.Sprintf("[%s] CLAUDE: %s", stamp, truncate(text, 700)))
			}
			for _, tc := range tools {
				lines = append(lines, fmt.Sprintf("[%s] TOOL: %s", stamp, tc))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}

	// Take trailing lines that fit the budget.
	total := 0
	start := len(lines)
	for start > 0 && total+len(lines[start-1])+1 <= charBudget {
		start--
		total += len(lines[start]) + 1
	}
	tail := lines[start:]

	var b strings.Builder
	if start > 0 && firstUser != "" {
		// The session goal scrolled out of the window — pin it at the top.
		b.WriteString(firstUser)
		b.WriteString("\n[... ")
		fmt.Fprintf(&b, "%d earlier entries omitted ...]\n", start)
	}
	b.WriteString(strings.Join(tail, "\n"))
	return b.String(), nil
}
