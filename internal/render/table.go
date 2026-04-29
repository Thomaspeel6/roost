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
func AgentTable(agents []state.Agent, noColor bool) string {
	if len(agents) == 0 {
		return "No agents seen yet. Start a Claude Code session, then run `roost ls` again.\n"
	}

	rows := make([][]string, 0, len(agents))
	for _, a := range agents {
		rows = append(rows, []string{a.Name, a.Status.String(), since(a.LastEvent), branchOrDash(a.GitBranch)})
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	case state.Done:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // green
	case state.Idle:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber for running
	}
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

