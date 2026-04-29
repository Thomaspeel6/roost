package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Thomaspeel6/roost/internal/adapters/cc"
)

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	uninstall := fs.Bool("uninstall", false, "remove roost hooks from CC settings")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	hookBin, err := exec.LookPath("roost-hook")
	if err != nil {
		fmt.Fprintln(os.Stderr, "roost-hook binary not found in PATH.")
		fmt.Fprintln(os.Stderr, "Install via:  brew install Thomaspeel6/tap/roost")
		fmt.Fprintln(os.Stderr, "Or build:    go build -o roost-hook ./cmd/roost-hook")
		return 1
	}
	hookBin, _ = filepath.Abs(hookBin)

	settingsPath := claudeSettingsPath()

	if *uninstall {
		if err := cc.Uninstall(settingsPath, hookBin); err != nil {
			fmt.Fprintf(os.Stderr, "uninstall: %v\n", err)
			return 1
		}
		fmt.Println("Roost hooks removed from", settingsPath)
		return 0
	}

	if err := cc.Install(settingsPath, hookBin); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	fmt.Println("Roost hooks installed at", settingsPath)
	fmt.Println("Start a Claude Code session, then run `roost ls` to see live status.")
	return 0
}

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}
