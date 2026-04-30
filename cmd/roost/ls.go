package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Thomaspeel6/roost/internal/events"
	"github.com/Thomaspeel6/roost/internal/render"
	"github.com/Thomaspeel6/roost/internal/state"
)

const defaultRecentWindow = 6 * time.Hour

func runLs(args []string) int {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	all := fs.Bool("all", false, "show every session ever seen, including stale ones")
	noColor := fs.Bool("no-color", os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb", "disable color output")
	maxEvents := fs.Int("max-events", 5000, "max recent events to consider")
	watch := fs.Bool("watch", false, "auto-refresh every second; Ctrl-C to exit")
	interval := fs.Duration("interval", time.Second, "refresh interval for --watch (e.g. 500ms, 2s)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *watch {
		return runLsWatch(*interval, *all, *noColor, *maxEvents)
	}

	return renderOnce(*all, *noColor, *maxEvents, true /*emitErrors*/)
}

func renderOnce(all, noColor bool, maxEvents int, emitErrors bool) int {
	evs, err := events.Replay(maxEvents)
	if err != nil {
		if emitErrors {
			fmt.Fprintf(os.Stderr, "read events: %v\n", err)
		}
		return 1
	}
	now := time.Now().UTC()
	agents := state.Classify(evs, now)

	if !all {
		filtered := agents[:0]
		for _, a := range agents {
			// Always show non-stale agents.
			if a.Status != state.Stale {
				filtered = append(filtered, a)
				continue
			}
			// Only show stale agents if they fired an event in the recent window.
			if now.Sub(a.LastEvent) <= defaultRecentWindow {
				filtered = append(filtered, a)
			}
		}
		agents = filtered
	}

	if len(agents) == 0 && len(evs) == 0 {
		if emitErrors {
			fmt.Fprintln(os.Stderr, "No events recorded yet. Run `roost init` first to install hooks,")
			fmt.Fprintln(os.Stderr, "then start a Claude Code session.")
		}
		return 1
	}
	fmt.Print(render.AgentTable(agents, noColor))
	return 0
}

// runLsWatch refreshes the table every interval until interrupted.
//
// We use ANSI escape codes to clear the screen + move cursor home rather than
// pulling in bubbletea for what is essentially a print-loop. Cleaner exit on
// Ctrl-C, no terminal-mode juggling.
func runLsWatch(interval time.Duration, all, noColor bool, maxEvents int) int {
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	// Hide cursor while watching, restore on exit.
	if !noColor {
		fmt.Print("\033[?25l")
		defer fmt.Print("\033[?25h")
	}

	for {
		// Clear screen + cursor home.
		fmt.Print("\033[2J\033[H")
		fmt.Printf("roost ls --watch  (every %s, Ctrl-C to quit, %s)\n\n", interval, time.Now().Local().Format("15:04:05"))
		_ = renderOnce(all, noColor, maxEvents, false /*emitErrors quietly during watch*/)

		select {
		case <-sigs:
			fmt.Println()
			return 0
		case <-tick.C:
		}
	}
}
