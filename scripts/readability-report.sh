#!/usr/bin/env bash
# Prints the readability scorecard from the repo-professionalization roadmap:
# oversized Go files and functions, internal fan-out per cometmind package, and
# oversized Svelte components and stores. Generated code is excluded.
#
# Usage: scripts/readability-report.sh
# In GitHub Actions the report is also appended to $GITHUB_STEP_SUMMARY.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_FILE_BUDGET=500
GO_FUNC_BUDGET=80
FAN_OUT_BUDGET=10
SVELTE_BUDGET=400
SVELTE_TS_BUDGET=500
TOP=10

# Packages allowed to exceed the fan-out budget because they wire everything.
COMPOSITION_ROOTS='^(\.|cmd|internal/server|internal/runtime|internal/tools)$'

go_metrics() {
  go run ./scripts/readability/main.go comet-sdk cometmind
}

fan_out() {
  (
    cd cometmind
    local module
    module="$(go list -m)"
    go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./... |
      awk -v mod="$module" '{
        n = 0
        for (i = 2; i <= NF; i++) if (index($i, mod "/") == 1) n++
        pkg = $1 == mod ? "." : substr($1, length(mod) + 2)
        print n, pkg
      }'
  ) | sort -k1,1nr -k2,2
}

svelte_sizes() {
  local pattern=$1
  [ -d cometline/src ] || return 0
  find cometline/src -name "$pattern" -not -path '*/generated/*' -print0 |
    xargs -0 wc -l |
    awk '$2 != "total" { print $1, $2 }' |
    sort -k1,1nr
}

over() {
  awk -v limit="$1" '$1 > limit'
}

count_lines() {
  if [ -z "$1" ]; then echo 0; else printf '%s\n' "$1" | wc -l | tr -d ' '; fi
}

indent_top() {
  if [ -n "$1" ]; then printf '%s\n' "$1" | head -n "$TOP" | sed 's/^/    /'; fi
}

report() {
  local metrics files funcs fanout roots_out svelte svelte_ts
  metrics="$(go_metrics)"
  files="$(printf '%s\n' "$metrics" | awk '$1 == "FILE" { print $2, $3 }' | sort -k1,1nr | over "$GO_FILE_BUDGET")"
  funcs="$(printf '%s\n' "$metrics" | awk '$1 == "FUNC" { print $2, $3, $4 }' | sort -k1,1nr | over "$GO_FUNC_BUDGET")"
  fanout="$(fan_out)"
  roots_out="$(printf '%s\n' "$fanout" | awk -v re="$COMPOSITION_ROOTS" '$2 !~ re')"
  svelte="$(svelte_sizes '*.svelte' | over "$SVELTE_BUDGET")"
  svelte_ts="$(svelte_sizes '*.svelte.ts' | over "$SVELTE_TS_BUDGET")"

  local largest_func max_fan
  largest_func="$(printf '%s\n' "$funcs" | head -n 1 | awk 'NF { print $1 " (" $2 ")" }')"
  max_fan="$(printf '%s\n' "$roots_out" | head -n 1 | awk 'NF { print $1 " (" $2 ")" }')"

  echo "Readability report @ $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
  echo
  echo "Scorecard (generated code and Go tests excluded)"
  printf '  %-58s %s\n' "Go files over ${GO_FILE_BUDGET} LOC:" "$(count_lines "$files")"
  printf '  %-58s %s\n' "Go functions over ${GO_FUNC_BUDGET} body lines:" "$(count_lines "$funcs")"
  printf '  %-58s %s\n' "Largest Go function (body lines):" "${largest_func:-none}"
  printf '  %-58s %s\n' "Highest cometmind fan-out outside composition roots:" "${max_fan:-none}"
  printf '  %-58s %s\n' "cometmind packages over ${FAN_OUT_BUDGET} internal imports:" \
    "$(count_lines "$(printf '%s\n' "$roots_out" | over "$FAN_OUT_BUDGET")")"
  printf '  %-58s %s\n' "Svelte components over ${SVELTE_BUDGET} LOC:" "$(count_lines "$svelte")"
  printf '  %-58s %s\n' ".svelte.ts modules over ${SVELTE_TS_BUDGET} LOC:" "$(count_lines "$svelte_ts")"
  echo
  echo "Largest Go files (top ${TOP}):"
  indent_top "$files"
  echo "Longest Go functions (top ${TOP}):"
  indent_top "$funcs"
  echo "cometmind internal fan-out (top ${TOP}, composition roots included):"
  indent_top "$fanout"
  echo "Largest Svelte components (top ${TOP}):"
  indent_top "$svelte"
  echo "Largest .svelte.ts modules (top ${TOP}):"
  indent_top "$svelte_ts"
}

output="$(report)"
printf '%s\n' "$output"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo '## Readability report'
    echo
    echo '```text'
    printf '%s\n' "$output"
    echo '```'
  } >> "$GITHUB_STEP_SUMMARY"
fi
