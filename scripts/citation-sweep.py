#!/usr/bin/env python3
"""citation-sweep — ship gate for docs/review/libs/*.md utilization reports.

Extracts every file:line citation from the reports and verifies file existence
plus line range against the consumer repo AND the audited library checkouts.
Citation forms accepted (en-dash ranges normalized before matching):
  - absolute path            /home/lars/projects/go-finding/finding.go:42
  - consumer-relative        pkg/provider/provider.go:30
  - lib-qualified            go-finding/finding.go:42
                              go-finding/toolsdk/register.go:17
  - bare basename            finding.go:42  (unique-match fallback, else defect)

Exit 0 = all citations resolve (ship gate); exit 1 = defects listed on stderr.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

CONSUMER = Path("/home/lars/projects/go-version-auto-configure")
LIBS = {
    name: Path("/home/lars/projects") / name
    for name in (
        "go-finding",
        "linter-autoconfigure-sdk",
        "go-policy-dsl",
        "go-dag-app",
        "go-linter-sdk",
        "go-compliance",
    )
}

CITE_RE = re.compile(
    r"`?(?P<path>(?:/[\w.\-]+(?:/[\w.\-]+)+)|[\w.\-]+(?:/[\w.\-]+)*\.(?:go|mod|nix|yml|yaml|json|sh|md))"
    r":(?P<start>\d+)(?:[–—-](?P<end>\d+))?`?"
)


def resolve(path: str) -> Path | None:
    """Resolve a cited path against consumer then lib repos. None = not found."""
    candidate = Path(path)
    if candidate.is_absolute():
        return candidate if candidate.is_file() else None
    if (CONSUMER / path).is_file():
        return CONSUMER / path
    first, _, rest = path.partition("/")
    if first in LIBS and rest:
        resolved = LIBS[first] / rest
        if resolved.is_file():
            return resolved
    return None


def resolve_basename(path: str) -> Path | None:
    """Basename fallback: unique match across consumer + libs, else None."""
    name = Path(path).name
    if not name:
        return None
    hits: list[Path] = []
    roots = [CONSUMER, *LIBS.values()]
    for root in roots:
        for hit in root.rglob(name):
            s = str(hit)
            if "/.git/" in s or "/vendor/" in s or "/node_modules/" in s:
                continue
            hits.append(hit)
            if len(hits) > 1:
                return None
    return hits[0] if hits else None


def main(argv: list[str]) -> int:
    reports = [Path(a) for a in argv[1:]] or sorted((CONSUMER / "docs/review/libs").glob("*.md"))
    if not reports:
        print("no reports found", file=sys.stderr)
        return 1
    defects = 0
    checked = 0
    for report in reports:
        text = report.read_text(encoding="utf-8").replace("–", "-").replace("—", "-")
        for match in CITE_RE.finditer(text):
            path, start, end = match["path"], int(match["start"]), match["end"]
            line = text.count("\n", 0, match.start()) + 1
            target = resolve(path)
            if target is None:
                target = resolve_basename(path)
            checked += 1
            if target is None:
                defects += 1
                print(f"DEFECT {report.name}:{line}: '{path}:{start}' — file not found in consumer or lib checkouts")
                continue
            line_count = sum(1 for _ in target.open(encoding="utf-8", errors="replace"))
            limit = int(end) if end else start
            if start > line_count or limit > line_count or start < 1:
                defects += 1
                print(f"DEFECT {report.name}:{line}: '{path}:{start}{'-' + end if end else ''}' — {target} has {line_count} lines")
    print(f"{checked} citations checked across {len(reports)} reports — {defects} defect(s)")
    return 1 if defects else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
