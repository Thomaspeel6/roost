package main

import (
	"fmt"
	"os"
)

var version = "0.0.1-dev" // overridden at release time via -ldflags "-X main.version=..."

func main() {
	if len(os.Args) < 2 {
		os.Exit(runWake(nil))
	}
	switch os.Args[1] {
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "ls":
		os.Exit(runLs(os.Args[2:]))
	case "wake":
		os.Exit(runWake(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		// Treat any unknown first arg as a pattern for `roost wake`.
		// `roost auth-branch` becomes shorthand for `roost wake auth-branch`.
		os.Exit(runWake(os.Args[1:]))
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `roost — mission control for parallel Claude Code agents

usage:
  roost                       recap most recent session in this directory
  roost <pattern>             recap most recent session matching <pattern>
  roost ls                    live status table for active sessions
  roost ls --watch            auto-refresh every second
  roost ls --all              include sessions that have gone stale
  roost wake [pattern]        same as `+"`roost <pattern>`"+`, explicit
  roost wake --list           list every transcript on disk, recent first
  roost wake -n <num>         show last <num> turns (default 6)
  roost init                  install Claude Code hooks (required for `+"`ls`"+`)
  roost init --uninstall      remove the hooks
  roost version               print version
  roost help                  show this message

how it works:
  reads Claude Code's per-session transcripts at ~/.claude/projects/.
  with `+"`roost init`"+`, additionally watches lifecycle hooks for live status.
  no daemon. no network calls (unless ANTHROPIC_API_KEY is set, then `+"`wake`"+`
  uses Claude Haiku for a structured 4-line recap of live sessions).

privacy:
  no telemetry. local only.`)
}
