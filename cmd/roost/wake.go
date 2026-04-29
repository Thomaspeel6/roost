package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
)

// transcriptRecord is the minimum we read from a transcript JSONL line.
// CC writes many record types; we only care about user, assistant, and the
// metadata fields used for filtering.
type transcriptRecord struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`
	GitBranch string          `json:"gitBranch"`
	SessionID string          `json:"sessionId"`
	Message   json.RawMessage `json:"message"`
}

type assistantContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`  // for tool_use
	Input json.RawMessage `json:"input"` // for tool_use
}

type sessionInfo struct {
	Path      string
	SessionID string
	CWD       string
	GitBranch string
	ModTime   time.Time
}

// roostProjectsDir is overridable for tests. CC writes transcripts here.
var roostProjectsDir = filepath.Join(os.Getenv("HOME"), ".claude", "projects")

func runWake(args []string) int {
	// Reorder args so flags appear before positionals. Stdlib flag.Parse stops at
	// the first non-flag, but real users type `roost kalshi -n 4` half the time.
	args = reorderFlagsFirst(args, []string{"-n"}, []string{"--list", "--live", "--no-live"})

	fs := flag.NewFlagSet("wake", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	list := fs.Bool("list", false, "list available sessions, most recent first")
	turns := fs.Int("n", 6, "number of turns to show")
	noLive := fs.Bool("no-live", false, "skip the LLM live recap; always show raw transcript tail")
	forceLive := fs.Bool("live", false, "force the LLM live recap even if no recent events were observed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	pattern := strings.Join(fs.Args(), " ")

	sessions, err := scanSessions(roostProjectsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan transcripts: %v\n", err)
		return 1
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No Claude Code sessions found at", roostProjectsDir)
		fmt.Fprintln(os.Stderr, "(have you used Claude Code yet?)")
		return 1
	}

	if *list {
		printSessionList(sessions)
		return 0
	}

	// If no pattern given and we're inside a directory CC has seen, prefer it.
	if pattern == "" {
		if cwd, err := os.Getwd(); err == nil {
			pattern = cwd
		}
	}

	matched := filterSessions(sessions, pattern)
	if len(matched) == 0 {
		fmt.Fprintf(os.Stderr, "No session matched %q.\n", pattern)
		fmt.Fprintln(os.Stderr, "Try:  roost wake --list")
		return 1
	}

	target := matched[0]

	// Decide live mode: explicit flag wins; otherwise auto-detect by checking
	// the events log for any event matching this session in the last hour.
	useLive := *forceLive || (!*noLive && sessionIsLive(target))

	if useLive {
		printed, err := tryLiveRecap(target)
		if err != nil {
			// Live recap is best-effort; log to stderr and fall back.
			fmt.Fprintf(os.Stderr, "live recap unavailable: %v\n", err)
		}
		if printed {
			return 0
		}
	}

	if err := printRecap(target, *turns); err != nil {
		fmt.Fprintf(os.Stderr, "recap: %v\n", err)
		return 1
	}
	return 0
}

// scanSessions globs every transcript jsonl under projectsDir and reads each
// file's last record to extract metadata. Returns sessions sorted most-recent-first.
func scanSessions(projectsDir string) ([]sessionInfo, error) {
	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	out := make([]sessionInfo, 0, len(paths))
	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil {
			continue
		}
		info := sessionInfo{Path: p, ModTime: stat.ModTime()}
		// Pull metadata from the last user/assistant record in the file.
		// Cheap because we read backwards from EOF.
		if md, err := lastMetadata(p); err == nil {
			info.SessionID = md.SessionID
			info.CWD = md.CWD
			info.GitBranch = md.GitBranch
		}
		// Fallback: derive session id from filename if absent in records.
		if info.SessionID == "" {
			info.SessionID = strings.TrimSuffix(filepath.Base(p), ".jsonl")
		}
		// Fallback: derive cwd from parent dir name (CC encodes / as -).
		if info.CWD == "" {
			info.CWD = decodeProjectDir(filepath.Base(filepath.Dir(p)))
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// lastMetadata reads a transcript and returns metadata from the most recent
// record that has cwd/sessionId. Streams forward; transcripts are small enough
// that the simplicity is worth it.
func lastMetadata(path string) (transcriptRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return transcriptRecord{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var last transcriptRecord
	for scanner.Scan() {
		var r transcriptRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.CWD != "" || r.SessionID != "" {
			last = r
		}
	}
	return last, scanner.Err()
}

// decodeProjectDir reverses CC's project-dir encoding. CC encodes "/Users/Thomas/projects/roost"
// as "-Users-Thomas-projects-roost". This is a best-effort decoder used only
// when a transcript's records lack cwd metadata.
func decodeProjectDir(name string) string {
	return strings.Replace(name, "-", "/", -1)
}

func filterSessions(sessions []sessionInfo, pattern string) []sessionInfo {
	if pattern == "" {
		return sessions
	}
	pl := strings.ToLower(pattern)
	out := make([]sessionInfo, 0, len(sessions))
	for _, s := range sessions {
		hay := strings.ToLower(s.CWD + " " + s.GitBranch + " " + s.SessionID)
		if strings.Contains(hay, pl) {
			out = append(out, s)
		}
	}
	return out
}

func printSessionList(sessions []sessionInfo) {
	for _, s := range sessions {
		ago := humanizeAgo(time.Since(s.ModTime))
		branch := s.GitBranch
		if branch == "" {
			branch = "-"
		}
		fmt.Printf("%-12s  %-30s  %s  %s\n", ago, branch, shortID(s.SessionID), s.CWD)
	}
}

func humanizeAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// printRecap streams the transcript and prints the last `turns` user→assistant
// exchanges in plain text. CC plumbing messages (UI commands, tool results
// wrapped in <local-command-*> tags) are filtered out.
func printRecap(s sessionInfo, turns int) error {
	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	type turn struct {
		ts        time.Time
		role      string
		text      string
		toolCalls []string
	}
	var all []turn

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var r transcriptRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, r.Timestamp)
		switch r.Type {
		case "user":
			text := extractUserText(r.Message)
			if text == "" || isCCPlumbing(text) {
				continue
			}
			all = append(all, turn{ts: ts, role: "user", text: text})
		case "assistant":
			text, tools := extractAssistantText(r.Message)
			if text == "" && len(tools) == 0 {
				continue
			}
			all = append(all, turn{ts: ts, role: "assistant", text: text, toolCalls: tools})
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Pair up trailing turns.
	if turns < 1 {
		turns = 6
	}
	maxRecords := turns * 2
	if len(all) > maxRecords {
		all = all[len(all)-maxRecords:]
	}

	branch := s.GitBranch
	if branch == "" {
		branch = "(no branch)"
	}
	fmt.Printf("session %s — %s — %s\n", shortID(s.SessionID), s.CWD, branch)
	fmt.Println(strings.Repeat("─", 72))
	for _, t := range all {
		stamp := t.ts.Local().Format("15:04:05")
		switch t.role {
		case "user":
			fmt.Printf("[%s] you:\n  %s\n", stamp, indent(truncate(t.text, 600), "  "))
		case "assistant":
			if t.text != "" {
				fmt.Printf("[%s] claude:\n  %s\n", stamp, indent(truncate(t.text, 600), "  "))
			}
			for _, tc := range t.toolCalls {
				fmt.Printf("[%s]   ↳ %s\n", stamp, tc)
			}
		}
	}
	fmt.Println(strings.Repeat("─", 72))
	return nil
}

func extractUserText(raw json.RawMessage) string {
	// user.message.content can be a string OR an array of content blocks
	// (when the user attaches an image, for example).
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || len(msg.Content) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return s
	}
	// Then array of blocks.
	var blocks []assistantContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		var b strings.Builder
		for _, blk := range blocks {
			if blk.Type == "text" && blk.Text != "" {
				b.WriteString(blk.Text)
			}
		}
		return b.String()
	}
	return ""
}

func extractAssistantText(raw json.RawMessage) (string, []string) {
	var msg struct {
		Content []assistantContentBlock `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", nil
	}
	var text strings.Builder
	var tools []string
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			if b.Text != "" {
				text.WriteString(b.Text)
				text.WriteString(" ")
			}
		case "tool_use":
			tools = append(tools, fmt.Sprintf("%s(%s)", b.Name, briefInput(b.Input)))
		}
	}
	return strings.TrimSpace(text.String()), tools
}

// briefInput returns a one-line preview of a tool-use input payload.
func briefInput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Pull common fields if present, else fall back to a short raw preview.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range []string{"command", "file_path", "path", "url", "pattern"} {
		if v, ok := m[k]; ok {
			s := fmt.Sprintf("%v", v)
			return truncate(s, 120)
		}
	}
	// Fallback to the first key's value.
	for _, v := range m {
		s := fmt.Sprintf("%v", v)
		return truncate(s, 120)
	}
	return ""
}

// isCCPlumbing returns true for messages that are Claude Code's own UI markers
// rather than something the user actually typed (slash commands, tool results
// that get re-injected as user messages, system reminders, etc).
func isCCPlumbing(s string) bool {
	t := strings.TrimSpace(s)
	prefixes := []string{
		"<local-command-",
		"<command-name>",
		"<command-message>",
		"<command-args>",
		"<system-reminder>",
		"Caveat:",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// sessionIsLive returns true if the events log has any event matching the
// session's UUID in the last hour. If the events log is missing or empty
// (user hasn't run `roost init`), this returns false.
func sessionIsLive(s sessionInfo) bool {
	if s.SessionID == "" {
		return false
	}
	evs, err := events.Replay(2000)
	if err != nil || len(evs) == 0 {
		return false
	}
	cutoff := time.Now().UTC().Add(-time.Hour)
	for i := len(evs) - 1; i >= 0; i-- {
		e := evs[i]
		if e.SessionID != s.SessionID {
			continue
		}
		if e.Timestamp.After(cutoff) {
			return true
		}
		break
	}
	return false
}

// tryLiveRecap attempts the LLM-summarized recap. Returns (true, nil) if it
// printed output, (false, nil) if no API key was configured, and (false, err)
// for genuine errors. Either of the latter two should fall back to the raw
// transcript reader.
func tryLiveRecap(s sessionInfo) (bool, error) {
	tail, err := transcriptTail(s.Path, 80)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(tail) == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := liveRecap(ctx, basenameSafe(s.CWD), tail)
	if err != nil {
		return false, err
	}
	if out == "" {
		// No API key configured. Caller will fall back.
		return false, nil
	}

	branch := s.GitBranch
	if branch == "" {
		branch = "(no branch)"
	}
	fmt.Printf("session %s — %s — %s\n", shortID(s.SessionID), s.CWD, branch)
	fmt.Println(strings.Repeat("─", 72))
	fmt.Println(out)
	fmt.Println(strings.Repeat("─", 72))
	return true, nil
}

// transcriptTail returns the last N lines of the transcript file as a single
// string suitable for embedding in an LLM prompt. Lines are returned in
// chronological order.
func transcriptTail(path string, lines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var all []string
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if lines > 0 && len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}

func basenameSafe(p string) string {
	if p == "" {
		return "(unknown)"
	}
	return filepath.Base(p)
}

// reorderFlagsFirst pulls known flags (and their values for valueFlags) to the
// front of the arg list so stdlib flag.Parse processes them. Unknown args
// remain in their original relative order as positionals.
func reorderFlagsFirst(args []string, valueFlags, boolFlags []string) []string {
	isValueFlag := map[string]bool{}
	for _, f := range valueFlags {
		isValueFlag[f] = true
		isValueFlag[f[1:]] = true   // also accept "n" form internally
		isValueFlag["-"+f[1:]] = true
	}
	isBoolFlag := map[string]bool{}
	for _, f := range boolFlags {
		isBoolFlag[f] = true
	}

	flags := []string{}
	rest := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case isBoolFlag[a]:
			flags = append(flags, a)
		case isValueFlag[a]:
			flags = append(flags, a)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
		case strings.HasPrefix(a, "-") && strings.Contains(a, "="):
			flags = append(flags, a)
		default:
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

func indent(s, prefix string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
