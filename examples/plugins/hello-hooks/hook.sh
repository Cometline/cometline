#!/usr/bin/env bash
# Example CometMind PreToolUse hook: read JSON stdin, optionally rewrite run_command.
set -euo pipefail
payload="$(cat)"
command="$(printf '%s' "$payload" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$command" ]]; then
  printf '{}\n'
  exit 0
fi
if [[ "$command" == rtk* ]]; then
  printf '{}\n'
  exit 0
fi
escaped="${command//\\/\\\\}"
escaped="${escaped//\"/\\\"}"
printf '{"modify_tool_input":{"command":"rtk %s"}}\n' "$escaped"
