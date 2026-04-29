package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
	"github.com/Thomaspeel6/roost/internal/render"
	"github.com/Thomaspeel6/roost/internal/state"
)

const recentWindow = 30 * time.Minute

func runLs(args []string) int {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	all := fs.Bool("all", false, "show every session ever seen, not just the recent ones")
	noColor := fs.Bool("no-color", os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb", "disable color output")
	maxEvents := fs.Int("max-events", 5000, "max recent events to consider")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	evs, err := events.Replay(*maxEvents)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read events: %v\n", err)
		return 1
	}
	now := time.Now().UTC()
	agents := state.Classify(evs, now)

	if !*all {
		filtered := agents[:0]
		for _, a := range agents {
			if now.Sub(a.LastEvent) <= recentWindow {
				filtered = append(filtered, a)
			}
		}
		agents = filtered
	}

	if len(agents) == 0 && len(evs) == 0 {
		fmt.Fprintln(os.Stderr, "No events recorded yet. Run `roost init` first to install hooks,")
		fmt.Fprintln(os.Stderr, "then start a Claude Code session.")
		return 1
	}
	fmt.Print(render.AgentTable(agents, *noColor))
	return 0
}
