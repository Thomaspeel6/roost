package main

import (
	"fmt"
	"os"
)

var version = "0.0.1-dev" // overridden at release time via -ldflags "-X main.version=..."

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
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
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`roost - mission control for parallel Claude Code agents

usage:
  roost init           install Claude Code hooks
  roost ls             list running agents
  roost wake <name>    recap of one agent's last task
  roost version        print version
  roost help           show this message

privacy:
  Roost runs locally. Anonymous opt-in usage telemetry is OFF by default.
  Disable explicitly: touch ~/.roost/telemetry-off`)
}
