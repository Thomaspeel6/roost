// Package render holds the small bit of presentation code Roost needs.
package render

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/Thomaspeel6/roost/internal/state"
)

// AgentTable renders agents as a sorted, color-coded table. If noColor is
// true the output has no ANSI codes (suitable for piping or NO_COLOR envs).
//
// When multiple agents share a Name (two CC sessions in the same project
// directory), each duplicate gets a session-id short prefix appended in
// parentheses so they're distinguishable in the table.
func AgentTable(agents []state.Agent, noColor bool) string {
	if len(agents) == 0 {
		return "No active sessions. Try `roost ls --all` to see everything ever seen.\n"
	}

	displayNames := disambiguateNames(agents)

	rows := make([][]string, 0, len(agents))
	for i, a := range agents {
		rows = append(rows, []string{displayNames[i], a.Status.String(), since(a.LastEvent), branchOrDash(a.GitBranch)})
	}

	t := table.New().
		Headers("AGENT", "STATUS", "LAST EVENT", "BRANCH").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240")))

	if !noColor {
		t = t.StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true)
			}
			if row >= 0 && row < len(agents) && col == 1 {
				return statusColor(agents[row].Status)
			}
			return lipgloss.NewStyle()
		})
	}
	return t.Render() + "\n"
}

func statusColor(s state.Status) lipgloss.Style {
	switch s {
	case state.Blocked:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red — needs you NOW
	case state.Running:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber — actively working
	case state.Idle:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // green — ready, waiting for prompt
	case state.Stale:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray — forgotten
	default:
		return lipgloss.NewStyle()
	}
}

// disambiguateNames returns one display string per agent; identical names get
// a short suffix (session-id prefix or branch name) so the table is readable.
func disambiguateNames(agents []state.Agent) []string {
	counts := map[string]int{}
	for _, a := range agents {
		counts[a.Name]++
	}
	out := make([]string, len(agents))
	for i, a := range agents {
		if counts[a.Name] <= 1 {
			out[i] = a.Name
			continue
		}
		// Prefer branch as discriminator if non-empty and not "-"; else use
		// short session id; else fall back to bare name.
		discriminator := ""
		if a.GitBranch != "" {
			discriminator = a.GitBranch
		} else if a.SessionID != "" {
			discriminator = a.SessionID
			if len(discriminator) > 8 {
				discriminator = discriminator[:8]
			}
		}
		if discriminator == "" {
			out[i] = a.Name
		} else {
			out[i] = a.Name + " (" + discriminator + ")"
		}
	}
	return out
}

func since(t time.Time) string {
	d := time.Since(t).Round(time.Second)
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

func branchOrDash(b string) string {
	if b == "" {
		return "-"
	}
	if len(b) > 30 {
		return b[:27] + "..."
	}
	return b
}

