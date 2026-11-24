#!/usr/bin/env python3
"""citation-sweep — ship gate for docs/review/libs/*.md utilization reports.

Extracts every file:line citation from the reports and verifies file existence
plus line range against the consumer repo AND the audited library checkouts.
Citation forms accepted (en-dash ranges normalized before matching):
  - absolute path            /home/lars/projects/go-finding/finding.go:42
  - tilde path               ~/go/pkg/mod/github.com/...@v1.16.0/finding.go:42
  - consumer-relative        pkg/provider/provider.go:30
  - lib-qualified            go-finding/finding.go:42
                              go-finding/toolsdk/register.go:17
  - module-cache (PINNED)    go-finding@v1.16.0/finding.go:42
                              github.com/larsartmann/go-finding@v1.16.0/finding.go:42
  - bare basename            finding.go:42  (unique-match fallback, tracked)

Module-cache cites resolve against GOMODCACHE (default ~/go/pkg/mod), so a
bridge citation can be verified against the PINNED version instead of the
checkout HEAD (M18).

Exit 0 = all citations resolve (ship gate); exit 1 = defects listed on stderr.
Flags: --summary (per-report counts + basename-fallback list), --json (CI),
--self-test (fixture tests for the resolvers).
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys
import tempfile
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
LIB_DOMAIN = "github.com/larsartmann"

CITE_RE = re.compile(
    r"`?(?P<path>(?:~?/[\w.\-]+(?:/[\w.\-]+(?:@[\w.\-]+)*)+)"
    r"|(?:[\w.\-]+(?:@[\w.\-]+)?(?:/[\w.\-]+(?:@[\w.\-]+)?)*)\.(?:go|mod|nix|yml|yaml|json|sh|md))"
    r":(?P<start>\d+)(?:[–—-](?P<end>\d+))?`?"
)


def modcache_root() -> Path:
    return Path(os.environ.get("GOMODCACHE", str(Path.home() / "go/pkg/mod")))


def resolve_modcache(path: str, modcache: Path) -> Path | None:
    """Resolve a @version cite against the module cache. None = not found."""
    stripped = path.removeprefix("~/")
    if not stripped.startswith("go/pkg/mod/"):
        stripped = f"go/pkg/mod/{stripped}"
    if not stripped.startswith(f"go/pkg/mod/{LIB_DOMAIN}/"):
        stripped = stripped.replace("go/pkg/mod/", f"go/pkg/mod/{LIB_DOMAIN}/", 1)
    candidate = modcache / stripped.removeprefix("go/pkg/mod/")
    return candidate if candidate.is_file() else None


def resolve(path: str, consumer: Path, libs: dict[str, Path], modcache: Path) -> Path | None:
    """Resolve a cited path. Order: modcache (@v), consumer, lib repos, absolute."""
    if "@" in path:
        hit = resolve_modcache(path, modcache)
        if hit is not None:
            return hit
    candidate = Path(path)
    if candidate.is_absolute():
        return candidate if candidate.is_file() else None
    if (consumer / path).is_file():
        return consumer / path
    first, _, rest = path.partition("/")
    if first in libs and rest:
        resolved = libs[first] / rest
        if resolved.is_file():
            return resolved
    return None


def resolve_basename(
    path: str, consumer: Path, libs: dict[str, Path]
) -> Path | None:
    """Basename fallback: unique match across consumer + libs, else None."""
    name = Path(path).name
    if not name:
        return None
    hits: list[Path] = []
    for root in (consumer, *libs.values()):
        for hit in root.rglob(name):
            s = str(hit)
            if "/.git/" in s or "/vendor/" in s or "/node_modules/" in s:
                continue
            hits.append(hit)
            if len(hits) > 1:
                return None
    return hits[0] if hits else None


def sweep_report(
    report: Path, consumer: Path, libs: dict[str, Path], modcache: Path
) -> dict:
    """Sweep one report. Returns {name, checked, defects, defect_lines, fallbacks}."""
    result = {
        "name": report.name,
        "checked": 0,
        "defects": 0,
        "defect_lines": [],
        "fallbacks": [],
    }
    text = report.read_text(encoding="utf-8").replace("–", "-").replace("—", "-")
    for match in CITE_RE.finditer(text):
        path, start, end = match["path"], int(match["start"]), match["end"]
        line = text.count("\n", 0, match.start()) + 1
        target = resolve(path, consumer, libs, modcache)
        via_basename = False
        if target is None and "@" not in path:
            target = resolve_basename(path, consumer, libs)
            via_basename = target is not None
        result["checked"] += 1
        if target is None:
            result["defects"] += 1
            result["defect_lines"].append(
                f"{report.name}:{line}: '{path}:{start}' — file not found"
            )
            print(f"DEFECT {report.name}:{line}: '{path}:{start}' — file not found in consumer, lib checkouts, or module cache", file=sys.stderr)
            continue
        if via_basename:
            result["fallbacks"].append(f"{report.name}:{line}: '{path}:{start}' -> {target}")
        line_count = sum(1 for _ in target.open(encoding="utf-8", errors="replace"))
        limit = int(end) if end else start
        if start > line_count or limit > line_count or start < 1:
            result["defects"] += 1
            result["defect_lines"].append(
                f"{report.name}:{line}: '{path}:{start}-{end if end else start}' — {target} has {line_count} lines"
            )
            print(
                f"DEFECT {report.name}:{line}: '{path}:{start}{'-' + end if end else ''}' — {target} has {line_count} lines",
                file=sys.stderr,
            )
    return result


def print_summary(results: list[dict]) -> None:
    total_checked = sum(r["checked"] for r in results)
    total_defects = sum(r["defects"] for r in results)
    print(f"{'report':40} {'checked':>8} {'defects':>8} {'basename':>9}")
    for r in results:
        print(f"{r['name']:40} {r['checked']:>8} {r['defects']:>8} {len(r['fallbacks']):>9}")
    print(f"{'TOTAL':40} {total_checked:>8} {total_defects:>8}")
    fallbacks = [f for r in results for f in r["fallbacks"]]
    if fallbacks:
        print("\nbasename-fallback cites (uniqueness-fragile — consider lib-qualifying):")
        for f in fallbacks:
            print(f"  {f}")


def self_test() -> int:
    """Fixture tests for the resolvers. Exit 0 = all pass."""
    tmp = Path(tempfile.mkdtemp(prefix="citation-sweep-test-"))
    consumer = tmp / "consumer"
    libs = {"go-finding": tmp / "go-finding", "go-dag-app": tmp / "go-dag-app"}
    modcache = tmp / "modcache"
    for d in (consumer / "pkg/provider", *libs.values(), modcache / LIB_DOMAIN / "go-finding@v1.16.0"):
        d.mkdir(parents=True)
    (consumer / "pkg/provider/provider.go").write_text("l1\nl2\nl3\n")
    (libs["go-finding"] / "finding.go").write_text("\n".join(f"l{i}" for i in range(1, 51)) + "\n")
    (libs["go-dag-app"] / "container.go").write_text("other\n")
    (libs["go-dag-app"] / "provider.go").write_text("collides with consumer basename\n")
    (modcache / LIB_DOMAIN / "go-finding@v1.16.0" / "finding.go").write_text(
        "\n".join(f"c{i}" for i in range(1, 61)) + "\n"
    )

    cases = [
        # (path, start, end, expected: resolved-path-substring or None, via_basename)
        ("pkg/provider/provider.go", 3, None, "consumer/pkg/provider/provider.go", False),
        ("go-finding/finding.go", 10, "12", "go-finding/finding.go", False),
        ("go-finding@v1.16.0/finding.go", 20, None, "go-finding@v1.16.0/finding.go", False),
        (f"{LIB_DOMAIN}/go-finding@v1.16.0/finding.go", 20, None, "go-finding@v1.16.0/finding.go", False),
        (f"~/go/pkg/mod/{LIB_DOMAIN}/go-finding@v1.16.0/finding.go", 20, None, "go-finding@v1.16.0/finding.go", False),
        ("go-finding@v9.9.9/finding.go", 20, None, None, False),
        ("finding.go", 10, None, "go-finding/finding.go", True),
        ("provider.go", 1, None, None, False),
        ("missing.go", 1, None, None, False),
        ("go-finding/finding.go", 51, None, "RANGE", False),
    ]
    failures = 0
    for path, start, end, expect, expect_basename in cases:
        target = resolve(path, consumer, libs, modcache)
        via_basename = False
        if target is None and "@" not in path:
            target = resolve_basename(path, consumer, libs)
            via_basename = target is not None
        ok = True
        if expect == "RANGE":
            line_count = sum(1 for _ in target.open())
            ok = start > line_count
        elif expect is None:
            ok = target is None
        else:
            ok = target is not None and expect in str(target)
        if ok:
            ok = via_basename == expect_basename
        if ok:
            print(f"PASS {path}:{start}{'-' + end if end else ''}")
        else:
            failures += 1
            print(f"FAIL {path}:{start}{'-' + end if end else ''} -> {target} (basename={via_basename})")
    shutil.rmtree(tmp)
    print(f"self-test: {len(cases) - failures}/{len(cases)} passed")
    return 1 if failures else 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="Verify docs/review/libs citations")
    parser.add_argument("reports", nargs="*", help="report files (default: docs/review/libs/*.md)")
    parser.add_argument("--summary", action="store_true", help="per-report counts + basename-fallback list")
    parser.add_argument("--json", action="store_true", help="machine-readable output (implies summary fields)")
    parser.add_argument("--self-test", action="store_true", help="run fixture tests and exit")
    args = parser.parse_args(argv[1:])

    if args.self_test:
        return self_test()

    reports = [Path(a) for a in args.reports] or sorted((CONSUMER / "docs/review/libs").glob("*.md"))
    if not reports:
        print("no reports found", file=sys.stderr)
        return 1
    results = [sweep_report(r, CONSUMER, LIBS, modcache_root()) for r in reports]
    total_checked = sum(r["checked"] for r in results)
    total_defects = sum(r["defects"] for r in results)

    if args.json:
        print(json.dumps({
            "reports": [
                {
                    "name": r["name"],
                    "checked": r["checked"],
                    "defects": r["defects"],
                    "defect_lines": r["defect_lines"],
                    "basename_fallbacks": r["fallbacks"],
                }
                for r in results
            ],
            "total_checked": total_checked,
            "total_defects": total_defects,
        }))
    elif args.summary:
        print_summary(results)
    else:
        print(f"{total_checked} citations checked across {len(reports)} reports — {total_defects} defect(s)")
    return 1 if total_defects else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
