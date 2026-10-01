#!/usr/bin/env bash
# Prints the readability scorecard from the repo-professionalization roadmap:
# oversized Go files and functions, internal fan-out per cometmind package, and
# oversized Svelte components, scoped CSS, and stores. Generated code is excluded.
#
# Usage:
#   scripts/readability-report.sh           print the scorecard (always exits 0)
#   scripts/readability-report.sh --check   print the scorecard, then fail if a
#                                           size or fan-out budget regresses or
#                                           a funlen/gocyclo offender is missing
#                                           from scripts/readability-allowlist.txt
#
# In GitHub Actions the report is also appended to $GITHUB_STEP_SUMMARY.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_FILE_BUDGET=500
GO_FUNC_BUDGET=80
FAN_OUT_BUDGET=10
SVELTE_BUDGET=400
SVELTE_CSS_BUDGET=200
SVELTE_TS_BUDGET=500
TOP=10
ALLOWLIST="$ROOT/scripts/readability-allowlist.txt"
CHECK=0
if [ "${1:-}" = "--check" ]; then
  CHECK=1
fi

# Packages allowed to exceed the fan-out budget because they wire everything.
# internal/tools is the tool-registry composition root. depguard does not
# exempt it; this list only exempts the fan-out scorecard.
COMPOSITION_ROOTS='^(\.|cmd|internal/server|internal/runtime|internal/tools)$'

if [ -z "${GOLANGCI_LINT:-}" ]; then
  GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"
fi

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

# Lines inside <style> blocks, summed per component. Tags themselves are not
# counted. Generated trees are excluded.
svelte_css_sizes() {
  [ -d cometline/src ] || return 0
  find cometline/src -name '*.svelte' -not -path '*/generated/*' -print0 |
    xargs -0 awk '
      FNR == 1 {
        if (filename != "") print count, filename
        filename = FILENAME
        count = 0
        in_style = 0
      }
      {
        if (!in_style && index($0, "<style") > 0) { in_style = 1; next }
        if (in_style && index($0, "</style>") > 0) { in_style = 0; next }
        if (in_style) count++
      }
      END { if (filename != "") print count, filename }
    ' |
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

collect_metrics() {
  local metrics
  metrics="$(go_metrics)"
  files="$(printf '%s\n' "$metrics" | awk '$1 == "FILE" { print $2, $3 }' | sort -k1,1nr | over "$GO_FILE_BUDGET")"
  funcs="$(printf '%s\n' "$metrics" | awk '$1 == "FUNC" { print $2, $3, $4 }' | sort -k1,1nr | over "$GO_FUNC_BUDGET")"
  fanout="$(fan_out)"
  roots_out="$(printf '%s\n' "$fanout" | awk -v re="$COMPOSITION_ROOTS" '$2 !~ re')"
  svelte="$(svelte_sizes '*.svelte' | over "$SVELTE_BUDGET")"
  svelte_css="$(svelte_css_sizes | over "$SVELTE_CSS_BUDGET")"
  svelte_ts="$(svelte_sizes '*.svelte.ts' | over "$SVELTE_TS_BUDGET")"
}

report() {
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
  printf '  %-58s %s\n' "Svelte scoped CSS over ${SVELTE_CSS_BUDGET} LOC:" "$(count_lines "$svelte_css")"
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
  echo "Largest scoped CSS blocks (top ${TOP}):"
  indent_top "$svelte_css"
  echo "Largest .svelte.ts modules (top ${TOP}):"
  indent_top "$svelte_ts"
}

size_keys() {
  printf '%s\n' "$files" | awk 'NF { print "file", $2 }'
  printf '%s\n' "$funcs" | awk 'NF { path = $3; sub(/:[0-9]+$/, "", path); print "func", path, $2 }'
  printf '%s\n' "$roots_out" | over "$FAN_OUT_BUDGET" | awk 'NF { print "fanout", $2 }'
  printf '%s\n' "$svelte" | awk 'NF { print "svelte", $2 }'
  printf '%s\n' "$svelte_css" | awk 'NF { print "svelte-css", $2 }'
  printf '%s\n' "$svelte_ts" | awk 'NF { print "svelte-ts", $2 }'
}

# Prints "kind path symbol" lines for funlen and gocyclo, prefixed with the module.
budget_keys() {
  local mod stdout_file stderr_file code
  for mod in comet-sdk cometmind; do
    stdout_file="$(mktemp)"
    stderr_file="$(mktemp)"
    set +e
    # Unquoted so "go run <module>@version" splits into a command.
    (cd "$ROOT/$mod" && NO_COLOR=1 $GOLANGCI_LINT run -c .golangci.budget.yml ./...) \
      >"$stdout_file" 2>"$stderr_file"
    code=$?
    set -e
    if [ "$code" -ne 0 ] && [ "$code" -ne 1 ]; then
      echo "golangci-lint failed in $mod (exit $code)" >&2
      cat "$stderr_file" >&2
      cat "$stdout_file" >&2
      rm -f "$stdout_file" "$stderr_file"
      exit 1
    fi
    sed -n -E \
      -e "s#^([^:]+):[0-9]+:[0-9]+: cyclomatic complexity [0-9]+ of func \`([^\`]+)\`.*\\(gocyclo\)\$#gocyclo ${mod}/\\1 \\2#p" \
      -e "s#^([^:]+):[0-9]+:[0-9]+: Function '([^']+)' is too long .*\\(funlen\)\$#funlen ${mod}/\\1 \\2#p" \
      "$stdout_file"
    rm -f "$stdout_file" "$stderr_file"
  done
}

allowlist_keys() {
  awk '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      n = index($0, " | ")
      if (n == 0) {
        printf "allow-list line needs \" | reason\": %s\n", $0 > "/dev/stderr"
        exit 2
      }
      key = substr($0, 1, n - 1)
      reason = substr($0, n + 3)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", reason)
      if (key == "" || reason == "") {
        printf "allow-list line needs a key and a reason: %s\n", $0 > "/dev/stderr"
        exit 2
      }
      print key
    }
  ' "$ALLOWLIST"
}

