# Roost

**Mission control for parallel Claude Code agents.**

You're running 4 Claude Code sessions across 4 worktrees. Which one needs you? What was each one doing? Roost answers both, in 50ms.

```bash
$ roost ls                    # live status of every active session
AGENT          STATUS    LAST EVENT  BRANCH
auth-branch    blocked   2m ago      main
ui-cleanup     running   12s ago     feature/ui
docs-sweep     idle      1m ago      docs/sweep
infra-bump     stale     8m ago      main

$ roost wake auth-branch      # recap of what that one was doing
WAS DOING: Refactoring the login middleware to use the new token format.
PROGRESS:
- Rewrote token parsing in middleware.go and session.go
- Updated session_test.go; suite passing (12/12)
LAST FINISHED: Ran the full test suite after the refactor.
STATUS: blocked — waiting on permission to run `git push`.
NEXT: Approve the push, or revise the commit message first.
```

## How it works

Two layers, both local:

1. **Transcript layer (no install).** Claude Code writes a JSONL transcript for every session at `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl`. Roost reads them. `roost wake [pattern]` works on every CC session you've ever run — no setup required.

2. **Live layer (one-time install).** `roost init` registers six lifecycle hooks (SessionStart, PreToolUse, PostToolUse, UserPromptSubmit, Stop, Notification) that append events to `~/.roost/events.jsonl`. `roost ls` reads that log to show real-time BLOCKED / RUNNING / IDLE / STALE status. The log is read tail-only and auto-rotates at 16MB, so `ls` stays fast forever.

The classifier:

```
Notification(notification_type=permission_prompt)  → BLOCKED   (needs you NOW)
Stop / Notification(idle_prompt)                   → IDLE      (turn done, ready for a prompt)
PreToolUse / PostToolUse / SessionStart / etc      → RUNNING
RUNNING but no event in 5 min                      → STALE     (probably forgotten)
```

`roost ls` shows sessions with activity in the last 6 hours; `--all` shows everything the log remembers.

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
roost ls --all              show all sessions, however old
roost ls --watch            auto-refresh every second (--interval to tune)
roost wake [pattern]        same as `roost <pattern>`, explicit
roost wake --list           list every transcript on disk, recent first
roost wake -n <num>         show last <num> turns in raw mode (default 6)
roost wake --live           force the LLM recap attempt
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

## LLM recap — no API key required

`roost wake` distills the transcript into a clean conversation digest (user turns, Claude's replies, tool calls — none of the JSONL noise) and asks Claude for the structured answer above (WAS DOING / PROGRESS / LAST FINISHED / STATUS / NEXT). It picks the first engine available:

1. **`claude` CLI on PATH** — headless `claude -p` call that rides your existing Claude Code login. If you use roost, you have this — so recaps work out of the box, no key, no extra cost beyond your subscription.
2. **`ANTHROPIC_API_KEY` set** — direct Claude Haiku 4.5 API call, used if the CLI is missing or fails (~half a cent per recap; override the model with `ROOST_MODEL`).
3. **Neither** — raw transcript tail, no network call.

Skip the LLM: `--no-live`. Force the attempt: `--live`.

## Privacy

- Roost runs entirely on your machine.
- The only network call is the `roost wake` LLM recap, which sends a digest of your transcript to Claude (via your API key, or via your own Claude Code login). Disable with `--no-live`.
- Zero telemetry. No PostHog, no pings, no opt-out needed because there's nothing to opt out of.

## Status

v0.4. Two commands (`ls`, `wake`) plus `init`. Read-only against your CC sessions; the only writes Roost makes are to `~/.roost/events.jsonl` and `~/.claude/settings.json` (only after `roost init`).

If you'd find this useful, [open an issue](https://github.com/Thomaspeel6/roost/issues) — feedback shapes v0.3.

## License

GPL-3.0. Free software: use it, read it, fork it, improve it — improvements stay open. See [LICENSE](LICENSE).

Want to help? See [CONTRIBUTING.md](CONTRIBUTING.md).
