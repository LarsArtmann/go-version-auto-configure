#!/usr/bin/env bash
# license-check-isolation — T16-2 / plan M15 harness.
# Runs BuildFlow's license-check step N times in this repo and records
# pass/fail per run, so the intermittent go-licenses std-package failure
# (google/go-licenses#128 class) can be correlated with a factor instead of
# guessed at. Output: one line per run (n, exit, seconds) + a verdict line.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

runs="${1:-5}"
pass=0
fail=0
fail_runs=""

for i in $(seq 1 "$runs"); do
  start=$(date +%s)
  out=$(nix develop -c buildflow -s license-check --format finding 2>&1)
  rc=$?
  end=$(date +%s)
  echo "run $i: exit=$rc seconds=$((end - start))"
  if [ "$rc" -eq 0 ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    fail_runs="$fail_runs $i"
    echo "$out" | rg -o "does not have module info[^\"]*" | sort -u >"/tmp/license-fail-$i.txt"
  fi
done

echo "verdict: $pass/$runs passed; failed runs:$fail_runs"
if [ "$fail" -gt 0 ]; then
  echo "failure packages per run:"
  for f in /tmp/license-fail-*.txt; do
    [ -e "$f" ] && echo "--- $f" && cat "$f"
  done
fi
