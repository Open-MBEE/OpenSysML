#!/usr/bin/env python3
"""Attribute a re-recorded baseline's movement to the sides of a merge.

After `ci:rebaseline` merges develop into a pull request and re-runs the recorders,
every generated record that the fresh run moved must be explained by the merge:
the pull request moved it (the record differs between the merge base and the PR
head), develop moved it (it differs between the merge base and develop), or both
did. A record neither side moved that the fresh run moves anyway is a movement
the merge does not explain — an interaction, an environment difference or a
non-deterministic recorder — and the workflow stops instead of committing it.

    rebaseline-movement.py --base <sha> --ours <sha> --theirs <sha> --merged <sha> \\
        [--tree <dir>] [--summary <path>] <generated path>...

Each generated path is a committed baseline (or a directory of goldens) read at the
four revisions and in the working tree (the fresh run). The summary is Markdown:
per baseline, how many records moved and by whose side, plus the figure deltas
the commit message and the pull-request comment state. Exit status 1 when a
movement is unexplained, 2 on a usage error.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field

TRAINING = "tests/corpus/testdata/training_examples_expected.txt"


def git(*args: str, cwd: str | None = None, check: bool = True) -> str:
    completed = subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)
    if check and completed.returncode != 0:
        raise SystemExit(f"git {' '.join(args)}: {completed.stderr.strip()}")
    return completed.stdout


# ---------------------------------------------------------------- record parsers


def parse_tsv(text: str) -> dict[str, str]:
    """`<verdict>\\t<path>` lines keyed by path; `#` lines are the header."""
    records = {}
    for line in text.splitlines():
        if not line or line.startswith("#"):
            continue
        value, _, path = line.rpartition("\t")
        records[path] = value
    return records


def parse_structure(text: str) -> dict[str, str]:
    """Per-root blocks: `root <name>: …` then `  <property>: …` lines, keyed by root and property."""
    records, root = {}, ""
    for line in text.splitlines():
        if line.startswith("root "):
            root = line[len("root "):].split(":", 1)[0].strip()
            records[root] = line[len("root ") + len(root) + 1:].strip()
        elif line.startswith("  ") and root:
            name, _, counts = line.strip().partition(": ")
            records[f"{root}\t{name}"] = counts
    return records


def parse_sweep(text: str) -> dict[str, str]:
    """Tab-separated probe lines keyed by every field but the outcome."""
    records = {}
    for line in text.splitlines():
        if not line or line.startswith("#"):
            continue
        key, _, outcome = line.rpartition("\t")
        records[key] = outcome
    return records


def parse_census(text: str) -> dict[str, str]:
    """derived_census_baseline.json: a record per root and rule; the totals are derived."""
    report = json.loads(text)
    records = {}
    for root in report.get("roots", []):
        for rule, counts in root.get("rules", {}).items():
            records[f"{root['name']}\t{rule}"] = json.dumps(counts, sort_keys=True)
        records[f"root\t{root['name']}"] = json.dumps({k: v for k, v in root.items() if k not in ("name", "rules")}, sort_keys=True)
    return records


def parse_census_records(text: str) -> dict[str, str]:
    """derived_census_records.tsv: `<rule>\t<root>\t<counts...>` lines keyed like parse_census."""
    records = {}
    for line in text.splitlines():
        fields = line.split("\t")
        if not line or line.startswith("#") or len(fields) < 2:
            continue
        records[f"{fields[1]}\t{fields[0]}"] = "\t".join(fields[2:])
    return records


def parse_apijson(text: str) -> dict[str, str]:
    """api-json-differential-baseline.json: a record per scorecard row and per root."""
    report = json.loads(text)
    records = {f"row\t{key}": json.dumps(counts) for key, counts in report.get("scorecard", {}).items()}
    for root in report.get("roots", []):
        records[f"root\t{root['name']}"] = json.dumps({k: v for k, v in root.items() if k != "name"}, sort_keys=True)
    return records


PARSERS = (
    (re.compile(r"^tests/corpus/testdata/[a-z_]+_expected\.txt$"), parse_tsv),
    (re.compile(r"structure_differential_expected\.txt$"), parse_structure),
    (re.compile(r"library-and-prefix-sweep-outcomes\.txt$"), parse_sweep),
    (re.compile(r"derived_census_baseline\.json$"), parse_census),
    (re.compile(r"derived_census_records\.tsv$"), parse_census_records),
    (re.compile(r"api-json-differential-baseline\.json$"), parse_apijson),
)


def parser_for(path: str):
    for pattern, parser in PARSERS:
        if pattern.search(path):
            return parser
    return None


# ---------------------------------------------------------------- figure deltas


def apijson_figures(text: str | None) -> dict[str, int]:
    if not text:
        return {}
    totals = json.loads(text).get("totals", {})
    props, elements = totals.get("properties", {}), totals.get("elements", {})
    return {
        "files": totals.get("files", 0),
        "matched": elements.get("matched", 0),
        "ours-only": elements.get("oursOnly", 0),
        "pilot-only": elements.get("pilotOnly", 0),
        "present-equal": props.get("presentEqual", 0),
        "present-different": props.get("presentDifferent", 0),
        "ours-missing": props.get("oursMissing", 0),
        "pilot-missing": props.get("pilotMissing", 0),
        "indeterminate": props.get("indeterminate", 0),
    }


def census_figures(text: str | None) -> dict[str, int]:
    if not text:
        return {}
    summary = json.loads(text).get("summary")
    if summary is None:
        return {}
    return {name: summary.get(name, 0) for name in ("served", "answering", "empty", "notDerived", "uncarried", "silent", "pilotAnswering")}


def census_records_figures(text: str | None) -> dict[str, int]:
    """The column totals of derived_census_records.tsv, named by its `# rule\troot\t...` header."""
    columns: list[str] = []
    figures: dict[str, int] = {}
    for line in (text or "").splitlines():
        fields = line.split("\t")
        if line.startswith("# rule\troot\t"):
            columns = [name.strip() for name in fields[2:]]
            continue
        if not line or line.startswith("#") or len(fields) != len(columns) + 2:
            continue
        for name, value in zip(columns, fields[2:]):
            if value.isdigit():
                figures[name] = figures.get(name, 0) + int(value)
    return figures


def structure_figures(text: str | None) -> dict[str, int]:
    figures = {}
    for root, counts in parse_structure(text or "").items():
        if "\t" in root:
            continue
        for part in counts.split(","):
            name, _, number = part.strip().rpartition(" ")
            if number.isdigit():
                figures[f"{root} {name}"] = int(number)
    return figures


def verdict_figures(text: str | None) -> dict[str, int]:
    figures: dict[str, int] = {}
    for verdict in parse_tsv(text or "").values():
        figures[verdict] = figures.get(verdict, 0) + 1
    return figures


FIGURES = (
    (re.compile(r"api-json-differential-baseline\.json$"), apijson_figures),
    (re.compile(r"derived_census_baseline\.json$"), census_figures),
    (re.compile(r"derived_census_records\.tsv$"), census_records_figures),
    (re.compile(r"structure_differential_expected\.txt$"), structure_figures),
    (re.compile(r"^tests/corpus/testdata/[a-z_]+_expected\.txt$"), verdict_figures),
)


def figures_for(path: str):
    for pattern, figures in FIGURES:
        if pattern.search(path):
            return figures
    return None


# ---------------------------------------------------------------- reading revisions


def blob_at(rev: str, path: str, cwd: str) -> str | None:
    completed = subprocess.run(["git", "show", f"{rev}:{path}"], cwd=cwd, capture_output=True, text=True)
    return completed.stdout if completed.returncode == 0 else None


def tree_at(rev: str, path: str, cwd: str) -> dict[str, str]:
    """Every blob under path at rev, keyed by repository path, valued by blob id."""
    out = git("ls-tree", "-r", rev, "--", path, cwd=cwd, check=False)
    records = {}
    for line in out.splitlines():
        meta, _, name = line.partition("\t")
        records[name] = meta.split()[2]
    return records


def tree_in_worktree(path: str, cwd: str) -> dict[str, str]:
    records = {}
    root = os.path.join(cwd, path)
    if os.path.isfile(root):
        return {path: blob_id(root)}
    for directory, _, names in os.walk(root):
        for name in names:
            full = os.path.join(directory, name)
            records[os.path.relpath(full, cwd).replace(os.sep, "/")] = blob_id(full)
    return records


def blob_id(path: str) -> str:
    with open(path, "rb") as handle:
        data = handle.read()
    return hashlib.sha1(b"blob %d\0" % len(data) + data).hexdigest()  # noqa: S324 - git's own object id


def file_in_worktree(path: str, cwd: str) -> str | None:
    full = os.path.join(cwd, path)
    if not os.path.isfile(full):
        return None
    with open(full, encoding="utf-8") as handle:
        return handle.read()


def is_directory_at(revs: list[str], path: str, cwd: str) -> bool:
    for rev in revs:
        out = git("ls-tree", rev, "--", path, cwd=cwd, check=False)
        if out.startswith("040000 tree"):
            return True
    return os.path.isdir(os.path.join(cwd, path))


# ---------------------------------------------------------------- attribution


@dataclass
class Movement:
    path: str
    moved: int = 0
    by_ours: int = 0
    by_theirs: int = 0
    by_both: int = 0
    unexplained: list[str] = field(default_factory=list)
    deltas: dict[str, tuple[int, int]] = field(default_factory=dict)
    regenerated: bool = False


def attribute(path: str, base: dict, ours: dict, theirs: dict, merged: dict, fresh: dict) -> Movement:
    """Classify every record the fresh run moved against the merged tree."""
    movement = Movement(path)
    for key in sorted(set(base) | set(ours) | set(theirs) | set(merged) | set(fresh)):
        b, o, t, m, f = base.get(key), ours.get(key), theirs.get(key), merged.get(key), fresh.get(key)
        if f == m:
            continue
        movement.moved += 1
        moved_ours, moved_theirs = o != b, t != b
        if moved_ours and moved_theirs:
            movement.by_both += 1
        elif moved_ours:
            if f != o:
                movement.unexplained.append(key)
            movement.by_ours += 1
        elif moved_theirs:
            if f != t:
                movement.unexplained.append(key)
            movement.by_theirs += 1
        else:
            movement.unexplained.append(key)
    return movement


def analyse(args, cwd: str) -> list[Movement]:
    movements = []
    revs = [args.base, args.ours, args.theirs, args.merged]
    for path in args.paths:
        if path == TRAINING:
            raise SystemExit(f"{TRAINING} is asserted, never re-recorded; drop it from the generated paths")
        parser = parser_for(path)
        if parser is None or is_directory_at(revs, path, cwd):
            versions = [tree_at(rev, path, cwd) for rev in revs]
            fresh = tree_in_worktree(path, cwd)
            if not any(versions) and not fresh:
                continue
            if parser is None:
                merged, regenerated = versions[3], Movement(path, regenerated=True)
                regenerated.moved = sum(1 for key in set(merged) | set(fresh) if merged.get(key) != fresh.get(key))
                movements.append(regenerated)
            else:
                movements.append(attribute(path, *versions, fresh))
            continue
        texts = [blob_at(rev, path, cwd) for rev in revs]
        fresh_text = file_in_worktree(path, cwd)
        if not any(texts) and fresh_text is None:
            continue
        versions = [parser(text) if text else {} for text in texts]
        movement = attribute(path, *versions, parser(fresh_text) if fresh_text else {})
        figures = figures_for(path)
        if figures is not None:
            was, now = figures(texts[3]), figures(fresh_text)
            movement.deltas = {name: (was.get(name, 0), now.get(name, 0)) for name in sorted(set(was) | set(now)) if was.get(name, 0) != now.get(name, 0)}
        movements.append(movement)
    return movements


# ---------------------------------------------------------------- the summary


def summarize(movements: list[Movement]) -> str:
    lines = []
    moved = [m for m in movements if m.moved]
    if not moved:
        lines.append("No generated record moved against the merged tree.")
    for m in moved:
        if m.regenerated:
            lines.append(f"- `{m.path}`: {m.moved} file(s) regenerated")
            continue
        parts = [f"{m.moved} record(s) moved"]
        if m.by_ours:
            parts.append(f"{m.by_ours} moved by this branch")
        if m.by_theirs:
            parts.append(f"{m.by_theirs} moved by develop")
        if m.by_both:
            parts.append(f"{m.by_both} moved by both sides (the merge's verdict, adjudicate)")
        lines.append(f"- `{m.path}`: " + "; ".join(parts))
        for name, (was, now) in m.deltas.items():
            lines.append(f"  - {name}: {was} → {now} ({now - was:+d})")
        if m.unexplained:
            lines.append(f"  - **{len(m.unexplained)} movement(s) the merge does not explain** (neither side moved the record, or the fresh value is neither side's):")
            for key in m.unexplained[:20]:
                lines.append(f"    - `{key}`")
            if len(m.unexplained) > 20:
                lines.append(f"    - … {len(m.unexplained) - 20} more")
    return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--base", required=True, help="merge base of the pull request and develop")
    parser.add_argument("--ours", required=True, help="the pull request's head before the merge")
    parser.add_argument("--theirs", required=True, help="develop as merged")
    parser.add_argument("--merged", required=True, help="the merge commit the recorders ran on")
    parser.add_argument("--tree", default=".", help="the working tree holding the fresh run (default: .)")
    parser.add_argument("--summary", help="write the Markdown summary here as well as to stdout")
    parser.add_argument("paths", nargs="+", help="generated paths, as the workflow lists them")
    args = parser.parse_args(argv)
    cwd = os.path.abspath(args.tree)
    movements = analyse(args, cwd)
    summary = summarize(movements)
    sys.stdout.write(summary)
    if args.summary:
        with open(args.summary, "w", encoding="utf-8") as handle:
            handle.write(summary)
    return 1 if any(m.unexplained for m in movements) else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
