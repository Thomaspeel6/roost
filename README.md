# Roost

**Recap any past Claude Code session in 50ms.**

You ran a CC session two hours ago in another tab. What was it doing? What was the last thing it tried? What did you ask it to do? Open the tab, scroll, squint — or:

```bash
$ roost                       # most recent session in this directory
$ roost kalshi                # most recent session matching "kalshi"
$ roost wake --list           # every session you've ever run, recent first
```

## How it works

Claude Code already writes a JSONL transcript for every session at `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl`. Roost reads them. That's it.

- **No install.** No hooks. No daemon. No background process.
- **Works on every past session**, not just future ones.
- **No network calls.** No telemetry. Local only.
- **One static Go binary, ~3MB.**

## Install

### Homebrew (macOS / Linux)

```bash
brew install Thomaspeel6/tap/roost
```

### Direct binary download

Pick a release from [github.com/Thomaspeel6/roost/releases](https://github.com/Thomaspeel6/roost/releases), unpack, drop in `/usr/local/bin/`:

```bash
# macOS arm64 example
curl -sSL https://github.com/Thomaspeel6/roost/releases/latest/download/roost_0.1.0_darwin_arm64.tar.gz | tar xz
sudo mv roost /usr/local/bin/
```

### From source

```bash
go install github.com/Thomaspeel6/roost/cmd/roost@latest
```

## Usage

```
roost                       recap most recent session in this directory
roost <pattern>             recap most recent session matching <pattern>
roost wake [pattern]        same as above, explicit
roost wake --list           list available sessions, most recent first
roost wake -n <num>         show last <num> turns (default 6)
roost version               print version
roost help                  show this message
```

`<pattern>` is a substring match against the session's `cwd`, `gitBranch`, or session UUID. Examples:

```bash
roost                              # current directory
roost auth-branch                  # any session whose worktree path matches
roost moveo                        # any session in a moveo project
roost wake b09b381e                # session UUID prefix
roost wake -n 20                   # show last 20 turns
```

## Why

Running 4 Claude Code agents in parallel is great until you switch back to one and have to scroll 200 lines to remember what it was doing. Roost gives you the recap in one command.

## Status

v0. Single command. Read-only. Works for the author and at least 2 other Claude Code power users. If you'd find this useful, [open an issue](https://github.com/Thomaspeel6/roost/issues) — feedback shapes v0.1.

## License

MIT.
