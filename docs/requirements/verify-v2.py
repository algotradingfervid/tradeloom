#!/usr/bin/env python3
"""Verification checks for REQUIREMENTS.md v2. Usage: verify.py <REQUIREMENTS.md> <FEATURES.md>"""
import re, sys

req_path, feat_path = sys.argv[1], sys.argv[2]
req = open(req_path, encoding="utf-8").read()
feat = open(feat_path, encoding="utf-8").read()
ok = True
def report(name, passed, detail=""):
    global ok
    ok = ok and passed
    print(f"[{'PASS' if passed else 'FAIL'}] {name}" + (f" — {detail}" if detail else ""))

i6 = req.index("\n## 6. Functional requirements")
i7 = req.index("\n## 7. Deliberate deviations")
i8 = req.index("\n## 8. Traceability")
i9 = req.index("\n## 9. Open questions")
i10 = req.index("\n## 10. Out of scope")
sec6, sec7, sec8, sec9, sec10 = req[i6:i7], req[i7:i8], req[i8:i9], req[i9:i10], req[i10:]

ids6 = re.findall(r"^### (FR-[A-Z]+-\d{3}) ", sec6, re.M)
ids10 = re.findall(r"^#### (FR-[A-Z]+-\d{3}(?:-opt)?) ", sec10, re.M)
all_ids = ids6 + ids10
dups = sorted({x for x in all_ids if all_ids.count(x) > 1})
report("FR ids unique", not dups, f"{len(ids6)} in §6, {len(ids10)} in §10; duplicates: {dups}")

refs = set(re.findall(r"\bFR-[A-Z]+-\d{3}(?:-opt)?\b", req))
missing = sorted(r for r in refs if r not in set(all_ids))
report("every FR referenced exists", not missing, f"missing: {missing}")

only10 = set(ids10) - set(ids6)
bad = {}
for m in re.finditer(r"^### (FR-[A-Z]+-\d{3}) .*?(?=^### FR-|^## |\Z)", sec6, re.S | re.M):
    body = m.group(0)
    hits = sorted(r for r in set(re.findall(r"\bFR-[A-Z]+-\d{3}(?:-opt)?\b", body)) if r in only10)
    if hits:
        bad[m.group(1)] = hits
report("no §6 FR references an FR that lives only in §10", not bad, f"{bad}")

feat_ids = re.findall(r"^### ([A-Z]+-\d{3}) ", feat, re.M)
rows8 = set(re.findall(r"^\| ([A-Z]+-\d{3}) \|", sec8, re.M))
miss8 = [f for f in feat_ids if f not in rows8]
extra8 = sorted(rows8 - set(feat_ids))
report("every FEATURES.md feature ID appears in §8", not miss8 and not extra8, f"{len(feat_ids)} features; missing {miss8}; not in catalogue {extra8}")

fields = ["Source features", "Requirement", "Acceptance criteria", "Data/Kite dependency", "Deviation from Streak", "Priority hint"]
lacking = {}
for m in re.finditer(r"^### (FR-[A-Z]+-\d{3}) .*?(?=^### FR-|^## |^---|\Z)", sec6, re.S | re.M):
    body = m.group(0)
    miss = [f for f in fields if f"**{f}:**" not in body]
    if miss:
        lacking[m.group(1)] = miss
report("every §6 FR has all fields", not lacking, f"{lacking}")

# old names outside research references (allow the version-history line that names the old title? we forbid entirely)
leftovers = [(ln_no + 1, ln.strip()[:100]) for ln_no, ln in enumerate(req.split("\n"))
             if re.search(r"Streak Desktop|streak-desktop|\.sdstrategy|\.sdscanner|sd\.strategy|sd\.scanner", ln)]
report('no "Streak Desktop"/"streak-desktop"/".sdstrategy" remains', not leftovers, f"{leftovers[:5]}")

# §6 must not mention options features by name except in "out of scope / §10" pointers
opt_lines = []
for ln_no, ln in enumerate(sec6.split("\n")):
    if re.search(r"\b(option chain|CE/PE|payoff|PCR|straddle|strike)\b", ln, re.I) and "§10" not in ln and "out of scope" not in ln.lower() and "NFR-15" not in ln and "FR-OPT" not in ln and "options are not supported" not in ln and "option contract" not in ln.lower():
        opt_lines.append(ln.strip()[:90])
report("§6 free of options feature text (manual review list)", True, f"{len(opt_lines)} lines to eyeball")
for l in opt_lines:
    print("      ", l)

mods = {}
for i in ids6:
    mods[i.split("-")[1]] = mods.get(i.split("-")[1], 0) + 1
print(f"COUNT §6 = {len(ids6)} :: " + ", ".join(f"{k} {v}" for k, v in mods.items()))
whole10 = [i for i in ids10 if not i.endswith("-opt")]
print(f"COUNT §10 = {len(ids10)} ({len(whole10)} whole FRs, {len(ids10)-len(whole10)} -opt fragments)")
oq_defined = set(re.findall(r"^\| (OQ-\d\d) \|", sec9, re.M)) | set(re.findall(r"^\| (OQ-§10-\d) \|", sec10, re.M))
oq_refs = set(re.findall(r"\bOQ-(?:§10-)?\d+\b", req))
oq_bad = sorted(r for r in oq_refs if r not in oq_defined)
q_old = sorted(set(re.findall(r"\bQ-\d\d\b", req)))
report("every OQ reference exists in §9/§10.0 and no old Q-xx ids remain", not oq_bad and not q_old, f"bad {oq_bad}; old {q_old}")
print(f"§7 rows: {len(re.findall(r'^[|] FR-', sec7, re.M))}; §8 rows: {len(rows8)}; §9 questions: {len(oq_defined)}")
sys.exit(0 if ok else 1)
