# shellcheck shell=bash
# Sourced by scripts/harness/run.sh and the Claude Code hooks (.claude/hooks/*.sh); needs ROOT set.
# Times one sensor or hook run, appends it to .cache/harness/timings.jsonl and, when the run took longer
# than its row in scripts/harness/budgets.txt, prints one warning line to stderr
# (rules.md#harness-latency-budget). In GitHub Actions the warning is also a `::warning` annotation and a
# line in the job summary.
#
# Runs on every hook call, so it stays cheap (NFR-H1): bash builtins only, plus one perl call per clock
# read on bash < 5 (macOS /bin/bash 3.2 has no EPOCHREALTIME, BSD date has no %N). Hook rows leave `sha`
# empty: `git rev-parse` alone costs ~5 ms. Timing never changes an exit status: whatever cannot be
# measured or written is skipped. HARNESS_BUDGETS overrides the budgets file (the bats tests point it at
# /dev/null so that a loaded machine cannot turn a warning into a test failure).

harness_now_ms() { # wall clock in ms
  if [[ -n "${EPOCHREALTIME:-}" ]]; then
    local t="${EPOCHREALTIME/[.,]/}" # the radix follows the locale
    echo $((t / 1000))
  else
    perl -MTime::HiRes=time -e 'printf "%d\n", time * 1000' 2>/dev/null
  fi
}

harness_seconds() { # ms -> seconds without trailing zeros: 52830 -> 52.83, 50 -> 0.05, 5000 -> 5
  local f
  printf -v f '%03d' $(($1 % 1000))
  while [[ "$f" == *0 ]]; do f="${f%0}"; done
  echo "$(($1 / 1000))${f:+.$f}"
}

# harness_record <sensor|hook> <name> <start-ms> <exit-status>
harness_record() {
  local end ms sha="" scope="hook" budgets id b="" n="" bm nf rest msg
  local timings="$ROOT/.cache/harness/timings.jsonl"
  end="$(harness_now_ms)"
  [[ "$3" =~ ^[0-9]+$ && "$end" =~ ^[0-9]+$ ]] || return 0
  ms=$((end - $3))
  if [[ "$1" == sensor ]]; then
    scope="${HARNESS_SCOPE:-changed}"
    sha="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null)"
  fi
  # One printf per row: appends this short are atomic, so parallel sensors (make -j) do not interleave.
  { [[ -d "${timings%/*}" ]] || mkdir -p "${timings%/*}" 2>/dev/null; } &&
    printf '{"sensor":"%s","start_ms":%s,"ms":%s,"exit":%s,"scope":"%s","sha":"%s","os":"%s"}\n' \
      "$2" "$3" "$ms" "$4" "$scope" "$sha" "$OSTYPE" >>"$timings" 2>/dev/null

  # budgets.txt: "<name> <budget-ms> <nfr-id>"; the "default" row applies to names without a row.
  budgets="${HARNESS_BUDGETS:-$ROOT/scripts/harness/budgets.txt}"
  [[ -r "$budgets" ]] || return 0
  while read -r id bm nf rest; do
    if [[ "$id" == "$2" ]]; then
      b="$bm" n="$nf"
      break
    fi
    [[ "$id" == default ]] && b="$bm" n="$nf"
  done <"$budgets"
  [[ "$b" =~ ^[0-9]+$ ]] && ((ms > b)) || return 0
  msg="harness: $1 '$2' took $(harness_seconds "$ms") s (budget $(harness_seconds "$b") s, $n)"
  echo "$msg" >&2
  if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
    echo "::warning title=harness-latency-budget::$msg"
    [[ -n "${GITHUB_STEP_SUMMARY:-}" ]] && echo "- $msg" >>"$GITHUB_STEP_SUMMARY" 2>/dev/null
  fi
  return 0
}

# harness_time_exit <sensor|hook> <name>: records the calling script's run when it exits, keeping its status.
harness_time_exit() {
  _harness_start="$(harness_now_ms)"
  # shellcheck disable=SC2064 # reason: kind and name are fixed when the trap is set (rule: sh-shellcheck)
  trap "_harness_on_exit $1 $2" EXIT
}

_harness_on_exit() { # sensor|hook name
  local rc=$?        # the script's exit status
  harness_record "$1" "$2" "$_harness_start" "$rc"
  exit "$rc"
}
