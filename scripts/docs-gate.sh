#!/usr/bin/env bash
# docs-gate — standing documentation health check for this repo.
# Wired as flake app `docs-gate` (nix run .#docs-gate); lychee comes from the
# flake's nixpkgs. Three checks:
#   1. link check (README, CHANGELOG, docs/) — hard gate
#   2. TODO_LIST hygiene: no checked boxes ([x]) — completed items move to
#      CHANGELOG (docs-health policy) — hard gate
#   3. stale-report probe: status reports older than 21 days that still sit in
#      docs/status/ root (not archived/) — advisory (harvest then archive)
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

fail=0

echo "== link check (lychee) =="
if command -v lychee >/dev/null 2>&1; then
  lychee --no-progress --max-retries 2 \
    --exclude 'localhost|127\.0\.0\.1|example\.com' \
    README.md CHANGELOG.md docs/ 2>&1 | tail -5
  rc=${PIPESTATUS[0]}
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: lychee reported broken links (exit $rc)"
    fail=1
  fi
else
  echo "SKIP: lychee not in PATH (run via: nix run .#docs-gate)"
fi

echo "== TODO_LIST hygiene: no checked boxes =="
if grep -nE '^\s*- \[[xX]\]' TODO_LIST.md; then
  echo "FAIL: TODO_LIST.md contains checked items — completed rows move to CHANGELOG.md"
  fail=1
else
  echo "OK: no checked boxes"
fi

echo "== stale-report probe (advisory) =="
cutoff_days=21
now=$(date +%s)
stale=0
while IFS= read -r -d '' f; do
  age=$(( (now - $(stat -c %Y "$f")) / 86400 ))
  if [ "$age" -ge "$cutoff_days" ]; then
    echo "ADVISORY: $f is ${age}d old and not archived — harvest open items into TODO_LIST, then move to docs/status/archived/"
    stale=1
  fi
done < <(find docs/status -maxdepth 1 -name '*.md' -print0 2>/dev/null)
[ "$stale" -eq 0 ] && echo "OK: no reports older than ${cutoff_days}d in status root"

exit "$fail"