enforce() {
  if ! printf '%s\n' 'internal/tools' | grep -Eq "$COMPOSITION_ROOTS"; then
    echo "internal/tools must stay a fan-out composition root (tool registry)" >&2
    exit 1
  fi

  local actual expected new_offenders stale dupes
  actual="$(
    {
      size_keys
      budget_keys
    } | awk 'NF' | sort -u
  )"
  expected="$(allowlist_keys | sort)"
  dupes="$(printf '%s\n' "$expected" | uniq -d)"
  if [ -n "$dupes" ]; then
    echo "Duplicate readability allow-list entries:" >&2
    printf '%s\n' "$dupes" | sed 's/^/  /' >&2
    exit 1
  fi
  new_offenders="$(comm -23 <(printf '%s\n' "$actual") <(printf '%s\n' "$expected"))"
  stale="$(comm -13 <(printf '%s\n' "$actual") <(printf '%s\n' "$expected"))"
  if [ -n "$new_offenders" ] || [ -n "$stale" ]; then
    echo "Readability check failed." >&2
    if [ -n "$new_offenders" ]; then
      echo "New offenders (fix them, or add a line to scripts/readability-allowlist.txt with a one-line reason):" >&2
      printf '%s\n' "$new_offenders" | sed 's/^/  /' >&2
    fi
    if [ -n "$stale" ]; then
      echo "Stale allow-list entries (remove them from scripts/readability-allowlist.txt):" >&2
      printf '%s\n' "$stale" | sed 's/^/  /' >&2
    fi
    exit 1
  fi
  echo "Readability check passed ($(printf '%s\n' "$expected" | awk 'NF' | wc -l | tr -d ' ') allow-listed funlen/gocyclo exceptions)."
}

collect_metrics
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

if [ "$CHECK" -eq 1 ]; then
  enforce
fi
