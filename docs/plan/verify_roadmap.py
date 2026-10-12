#!/usr/bin/env python3
"""Verify docs/plan/ROADMAP.md against docs/requirements/REQUIREMENTS.md §6.

Checks
  1. every §6 FR ID appears exactly once in the ROADMAP appendix (and nothing else does);
  2. the appendix phase counts equal the cross-check table in the ROADMAP;
  3. forward references: for every §6 FR, every `FR-XXX-NNN` it cites (outside its
     "Source features" line) must be scheduled in the same or an earlier phase, unless the
     pair is listed in the ROADMAP table "Intentional forward references".

Usage: python3 -I verify_roadmap.py [--preview phases.tsv]
Exit code 0 = all checks pass.
"""
import re, sys, collections, pathlib

HERE = pathlib.Path(__file__).resolve().parent
REQ = HERE.parent / "requirements" / "REQUIREMENTS.md"
ROADMAP = HERE / "ROADMAP.md"
FR_RE = re.compile(r"FR-(?:SHELL|HOME|CHART|ANLY|WATCH|ACCT|DATA|EQB|DISC|MYS|BT|DEP|SCAN|SCALP|ORD|TG)-\d{3}")

def section6(text):
    start = text.index("\n## 6. Functional requirements")
    end = text.index("\n## 7. Deliberate deviations")
    return text[start:end]

def parse_frs(sec6):
    """Return ordered dict id -> body text (excluding the Source features line)."""
    frs = collections.OrderedDict()
    cur = None
    for line in sec6.splitlines():
        m = re.match(r"^### (FR-[A-Z]+-\d{3}) ", line)
        if m:
            cur = m.group(1); frs[cur] = []
            continue
        if cur and not line.startswith("- **Source features:**"):
            frs[cur].append(line)
    return collections.OrderedDict((k, "\n".join(v)) for k, v in frs.items())

def parse_appendix(md):
    phase = {}
    dups = []
    in_app = False
    for line in md.splitlines():
        if line.startswith("## Appendix A"):
            in_app = True; continue
        if in_app and line.startswith("## ") :
            in_app = False
        if in_app:
            m = re.match(r"^\|\s*(FR-[A-Z]+-\d{3})\s*\|\s*(P\d)\s*\|", line)
            if m:
                fid, ph = m.groups()
                if fid in phase: dups.append(fid)
                phase[fid] = ph
    return phase, dups

def parse_crosscheck(md):
    """Return {(module, phase): count} and totals row from the cross-check table."""
    rows = {}
    totals = {}
    phases = None
    in_tbl = False
    for line in md.splitlines():
        if line.startswith("| Module | FR count |"):
            phases = [c.strip() for c in line.strip("|").split("|")][2:]
            in_tbl = True; continue
        if in_tbl:
            if not line.startswith("|"):
                in_tbl = False; continue
            cells = [c.strip() for c in line.strip("|").split("|")]
            if cells[0].startswith("---"): continue
            name = cells[0].strip("*")
            vals = [int(c.strip("*")) if c.strip("*").isdigit() else 0 for c in cells[2:]]
            if name == "Total":
                totals = dict(zip(phases, vals)); totals["_count"] = int(cells[1].strip("*"))
            else:
                for p, v in zip(phases, vals): rows[(name, p)] = v
    return rows, totals

def parse_allowlist(md):
    allowed = {}
    in_tbl = False
    for line in md.splitlines():
        if "Intentional forward references" in line and line.startswith("#"):
            in_tbl = True; continue
        if in_tbl and line.startswith("## ") :
            break
        if in_tbl:
            m = re.match(r"^\|\s*(FR-[A-Z]+-\d{3})\s*\(P\d\)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|$", line)
            if m:
                src = m.group(1)
                for tgt in FR_RE.findall(m.group(2)):
                    allowed[(src, tgt)] = m.group(3)
    return allowed

def main():
    preview = None
    if "--preview" in sys.argv:
        preview = pathlib.Path(sys.argv[sys.argv.index("--preview") + 1])
    req = REQ.read_text(encoding="utf-8")
    frs = parse_frs(section6(req))
    ids = list(frs)
    ok = True
    print(f"REQUIREMENTS.md §6 FR IDs found: {len(ids)}")

    if preview:
        phase = dict(l.split("\t") for l in preview.read_text().splitlines() if l.strip())
        dups = []; allowed = {}
        print(f"(preview mode: phases from {preview.name})")
    else:
        md = ROADMAP.read_text(encoding="utf-8")
        phase, dups = parse_appendix(md)
        allowed = parse_allowlist(md)
        print(f"ROADMAP.md appendix rows: {len(phase)} (+{len(dups)} duplicate ids)")

    # --- Check 1: exactly-once coverage
    missing = [i for i in ids if i not in phase]
    extra = [i for i in phase if i not in frs]
    if missing: ok = False; print("MISSING from appendix:", missing)
    if extra:   ok = False; print("EXTRA ids in appendix (not in §6):", extra)
    if dups:    ok = False; print("DUPLICATE ids in appendix:", dups)
    if not (missing or extra or dups):
        print("Check 1 PASS: every §6 FR appears exactly once in the appendix.")

    # --- Check 2: counts
    pc = collections.Counter(phase[i] for i in ids if i in phase)
    mc = collections.Counter((i.split("-")[1], phase[i]) for i in ids if i in phase)
    print("Per-phase counts:", dict(sorted(pc.items())), "sum =", sum(pc.values()))
    if not preview:
        rows, totals = parse_crosscheck(md)
        bad = [(k, v, mc.get(k, 0)) for k, v in rows.items() if v != mc.get(k, 0)]
        bad += [(("ALL", p), v, pc.get(p, 0)) for p, v in totals.items() if p != "_count" and v != pc.get(p, 0)]
        if totals.get("_count") != len(ids): bad.append((("ALL", "count"), totals.get("_count"), len(ids)))
        if bad: ok = False; print("Check 2 FAIL: cross-check table mismatches (key, table, actual):", bad)
        else: print("Check 2 PASS: cross-check table matches the appendix and sums to", len(ids))

    # --- Check 3: forward references
    fwd = []
    for src, body in frs.items():
        if src not in phase: continue
        for tgt in sorted(set(FR_RE.findall(body))):
            if tgt == src or tgt not in phase: continue
            if int(phase[tgt][1]) > int(phase[src][1]):
                fwd.append((src, phase[src], tgt, phase[tgt]))
    unexpected = [f for f in fwd if (f[0], f[2]) not in allowed]
    print(f"Forward references found: {len(fwd)}; declared intentional: {len(fwd) - len(unexpected)}; unexpected: {len(unexpected)}")
    if preview or unexpected:
        for s, sp, t, tp in (fwd if preview else unexpected):
            print(f"  {s} ({sp}) -> {t} ({tp})")
    if unexpected and not preview: ok = False
    elif not preview: print("Check 3 PASS: no undeclared forward reference.")
    stale = [k for k in allowed if k not in {(f[0], f[2]) for f in fwd}]
    if stale: print("Note: allowlist entries that are not forward references any more:", stale)

    print("RESULT:", "PASS" if ok else "FAIL")
    sys.exit(0 if ok else 1)

if __name__ == "__main__":
    main()
