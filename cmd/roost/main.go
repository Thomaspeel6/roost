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
	case "wake":
		os.Exit(runWake(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		// Treat any unknown first arg as a pattern for `roost wake`.
		// This makes `roost auth-branch` work as shorthand for `roost wake auth-branch`.
		os.Exit(runWake(os.Args[1:]))
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `roost — recap any past Claude Code session

usage:
  roost                       recap most recent session in this directory
  roost <pattern>             recap most recent session matching <pattern>
  roost wake [pattern]        same as above, explicit
  roost wake --list           list available sessions, most recent first
  roost wake -n <num>         show last <num> turns (default 6)
  roost version               print version
  roost help                  show this message

how it works:
  reads Claude Code's per-session transcripts at ~/.claude/projects/.
  no install, no hooks, no daemon. local only.

privacy:
  no network calls. no telemetry.`)
}
