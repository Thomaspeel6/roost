// Command roost-hook is the tiny binary Claude Code invokes via its hook
// configuration. It reads a JSON payload from stdin, parses it, appends a
// roost Event to ~/.roost/events.jsonl, and exits.
//
// Errors are logged to stderr and exit is always 0 — Claude Code must never
// be blocked by a roost-hook failure.
package main

import (
	"fmt"
	"os"

	"github.com/Thomaspeel6/roost/internal/adapters/cc"
	"github.com/Thomaspeel6/roost/internal/events"
)

func main() {
	e, err := cc.Parse(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "roost-hook: parse: %v\n", err)
		os.Exit(0)
	}
	if err := events.Append(e); err != nil {
		fmt.Fprintf(os.Stderr, "roost-hook: append: %v\n", err)
		os.Exit(0)
	}
}
