"""Check relative markdown links in tracked docs resolve to real files.

Complements TestDocsHaveNoDanglingLinks in docs_links_test.go: the Go test runs
under `go test`, this script is for a quick check without the toolchain.

Usage:
  python scripts/check_doc_links.py     # exit 0 = no dead links
"""
import os
import pathlib
import re
import sys

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except (AttributeError, OSError):
    pass

# Default to this script's parent directory so the check runs from anywhere.
ROOT = pathlib.Path(__file__).resolve().parent.parent
SKIP_DIRS = {".git", "node_modules", "dist", ".tmp-tools", "changelogs",
             "vendor", ".vite", "npm"}

LINK = re.compile(r"\]\(([^)\s]+?)(#[^)]*)?\)")

# Links that illustrate link syntax rather than navigate anywhere.
ILLUSTRATIONS = {"/abs/path/file.ts"}

dead = []
checked = 0
for dirpath, dirnames, filenames in os.walk(ROOT):
    dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
    for name in filenames:
        if not name.endswith(".md"):
            continue
        p = pathlib.Path(dirpath) / name
        if p.name == "CHANGELOG.md":
            continue
        text = p.read_text(encoding="utf-8", errors="replace")
        for m in LINK.finditer(text):
            target = m.group(1)
            if target.startswith(("http://", "https://", "mailto:", "#", "data:")):
                continue
            if target in ILLUSTRATIONS:
                continue
            checked += 1
            resolved = (p.parent / target).resolve()
            if not resolved.exists():
                dead.append(f"{p.relative_to(ROOT)} -> {target}")

print(f"checked {checked} relative links")
print(f"dead: {len(dead)}")
for d in sorted(set(dead)):
    print("  ", d)
sys.exit(1 if dead else 0)
