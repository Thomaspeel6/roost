# Roost

**Mission control for parallel Claude Code agents.**

You're running 4 Claude Code sessions across 4 worktrees. Which one needs you? What was each one doing? Roost answers both, in 50ms.

```bash
$ roost ls                    # live status of every active session
AGENT          STATUS    LAST EVENT  BRANCH
auth-branch    blocked   2m ago      main
ui-cleanup     running   12s ago     feature/ui
docs-sweep     done      1m ago      docs/sweep
infra-bump     idle      8m ago      main

$ roost wake auth-branch      # 4-line recap of what that one was doing
WAS DOING:    Refactoring login middleware to use the new token format.
LAST FINISHED: Updated session_test.go and ran the suite (12/12 passing).
STATUS:       blocked — waiting on permission to run `git push`.
NEXT:         Approve the push, or revise the commit message.
```

## How it works

Two layers, both local:

1. **Transcript layer (no install).** Claude Code writes a JSONL transcript for every session at `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl`. Roost reads them. `roost wake [pattern]` works on every CC session you've ever run — no setup required.

2. **Live layer (one-time install).** `roost init` registers six lifecycle hooks (SessionStart, PreToolUse, PostToolUse, UserPromptSubmit, Stop, Notification) that append events to `~/.roost/events.jsonl`. `roost ls` reads that log to show real-time BLOCKED / RUNNING / DONE / IDLE status.

The classifier:

```
Notification(notification_type=idle_prompt)        → BLOCKED
Stop                                               → DONE
PreToolUse / PostToolUse / SessionStart / etc      → RUNNING
No event in 5 min                                  → IDLE
```

## Install

### Homebrew (macOS / Linux)

```bash
brew install Thomaspeel6/tap/roost
roost init      # one-time, installs CC hooks
```

### Direct binary download

[github.com/Thomaspeel6/roost/releases](https://github.com/Thomaspeel6/roost/releases) — grab a tarball, drop `roost` and `roost-hook` into `/usr/local/bin/`.

### From source

```bash
go install github.com/Thomaspeel6/roost/cmd/roost@latest
go install github.com/Thomaspeel6/roost/cmd/roost-hook@latest
```

## Usage

```
roost                       recap most recent session in this directory
roost <pattern>             recap most recent session matching <pattern>
roost ls                    live status table for currently-active sessions
roost ls --all              show all sessions including idle ones
roost wake [pattern]        same as `roost <pattern>`, explicit
roost wake --list           list every transcript on disk, recent first
roost wake -n <num>         show last <num> turns (default 6)
roost wake --live           force the LLM live recap
roost wake --no-live        always raw transcript, never LLM
roost init                  install Claude Code hooks (required for `ls`)
roost init --uninstall      remove the hooks
roost version               print version
roost help                  show this message
```

`<pattern>` is a substring match against the session's `cwd`, `gitBranch`, or session UUID. Examples:

```bash
roost                              # current directory
roost auth-branch                  # any session whose worktree path matches
roost moveo                        # any session in a moveo project
roost wake b09b381e                # session UUID prefix
```

## LLM live recap (optional)

If `ANTHROPIC_API_KEY` is set in your environment and a session has events in the last hour, `roost wake` prefers a Claude Haiku 4.5 call to produce the structured 4-line answer (WAS DOING / LAST FINISHED / STATUS / NEXT). Without the key, you get the raw transcript tail.

Skip the LLM call: `--no-live`. Force it: `--live`. Costs about half a cent per recap.

## Privacy

- Roost runs entirely on your machine.
- The only network call is the optional `roost wake` LLM recap, which sends your transcript tail to Anthropic's API. Disable with `--no-live` or by unsetting `ANTHROPIC_API_KEY`.
- Zero telemetry. No PostHog, no pings, no opt-out needed because there's nothing to opt out of.

## Status

v0.2. Two commands (`ls`, `wake`) plus `init`. Read-only against your CC sessions; the only write Roost makes is to `~/.roost/events.jsonl` and `~/.claude/settings.json` (only after `roost init`).

If you'd find this useful, [open an issue](https://github.com/Thomaspeel6/roost/issues) — feedback shapes v0.3.

## License

MIT.
