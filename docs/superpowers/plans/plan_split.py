#!/usr/bin/env python3
"""Split a monolithic P0 sub-plan into MANIFEST.md + tasks/NN-<slug>.md + APPENDIX.md, and join it back.

    python3 -I plan_split.py split <plan.md> <out-dir>   # writes the directory, verifies a byte-exact round trip
    python3 -I plan_split.py join  <plan-dir>           # prints the reassembled monolithic plan to stdout

Layout of a split plan directory:
    MANIFEST.md   header, Global Constraints, Review Focus, File Structure, Consumes/Produces (verbatim)
                  + a generated task table between TABLE_BEGIN/TABLE_END markers
    tasks/NN-*.md a generated pointer header, the BODY marker, then the task verbatim
    APPENDIX.md   everything after the last task (coverage, self-review), verbatim; may be absent

Split points: `### Task N` headings and the first `## ` heading after the last task, ignoring fenced code blocks.
"""
import os
import re
import sys
import unicodedata

TABLE_BEGIN = "<!-- generated task table: begin (plan_split.py) -->"
TABLE_END = "<!-- generated task table: end -->"
BODY = "<!-- task body (verbatim from the plan) -->"
FENCE = re.compile(r"^\s*(```|~~~)")
TASK = re.compile(r"^### Task (\d+)[:.]?\s*(.*)$")


def split_points(lines):
    fence, starts = False, []
    for i, l in enumerate(lines):
        if FENCE.match(l):
            fence = not fence
        elif not fence and TASK.match(l):
            starts.append(i)
    if fence or not starts:
        raise SystemExit("unbalanced fences or no tasks")
    tail, fence = len(lines), False
    for i in range(starts[-1] + 1, len(lines)):
        if FENCE.match(lines[i]):
            fence = not fence
        elif not fence and lines[i].startswith("## "):
            tail = i
            break
    return starts, tail


def slug(title):
    s = unicodedata.normalize("NFKD", title.lower()).encode("ascii", "ignore").decode()
    s = re.sub(r"`|\(.*?\)", "", s)
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
    return "-".join(s.split("-")[:6]) or "task"


def task_refs(body, own):
    """Earlier tasks named in the task's Interfaces → Consumes lines (same plan)."""
    refs = set()
    for l in body:
        if "Consumes" in l or l.lstrip().startswith("- Consumes"):
            refs |= {int(n) for n in re.findall(r"Task (\d+)", l)}
    return sorted(r for r in refs if r < own)


def split(src, out):
    text = open(src, encoding="utf-8").read()
    lines = text.split("\n")
    starts, tail = split_points(lines)
    title = lines[0].lstrip("# ").strip()
    os.makedirs(os.path.join(out, "tasks"), exist_ok=True)
    rows = []
    bounds = starts + [tail]
    for a, b in zip(bounds, bounds[1:]):
        m = TASK.match(lines[a])
        n, name = int(m.group(1)), m.group(2)
        fname = f"{n:02d}-{slug(name)}.md"
        body = lines[a:b]
        tokens = len("\n".join(body)) // 4
        head = [
            f"> **{title} — Task {n}.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,",
            "> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.",
            "",
            BODY,
        ]
        with open(os.path.join(out, "tasks", fname), "w", encoding="utf-8") as f:
            f.write("\n".join(head + body))
        rows.append((n, name, fname, tokens, task_refs(body, n)))
    table = [
        "",
        TABLE_BEGIN,
        "## Task files",
        "",
        "Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).",
        "\"Consumes tasks\" lists earlier tasks named in the task's Consumes lines. Dispatch in number order",
        "unless Global Constraints or the P0 index state a different execution order.",
        "",
        "| # | Task | File | ~tokens | Consumes tasks |",
        "|---|---|---|---|---|",
    ]
    for n, name, fname, tokens, refs in rows:
        cell = name.replace("|", "\\|")
        table.append(f"| {n} | {cell} | [`tasks/{fname}`](tasks/{fname}) | {tokens // 100 / 10:.1f}k | "
                     f"{', '.join(map(str, refs)) or '—'} |")
    if tail < len(lines):
        table += ["", "Coverage and self-review: [`APPENDIX.md`](APPENDIX.md)."]
    table.append(TABLE_END)
    with open(os.path.join(out, "MANIFEST.md"), "w", encoding="utf-8") as f:
        f.write("\n".join(lines[: starts[0]] + table))
    if tail < len(lines):
        with open(os.path.join(out, "APPENDIX.md"), "w", encoding="utf-8") as f:
            f.write("\n".join(lines[tail:]))
    if join(out) != text:
        raise SystemExit(f"round trip FAILED for {src}")
    print(f"{src} -> {out}: {len(rows)} tasks, round trip byte-exact")


def join(d):
    man = open(os.path.join(d, "MANIFEST.md"), encoding="utf-8").read().split("\n")
    b = man.index(TABLE_BEGIN)
    parts = man[: b - 1]  # drop the blank line written before TABLE_BEGIN
    tdir = os.path.join(d, "tasks")
    for fname in sorted(os.listdir(tdir), key=lambda s: int(s.split("-")[0])):
        t = open(os.path.join(tdir, fname), encoding="utf-8").read().split("\n")
        parts += t[t.index(BODY) + 1:]
    app = os.path.join(d, "APPENDIX.md")
    if os.path.exists(app):
        parts += open(app, encoding="utf-8").read().split("\n")
    return "\n".join(parts)


if __name__ == "__main__":
    if len(sys.argv) == 4 and sys.argv[1] == "split":
        split(sys.argv[2], sys.argv[3])
    elif len(sys.argv) == 3 and sys.argv[1] == "join":
        sys.stdout.write(join(sys.argv[2]))
    else:
        raise SystemExit(__doc__)
