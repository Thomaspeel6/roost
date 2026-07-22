# Contributing to Roost

Thanks for wanting to help. Roost is a small, local-first tool for people who run
several Claude Code sessions in parallel, and contributions that keep it fast,
simple, and trustworthy are very welcome.

## Ground rules

- **Keep changes focused.** One PR = one fix or feature. Small, reviewable diffs
  get merged; sprawling ones stall.
- **Stay in the spirit of the project:** local-first, no daemon, no telemetry,
  nothing that phones home. The only network call Roost makes is the recap the
  user explicitly asks for.
- **Don't add dependencies lightly.** Roost talks to the Anthropic API with
  stdlib `net/http` on purpose. Prefer the standard library; if a few lines do
  the job, write the few lines.
- **Keep it fast.** `roost ls` should answer in well under a second no matter
  how big `~/.roost/events.jsonl` has grown. Anything that reads whole logs or
  whole transcripts when a tail would do will be asked to change.
- **No secrets in commits.** Roost never stores credentials; it reads
  `ANTHROPIC_API_KEY` from the environment or rides the user's `claude` login.

## Submitting a pull request

1. Fork the repo and create a branch off `main`.
2. Make your change, then run the checks CI runs:
   ```bash
   gofmt -l .        # should print nothing
   go vet ./...
   go test -race ./...
   go build ./cmd/roost ./cmd/roost-hook
   ```
3. Try it for real: `go build -o roost ./cmd/roost && ./roost ls && ./roost wake`.
   Roost's test bed is your own Claude Code history — use it.
4. Open a PR against `main` with a short description of *what* changed and
   *why*. Paste terminal output for anything user-visible.
5. I'll review as soon as I can. Expect a few rounds of feedback — that's
   normal, not a rejection.

Good first contributions: bug fixes, recap-prompt improvements, better handling
of odd transcript shapes, Windows/Linux quirks, and docs. If you want to tackle
something larger (new commands, new adapters beyond Claude Code), open an issue
first so we can agree on the shape before you spend time on it.

## Reporting bugs / requesting features

Open a [GitHub issue](https://github.com/Thomaspeel6/roost/issues) with:

- what you expected, what actually happened, and steps to reproduce
- your OS, and the output of `roost version`
- for status bugs: the relevant lines of `~/.roost/events.jsonl` (they contain
  only hook names, paths, branch names, and timestamps — no conversation content)

## Getting in touch

For anything that doesn't fit an issue or PR — questions, ideas, "is this worth
doing before I build it" — open an issue and say so, or reach me via
[@Thomaspeel6](https://github.com/Thomaspeel6).

I'd rather you ask first than burn a weekend on something I can't merge. Don't
be shy.
