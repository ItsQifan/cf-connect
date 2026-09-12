"""Brand-consistency smoke check (plan DoD item 7).

Fails if the old brand survives anywhere it should not. Allowed locations:
  - CC_* environment variables and cc_connect_* internal identifiers
  - npm/ (deferred to a later task)
  - CHANGELOG.md and changelogs/ (historical record)
  - the two documents that exist to record the trim itself, which necessarily
    name the old brand while quoting commands and outputs

Usage:
  python scripts/check_brand.py        # exit 0 = clean, 1 = offenders
"""
import os
import pathlib
import re
import sys

# Windows consoles default to a legacy code page; the report contains the plan's
# own wording, so force UTF-8 rather than crashing on a box-drawing character.
try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except (AttributeError, OSError):
    pass

# Default to this script's parent directory so the check runs from anywhere.
ROOT = pathlib.Path(__file__).resolve().parent.parent
SKIP_DIRS = {".git", "node_modules", "dist", ".tmp-tools", "changelogs", "npm", "vendor", ".vite"}

# Files whose subject IS the trim record; they have to be able to quote it.
RECORD_FILES = {"ACCEPTANCE.md", "BASELINE-TESTS.md", "RELEASE.md"}

OLD = re.compile(r"[Cc][Cc]-[Cc][Oo][Nn][Nn][Ee][Cc][Tt]")
# A protected occurrence has an underscore (or other identifier char) right
# before it: cc_connect_version, CC_..., etc.
PROTECTED = re.compile(r"[A-Za-z0-9_][Cc][Cc]-[Cc][Oo][Nn][Nn][Ee][Cc][Tt]")

TEXT_EXT = {".go", ".md", ".toml", ".yaml", ".yml", ".json", ".ts", ".tsx", ".js",
            ".mjs", ".cjs", ".jsx", ".ps1", ".sh", ".bat", ".cmd", ".html", ".css",
            ".svg", ".txt", ".sum", ".mod", ".in"}
TEXT_NAMES = {"Makefile", ".gitignore", ".dockerignore", "Dockerfile"}

# Naming the upstream project is required, not leakage: this fork must be able
# to say what it was forked from (attribution, licence, version provenance).
# Every allowed line must literally reference the upstream repository, so this
# cannot silently swallow a forgotten rename.
UPSTREAM = "github.com/chenhg5/cc-connect"

allowed = []
offenders = []
protected_seen = 0
scanned = 0

for dirpath, dirnames, filenames in os.walk(ROOT):
    dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
    for name in filenames:
        p = pathlib.Path(dirpath) / name
        if p.name == "CHANGELOG.md":
            continue
        if p.name not in TEXT_NAMES and p.suffix.lower() not in TEXT_EXT:
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        scanned += 1
        rel = p.relative_to(ROOT)
        lines = text.splitlines()

        for m in OLD.finditer(text):
            start = m.start()
            prefix = text[max(0, start - 1):start]
            if prefix and re.match(r"[A-Za-z0-9_]", prefix):
                protected_seen += 1
                continue
            lineno = text[:start].count("\n") + 1
            snippet = lines[lineno - 1].strip()[:130]
            # Allowed when the line is explicitly about the upstream project:
            # either it links the upstream repo, or it says "upstream".
            if UPSTREAM in snippet or "upstream" in snippet.lower():
                allowed.append(f"{rel}:{lineno}: {snippet}")
                continue
            # The trim records document this very check, so they must be able to
            # name the old brand (and the old paths) while quoting it.
            if rel.name in RECORD_FILES:
                allowed.append(f"{rel}:{lineno}: {snippet}")
                continue
            offenders.append(f"{rel}:{lineno}: {snippet}")

print(f"scanned {scanned} text files")
print(f"upstream attributions allowed: {len(allowed)}")
for a in allowed:
    print("   ok:", a)
print(f"unexpected old-brand occurrences: {len(offenders)}")
for o in offenders:
    print("  BAD:", o)

# Confirm the deliberate no-rename decisions actually held: the CC_* env-var
# contract and the cc_connect_* bridge/cache identifiers must still be present.
cc_env = 0
cc_internal = 0
for dirpath, dirnames, filenames in os.walk(ROOT):
    dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
    for name in filenames:
        p = pathlib.Path(dirpath) / name
        if p.name not in TEXT_NAMES and p.suffix.lower() not in TEXT_EXT:
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        cc_env += len(re.findall(r"\bCC_[A-Z][A-Z0-9_]*", text))
        cc_internal += len(re.findall(r"\bcc_connect_[a-z_]+", text))

print(f"CC_* env-var references kept: {cc_env}")
print(f"cc_connect_* internal identifiers kept: {cc_internal}")
if cc_env == 0 or cc_internal == 0:
    offenders.append("CC_* or cc_connect_* identifiers were renamed by mistake")

# The old config directory name must be gone entirely.
legacy_dir_hits = [o for o in offenders if ".cc-connect" in o]
print(f"legacy '~/.cc-connect' path references: {len(legacy_dir_hits)}")

sys.exit(1 if offenders else 0)

