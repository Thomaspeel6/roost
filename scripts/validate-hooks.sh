#!/usr/bin/env bash
# Day-1 hook semantics validation for Roost.
# Installs Claude Code hooks that dump every payload to ~/.roost-validation/hook-payloads.jsonl
# so we can verify that hooks expose enough state to distinguish BLOCKED / IDLE / DONE / RUNNING.
#
# Usage:
#   ./scripts/validate-hooks.sh install     # install validation hooks (backs up your settings)
#   ./scripts/validate-hooks.sh analyze     # summarize captured payloads
#   ./scripts/validate-hooks.sh restore     # restore your original settings
set -euo pipefail

OUT="$HOME/.roost-validation/hook-payloads.jsonl"
SETTINGS="$HOME/.claude/settings.json"
BACKUP="$HOME/.claude/settings.json.roost-backup"

cmd="${1:-install}"

case "$cmd" in
  install)
    mkdir -p "$HOME/.roost-validation"
    : > "$OUT"

    if [ -f "$SETTINGS" ] && [ ! -f "$BACKUP" ]; then
      cp "$SETTINGS" "$BACKUP"
      echo "Backed up existing settings → $BACKUP"
    fi

    cat > "$SETTINGS" <<'JSON'
{
  "hooks": {
    "SessionStart":     [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"SessionStart\",     ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}],
    "PreToolUse":       [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"PreToolUse\",       ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}],
    "PostToolUse":      [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"PostToolUse\",      ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"UserPromptSubmit\", ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}],
    "Stop":             [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"Stop\",             ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}],
    "Notification":     [{"hooks": [{"type": "command", "command": "jq -c '. + {hook: \"Notification\",     ts: now}' >> ~/.roost-validation/hook-payloads.jsonl"}]}]
  }
}
JSON

    echo ""
    echo "Validation hooks installed. Now exercise Claude Code through these 5 scenarios:"
    echo ""
    echo "  1. Start a session, ask 'what's 2+2'.  (covers SessionStart, UserPromptSubmit, Stop)"
    echo "  2. Ask Claude to run a Bash command.  (covers PreToolUse, PostToolUse)"
    echo "  3. Ask Claude to do something that triggers a permission prompt"
    echo "     (e.g. 'rm a file', 'git push'). DO NOT approve immediately — wait 10s."
    echo "     (covers Notification?)"
    echo "  4. Approve the prompt; let it complete; quit cleanly with /exit. (Stop fires?)"
    echo "  5. Start another session, kill it mid-response with Ctrl-C. (Stop fires on crash?)"
    echo ""
    echo "When done, run:  $0 analyze"
    ;;

  analyze)
    if [ ! -s "$OUT" ]; then
      echo "No payloads captured at $OUT. Did you run a Claude Code session?"
      exit 1
    fi

    echo "=== Hook frequency ==="
    jq -s 'group_by(.hook) | map({hook: .[0].hook, count: length}) | sort_by(.count) | reverse' "$OUT"

    echo ""
    echo "=== Sample payload per hook (first occurrence) ==="
    jq -s 'group_by(.hook) | map({hook: .[0].hook, sample: .[0]})' "$OUT"

    echo ""
    echo "=== Fields available per hook ==="
    jq -s 'group_by(.hook) | map({hook: .[0].hook, keys: (.[0] | keys)})' "$OUT"

    echo ""
    echo "Now answer in ~/.roost-validation/decision.md:"
    echo "  1. Can we detect BLOCKED?  (a hook that fires only when CC waits on user)"
    echo "  2. Can we detect DONE?     (Stop fires reliably on clean exit AND on Ctrl-C?)"
    echo "  3. Can we detect IDLE vs RUNNING?  (any signal of 'thinking' vs 'tool running'?)"
    echo ""
    echo "If yes to all 3 → continue with Task 2."
    echo "If BLOCKED is undetectable → STOP. Reshape product (drop status sort, lean on roost wake)."
    ;;

  restore)
    if [ -f "$BACKUP" ]; then
      mv "$BACKUP" "$SETTINGS"
      echo "Restored $SETTINGS from backup."
    else
      rm -f "$SETTINGS"
      echo "No backup existed. Removed $SETTINGS (it was created by validation)."
    fi
    ;;

  *)
    echo "usage: $0 {install|analyze|restore}"
    exit 2
    ;;
esac
