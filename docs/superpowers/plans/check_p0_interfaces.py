#!/usr/bin/env python3
"""Cross-plan interface check for the Tradeloom Phase 0 sub-plans.

Checks, over 2026-10-12-p0-1 … p0-5:
  1. Every identifier referenced in a Consumes block (Go name, TS binding `Struct.Method`, event name,
     React component/hook/helper) is defined in some Produces block, in a code block of an earlier task,
     or belongs to a known library.
  2. No Go function/method is defined in two Produces blocks with different signatures
     (keyed by package + receiver type + name).
  3. No TS binding `Struct.Method` is declared with two different parameter lists.
  4. The plans contain no placeholder markers (TBD / TODO / implement later / similar to Task).
  5. Every binding DTO that P0-4/P0-5 declare as a TS interface has exactly the JSON field names of the
     P0-3 Go struct of the same name (field-for-field, both directions).

Usage: python3 check_p0_interfaces.py [--verbose] [--ledger <out.md>]      (exit 1 when anything fails)
"""
from __future__ import annotations

import os
import re
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
PLANS = [
    ("P0-1", "2026-10-12-p0-1-platform-core"),
    ("P0-2", "2026-10-12-p0-2-market-data"),
    ("P0-3", "2026-10-12-p0-3-session-service"),
    ("P0-4", "2026-10-12-p0-4-frontend-shell"),
    ("P0-5", "2026-10-12-p0-5-chart-watchlists"),
]

# Names that come from libraries, the standard library, test harnesses or language keywords — never
# produced by a plan, so a Consumes mention of them is fine.
LIBRARY = {
    # Go std / third party
    "context", "errors", "time", "sync", "slog", "json", "http", "url", "fs", "sql", "io", "os",
    "testing", "reflect", "atomic", "kiteconnect", "kiteticker", "models", "wails", "runtime",
    "options", "assetserver", "logger", "flock", "dbus", "websocket", "httptest", "zstd", "argon2",
    "chacha20poly1305", "keyring", "lumberjack", "packages", "mock", "Error", "New", "Client",
    "SetHTTPClient", "SetBaseURI", "SetAccessToken", "GetLoginURLWithparams", "GenerateSession",
    "InvalidateAccessToken", "GetUserProfile", "GetHistoricalData", "GetOrders", "GetPositions",
    "GetHoldings", "GetLTP", "UserProfile", "Order", "Orders", "Position", "Positions", "Holding",
    "Holdings", "QuoteLTP", "QuoteOHLC", "HistoricalData", "Instrument", "Instruments", "Tick",
    "SetRootURL", "SetAutoReconnect", "SetReconnectMaxRetries", "SetConnectTimeout", "OnConnect",
    "OnError", "OnNoReconnect", "ServeWithContext", "Stop", "Conn", "Ticker", "EventsEmit",
    "EventsOn", "BrowserOpenURL", "WindowShow", "WindowUnminimise", "WindowGetSize",
    "WindowGetPosition", "WindowIsMaximised", "WindowSetSize", "WindowSetPosition", "WindowMaximise",
    "OpenDirectoryDialog", "OpenFileDialog", "Quit", "App", "ErrorFormatter", "SingleInstanceLock",
    "SecondInstanceData", "ERROR", "INFO", "TryLock", "Unlock", "ConnectSessionBus", "Object",
    "Call", "Store", "Export", "RequestName", "NameFlagDoNotQueue", "BusObject", "Flags",
    "GetServerInformation", "Notifications", "TokenError", "PermissionError", "InputError",
    "NetworkError", "OrderParams", "OrderResponse", "ConvertPositionParams", "GTTParams",
    "GTTResponse", "DoRaw", "AxeBuilder", "withTags", "analyze", "createFrom", "typescriptify",
    "Time", "Date", "OHLC", "LastPrice", "InstrumentToken", "VolumeTraded", "Timestamp", "Close",
    "Error", "Code", "ErrorType", "Message", "Data", "GetLoginURL", "SingleInstance", "SetupSingleInstance", "Frontend",
    # TS / React / libraries
    "react", "zustand", "vitest", "playwright", "vite", "tsc", "eslint", "RouteObject", "ReactNode",
    "ReactElement", "RefObject", "ComponentType", "ComponentPropsWithRef", "KeyboardEvent",
    "NavigateFunction", "useNavigate", "createMemoryRouter", "HTMLElement", "Element", "Promise",
    "Record", "Omit", "Pick", "Partial", "Array", "window", "localStorage", "describe", "it",
    "expect", "vi", "page", "locator", "UTCTimestamp", "lightweight-charts", "LwcLib",
    "jest-dom", "jsdom", "Playwright", "AxeBuilder",
}

# Tokens in Consumes blocks that are prose, not identifiers.
PROSE = {
    "Create", "Modify", "Generated", "Consumes", "Produces", "Task", "Tasks", "Step", "P0", "P1",
    "P3", "P4", "FR", "AC1", "AC2", "AC3", "AC4", "AC5", "AC6", "ARCH", "ARCHITECTURE", "A07",
    "OQ", "S0", "ROADMAP", "CI", "JSON", "TS", "Go", "SQL", "HTTP", "URL", "ID", "IST", "UTC",
    "NSE", "BSE", "NFO", "BFO", "EQ", "FUT", "ms", "Ms",
}

GO_FUNC = re.compile(r"^\s*func\s+(?:\((?P<recv>[^)]*)\)\s*)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*(?P<gen>\[[^\]]*\])?\s*\((?P<sig>.*)$")
GO_DECL = re.compile(r"^\s*(?:type|var|const)\s+([A-Z][A-Za-z0-9_]*)\b")
GO_INLINE = re.compile(r"`(?:func\s+(?:\([^)]*\)\s*)?)?([A-Z][A-Za-z0-9_]*)(?:\[[^\]]*\])?\(")
TS_EXPORT = re.compile(r"^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|const|class|interface|type|enum|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)")
TS_BINDING = re.compile(r"\b([A-Z][A-Za-z]+)\.([A-Z][A-Za-z]+)\(([^)]*)\)")
BOUND = "Session|Portfolio|Settings|Chart|Instruments|Quotes|Watchlists|Data|Calendar|Diagnostics|System"
TS_API_ROW = re.compile(r"^\|\s*`(?:api/)?(" + BOUND + r")`\s*\|(.*)$")
# a Consumes table cell such as `Watchlists.Rename` / `Delete` / `SetActive` or `Session.Login()`
TS_BIND_CELL = re.compile(r"^\|\s*(`(?:" + BOUND + r")\.[A-Z][A-Za-z]+(?:\([^)]*\))?`[^|]*)\|")
GO_TOPIC = re.compile(r'NewTopic\[[^\]]*\]\("([a-z][a-z_]*\.[a-z][a-z_.]*)"\)|\bEv[A-Za-z]+\s*=\s*"([a-z][a-z_]*\.[a-z][a-z_.]*)"')
GO_KEY = re.compile(r'Key\[[^\]]*\]\{\s*Name:\s*"([a-z][a-z_]*(?:\.[a-z][a-z_]*)+)"|`"([a-z][a-z_]*(?:\.[a-z][a-z_]*)+)"`|ID:\s*"([a-z][a-z_]*\.[a-zA-Z][a-zA-Z0-9_]*)"')
EVENT = re.compile(r"[`\"']([a-z][a-z_]*\.[a-z][a-z_]*(?:\.[a-z_]+)*)[`\"']")
EVENT_EXCLUDE_SUFFIX = (".go", ".ts", ".tsx", ".md", ".json", ".yml", ".yaml", ".css", ".sql", ".html",
                        ".js", ".sh", ".txt", ".db", ".csv", ".gz", ".enc", ".zip", ".log", ".deb",
                        ".py", ".mod", ".sum", ".lock", ".png", ".svg", ".mock", ".trade", ".com", ".org",
                        ".test", ".spec", ".config", ".tsbuildinfo", ".dev", ".jest")
BACKTICK = re.compile(r"`([^`]+)`")
IDENT = re.compile(r"[A-Za-z_$][A-Za-z0-9_$]*")


def read(name: str) -> list[str]:
    """Reassemble a split plan directory (MANIFEST.md + tasks/ + APPENDIX.md) into its monolithic lines."""
    sys.path.insert(0, HERE)
    from plan_split import join
    return join(os.path.join(HERE, name)).splitlines()


def sections(lines: list[str]):
    """Yield (kind, task, block_lines) for every Consumes / Produces block and every fenced code block.

    kind ∈ {"consumes", "produces", "code"}; task is the current `### Task N:` heading or a top-level
    section title.
    """
    task = "(preamble)"
    kind = None
    buf: list[str] = []
    in_fence = False
    fence_task = None
    fence_kind = None
    for raw in lines:
        line = raw.rstrip("\n")
        if line.startswith("### Task "):
            if kind:
                yield kind, task, buf
                kind, buf = None, []
            task = line[4:].strip()
            continue
        if line.startswith("## ") and not in_fence:
            if kind:
                yield kind, task, buf
                kind, buf = None, []
            title = line[3:].strip()
            low = title.lower()
            if low.startswith("consumes"):
                kind, buf = "consumes", []
                task = title
                continue
            if low.startswith("produces"):
                kind, buf = "produces", []
                task = title
                continue
            task = title
            continue
        if line.startswith("```"):
            if in_fence:
                in_fence = False
                if fence_kind != "consumes":  # stubs quoted inside a Consumes section are not productions
                    yield "code", fence_task, buf_code  # type: ignore[name-defined]
            else:
                in_fence = True
                fence_task = task
                fence_kind = kind
                buf_code = []  # type: ignore[assignment]
            if kind:
                buf.append(line)
            continue
        if in_fence:
            buf_code.append(line)  # type: ignore[name-defined]
            if kind:
                buf.append(line)
            continue
        s = line.lstrip()
        if s.startswith("- Consumes") or s.startswith("**Consumes") or s.startswith("- **Consumes"):
            if kind:
                yield kind, task, buf
            kind, buf = "consumes", [line]
            continue
        if s.startswith("- Produces") or s.startswith("**Produces") or s.startswith("- **Produces"):
            if kind:
                yield kind, task, buf
            kind, buf = "produces", [line]
            continue
        if kind:
            # a block ends at the first step heading or blank line followed by a non-indented line
            if s.startswith("- [ ] **Step") or s.startswith("**Step") or s.startswith("**Files") or line.startswith("- Create") or line.startswith("- Modify"):
                yield kind, task, buf
                kind, buf = None, []
                continue
            buf.append(line)
    if kind:
        yield kind, task, buf


def go_package_of(task_block: list[str]) -> str:
    for line in task_block:
        m = re.search(r"`(internal/[A-Za-z0-9_/]+)/[A-Za-z0-9_]+\.go`", line)
        if m:
            return m.group(1)
    return ""


def collect(plan: str, lines: list[str]):
    produced_names: dict[str, set[str]] = defaultdict(set)      # bare name → {plan:task}
    produced_quals: dict[str, set[str]] = defaultdict(set)      # pkg.Name / Struct.Method → {plan:task}
    go_sigs: dict[tuple[str, str, str], dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
    ts_bindings: dict[str, dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
    events: dict[str, set[str]] = defaultdict(set)
    keys: dict[str, set[str]] = defaultdict(set)                 # settings keys and shortcut ids (Go-declared)
    consumed: list[tuple[str, str, str]] = []                    # (plan:task, token, kind)
    placeholders: list[str] = []
    go_plan = plan in ("P0-1", "P0-2", "P0-3")                    # only Go plans declare topics and keys

    # package context per task (from the Files block)
    task_pkg: dict[str, str] = {}
    cur = "(preamble)"
    cur_files: list[str] = []
    for line in lines:
        if line.startswith("### Task "):
            if cur_files:
                task_pkg[cur] = go_package_of(cur_files)
            cur, cur_files = line[4:].strip(), []
        elif line.startswith("- Create:") or line.startswith("- Modify") or line.startswith("- Generated"):
            cur_files.append(line)
    if cur_files:
        task_pkg[cur] = go_package_of(cur_files)

    for kind, task, block in sections(plan, lines) if False else sections(lines):
        where = f"{plan}:{task}"
        text = "\n".join(block)
        if kind in ("produces", "code"):
            pkg = task_pkg.get(task, "")
            in_go = False
            for line in block:
                if line.startswith("```"):
                    in_go = line.strip() in ("```go",)
                    continue
                mp = re.match(r"^\s*(?://\s*)?package\s+([a-z][a-z0-9_]*)\b", line)
                if mp:
                    pkg = (pkg.rsplit("/", 1)[0] + "/" if "/" in pkg else "internal/") + mp.group(1)
                m = GO_FUNC.match(line)
                if m:
                    recv = re.sub(r"\s+", "", m.group("recv") or "")
                    recv_type = re.sub(r"^[a-zA-Z_]*\*?", "", recv) if recv else ""
                    recv_type = recv_type.lstrip("*")
                    name = m.group("name")
                    sig = re.sub(r"\s+", " ", (m.group("gen") or "") + "(" + m.group("sig")).strip()
                    sig = sig.split("{")[0].strip()
                    produced_names[name].add(where)
                    if recv_type:
                        produced_quals[f"{recv_type}.{name}"].add(where)
                    if pkg:
                        produced_quals[f"{pkg.split('/')[-1]}.{name}"].add(where)
                    if kind == "produces" and name[0].isupper():
                        go_sigs[(pkg, recv_type, name)][sig].add(where)
                m = GO_DECL.match(line)
                if m:
                    produced_names[m.group(1)].add(where)
                    if pkg:
                        produced_quals[f"{pkg.split('/')[-1]}.{m.group(1)}"].add(where)
                mc = re.match(r"^\s+([A-Z][A-Za-z0-9_]*)\s*(?://.*)?$", line)
                if mc and (kind == "code" or in_go or line.startswith("\t")):
                    produced_names[mc.group(1)].add(where)
                    if pkg:
                        produced_quals[f"{pkg.split('/')[-1]}.{mc.group(1)}"].add(where)
                # const / struct-field names inside Go blocks: "	KindSession  Kind = iota" / "	Token uint32 `json:…`"
                mf = re.match(r"^\s+([A-Z][A-Za-z0-9_]*(?:,\s*[A-Z][A-Za-z0-9_]*)*)\s+(?:=|[A-Za-z*\[\]]|map\b|func\b|chan\b|<-)", line)
                if mf and (kind == "code" or in_go or line.startswith("\t")):
                    for nm in re.split(r",\s*", mf.group(1)):
                        produced_names[nm].add(where)
                        if pkg:
                            produced_quals[f"{pkg.split('/')[-1]}.{nm}"].add(where)
                for m in GO_INLINE.finditer(line):
                    produced_names[m.group(1)].add(where)
                m = TS_EXPORT.match(line)
                if m:
                    produced_names[m.group(1)].add(where)
                mm = re.match(r"^\s+(?:readonly\s+)?([a-z][A-Za-z0-9_$]*)(?:<[^>]*>)?\??\(", line)
                if mm and kind == "code":
                    produced_names[mm.group(1)].add(where)  # TS interface / object members such as on<K>(name, cb)
                # TS binding tables: | `api/Session` | `Status()`, `Login()` … |
                m = TS_API_ROW.match(line)
                if m:
                    struct = m.group(1)
                    for mm in re.finditer(r"`([A-Z][A-Za-z]+)\(([^)]*)\)", m.group(2)):
                        produced_quals[f"{struct}.{mm.group(1)}"].add(where)
                        produced_names[mm.group(1)].add(where)
                        ts_bindings[f"{struct}.{mm.group(1)}"][re.sub(r"\s+", " ", mm.group(2)).strip()].add(where)
                # Go bound-struct methods: func (s *Session) Status() … → binding Session.Status
                mm = re.match(r"^\s*func \([a-z]+ \*([A-Z][A-Za-z]+)\) ([A-Z][A-Za-z]+)\(([^)]*)\)", line)
                if mm and kind == "code" and "internal/app/api" in pkg:
                    produced_quals[f"{mm.group(1)}.{mm.group(2)}"].add(where)
                if go_plan:
                    for m in GO_TOPIC.finditer(line):
                        events[m.group(1) or m.group(2)].add(where)
                    for m in GO_KEY.finditer(line):
                        k = m.group(1) or m.group(2) or m.group(3)
                        if not k.endswith(EVENT_EXCLUDE_SUFFIX):
                            keys[k].add(where)
                    if kind == "produces":
                        # table rows of the binding/event catalogue: | `session.state` | payload | …
                        mr = re.match(r"^\|\s*`([a-z][a-z_]*\.[a-z][a-z_.]*)`\s*\|", line)
                        if mr and not mr.group(1).endswith(EVENT_EXCLUDE_SUFFIX):
                            target = events if re.search(r"immediate|coalesc|latest|merged|/ 100 ms|payload", line, re.I) else keys
                            target[mr.group(1)].add(where)
                        if re.search(r"\b(action id|shortcut|settings? key|Keys?:)", line, re.I):
                            for mk in re.finditer(r"`([a-z][a-z_]*(?:\.[a-zA-Z][a-zA-Z0-9_]*)+)`", line):
                                if not mk.group(1).endswith(EVENT_EXCLUDE_SUFFIX):
                                    keys[mk.group(1)].add(where)
                for mj in re.finditer(r'json:"([A-Za-z_][A-Za-z0-9_]*)', line):
                    produced_names[mj.group(1)].add(where)
                for m in BACKTICK.finditer(line):
                    tok = m.group(1)
                    for mc in re.finditer(r"\b([a-z][A-Za-z0-9_$]*)\(", tok):
                        produced_names[mc.group(1)].add(where)  # members such as `useProfile(): { profile; reload() }`
                    if re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*", tok):
                        produced_names[tok].add(where)
                    if re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)+", tok):
                        produced_quals[tok].add(where)
                    mq = re.fullmatch(r"<?([A-Z][A-Za-z0-9]*)\b.*>?", tok)
                    if mq and tok.startswith("<"):
                        produced_names[mq.group(1)].add(where)
                    for mq in re.finditer(r"\b([a-z][A-Za-z0-9]*|[A-Z][A-Za-z0-9]*)\.([A-Z][A-Za-z0-9_]*)", tok):
                        produced_quals[f"{mq.group(1)}.{mq.group(2)}"].add(where)
                        produced_names[mq.group(2)].add(where)
        if kind == "consumes":
            for line in block:
                for m in BACKTICK.finditer(line):
                    tok = m.group(1)
                    if "/" in tok and not re.search(r"\(", tok) and not tok.startswith("(*"):
                        parts = [p for p in re.split(r"[\s/,]+", tok) if p]
                    else:
                        parts = [tok]
                    for part in parts:
                        part = part.strip()
                        if part.startswith("(*"):
                            part = re.sub(r"^\(\*([A-Za-z0-9_.]+)\)\.", r"\1.", part)
                        part = re.sub(r"\[[^\]]*\]", "", part)
                        part = part.split("(")[0]
                        part = part.split("{")[0]
                        part = part.strip(" .")
                        if not part or part.startswith("internal/") or part.startswith("frontend/") or part.startswith("docs/") or part.startswith("cmd/"):
                            continue
                        if re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)*", part):
                            consumed.append((where, part, "name"))
                for m in TS_BINDING.finditer(line):
                    consumed.append((where, f"{m.group(1)}.{m.group(2)}", "binding"))
                mb = TS_BIND_CELL.match(line)
                if mb:
                    struct = None
                    for mm in re.finditer(r"`(?:(" + BOUND + r")\.)?([A-Z][A-Za-z]+)(?:\([^)]*\))?`", mb.group(1)):
                        struct = mm.group(1) or struct
                        if struct:
                            consumed.append((where, f"{struct}.{mm.group(2)}", "binding"))
                for m in EVENT.finditer(line):
                    ev = m.group(1)
                    if not ev.endswith(EVENT_EXCLUDE_SUFFIX):
                        consumed.append((where, ev, "event"))
        for i, line in enumerate(block):
            if re.search(r"\bTBD\b|\bTODO\b|implement later|similar to Task", line):
                placeholders.append(f"{where}: {line.strip()[:120]}")
    return produced_names, produced_quals, go_sigs, ts_bindings, events, keys, consumed, placeholders


def short(where: str) -> str:
    """P0-3:Task 16: Chart data … → P0-3 T16"""
    m = re.match(r"(P0-\d):Task (\d+)", where)
    if m:
        return f"{m.group(1)} T{m.group(2)}"
    m = re.match(r"(P0-\d):", where)
    return m.group(1) + " hdr" if m else where


def write_ledger(path: str, names, quals, events, keys, bindings, consumed) -> None:
    """Markdown ledger: one row per consumed interface name (plus every event and TS binding)."""
    rows: dict[tuple[str, str], tuple[set[str], set[str]]] = {}

    def add(name: str, kind: str, producers: set[str], consumer: str | None) -> None:
        key = (name, kind)
        prod, cons = rows.setdefault(key, (set(), set()))
        prod |= producers
        if consumer:
            cons.add(consumer)

    for where, tok, kind in consumed:
        if kind == "event":
            if tok in events:
                add(tok, "event", events[tok], where)
            elif tok in keys:
                add(tok, "key", keys[tok], where)
            continue
        if kind == "binding":
            if tok in quals:
                add(tok, "binding", quals[tok], where)
            continue
        parts = tok.split(".")
        last = parts[-1]
        if tok in quals:
            add(tok, "go/ts", quals[tok], where)
        elif len(parts) >= 2 and f"{parts[-2]}.{last}" in quals:
            add(f"{parts[-2]}.{last}", "go/ts", quals[f"{parts[-2]}.{last}"], where)
        elif tok in names:
            add(tok, "go/ts", names[tok], where)
        elif last in names and last[0].isupper():
            add(tok, "go/ts", names[last], where)
    for ev, prods in events.items():
        add(ev, "event", prods, None)
    for k, prods in keys.items():
        add(k, "key", prods, None)
    for b, variants in bindings.items():
        for _sig, prods in variants.items():
            add(b, "binding", prods, None)
    with open(path, "w", encoding="utf-8") as f:
        f.write("| Name | Kind | Producer (plan:task) | Consumers |\n|---|---|---|---|\n")
        for (name, kind), (prod, cons) in sorted(rows.items(), key=lambda kv: (kv[0][1], kv[0][0].lower())):
            pr = ", ".join(sorted({short(w) for w in prod}))
            co = ", ".join(sorted({short(w) for w in cons})) or "—"
            f.write(f"| `{name}` | {kind} | {pr} | {co} |\n")


DTO_NAMES = ["SessionStatus", "SecretStorageInfo", "CredentialsInfo", "LoginStart", "LogoutResult", "Profile",
             "BookSnapshot", "OrderRow", "SectionCounts", "PositionsSnapshot", "PositionRow", "HoldingsSnapshot",
             "HoldingRow", "HoldingsSummary", "TickDTO", "CandleDTO", "CandleEventDTO", "CandlesRequest",
             "CandlesResult", "RefreshRequest", "InstrumentDTO", "WatchResult", "WatchlistsState", "Watchlist",
             "WatchlistItem", "RemovedItem", "SettingValue", "ShortcutAction", "DataStats", "BackupInfo",
             "JobProgress", "CalendarInfo", "CalendarEntryDTO", "PhaseDTO", "CalendarEntryInput", "CrashInfo",
             "AppInfo", "Licence", "StatusSnapshot", "MarketStatus", "Navigate"]


def go_json_tags(text: str) -> dict[str, set[str]]:
    out: dict[str, set[str]] = {}
    for m in re.finditer(r"^type (\w+) struct \{\n(.*?)^\}", text, re.M | re.S):
        tags = re.findall(r'json:"([A-Za-z_][A-Za-z0-9_]*)', m.group(2))
        if tags:
            out[m.group(1)] = set(tags)  # the last definition in the plan wins
    return out


def ts_interfaces(lines: list[str]) -> dict[str, set[str]]:
    out: dict[str, set[str]] = {}
    i = 0
    while i < len(lines):
        m = re.match(r"^export interface (\w+)(?: extends [^{]+)? \{(.*)$", lines[i])
        if m:
            nm, rest = m.group(1), m.group(2)
            if "}" in rest:
                body = rest.split("}")[0]
            else:
                buf: list[str] = []
                i += 1
                while i < len(lines) and lines[i] != "}":
                    buf.append(lines[i])
                    i += 1
                body = "\n".join(buf)
            fields = set(re.findall(r"(?:^|[\s;{])(?:readonly\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\??:", body))
            if fields:
                out[nm] = fields
        i += 1
    return out


def check_dtos() -> list[str]:
    go = go_json_tags("\n".join(read(PLANS[2][1])))
    problems: list[str] = []
    for plan, fname in PLANS[3:]:
        ts = ts_interfaces(read(fname))
        for name in DTO_NAMES:
            if name in ts and name in go:
                ts_only, go_only = sorted(ts[name] - go[name]), sorted(go[name] - ts[name])
                if ts_only or go_only:
                    problems.append(f"{plan} {name}: TS-only={ts_only} Go-only={go_only}")
            elif name in ts and name not in go:
                problems.append(f"{plan} {name}: no Go struct with json tags in P0-3")
    return problems


def main() -> int:
    verbose = "--verbose" in sys.argv
    names: dict[str, set[str]] = defaultdict(set)
    quals: dict[str, set[str]] = defaultdict(set)
    sigs: dict[tuple[str, str, str], dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
    bindings: dict[str, dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
    events: dict[str, set[str]] = defaultdict(set)
    keys: dict[str, set[str]] = defaultdict(set)
    consumed: list[tuple[str, str, str]] = []
    placeholders: list[str] = []
    for plan, fname in PLANS:
        n, q, s, b, e, kk, c, p = collect(plan, read(fname))
        for k, v in kk.items():
            keys[k] |= v
        for k, v in n.items():
            names[k] |= v
        for k, v in q.items():
            quals[k] |= v
        for k, v in s.items():
            for sig, ws in v.items():
                sigs[k][sig] |= ws
        for k, v in b.items():
            for sig, ws in v.items():
                bindings[k][sig] |= ws
        for k, v in e.items():
            events[k] |= v
        consumed += c
        placeholders += p

    failures = 0
    missing: list[str] = []
    for where, tok, kind in consumed:
        if kind == "event":
            segs = tok.split(".")
            if tok in events or tok in keys:
                continue
            if all(sg in names for sg in segs):
                continue  # a JSON field path such as `secrets.mode`, not an event
            missing.append(f"[event/key] {where}: `{tok}` is consumed but no Go plan declares it as a topic, settings key or shortcut id")
            continue
        if kind == "binding":
            if tok in quals:
                continue
            struct, method = tok.split(".")
            if struct in LIBRARY or method in LIBRARY:
                continue
            missing.append(f"[binding] {where}: `{tok}` is consumed but no Produces block declares it")
            continue
        parts = tok.split(".")
        last = parts[-1]
        if re.search(r"\.(ts|tsx|go|db|mod|sum|png|org|json|yml|yaml|md|css|sql|html|js|sh|txt|csv|gz|enc|zip|log|deb|py|lock|svg|trade|com)$", tok):
            continue
        if re.fullmatch(r"[A-Z][A-Z0-9_]+", last) and last not in names:
            continue  # env vars, HTTP verbs, SQL keywords
        if re.fullmatch(r"[a-z][a-z0-9_]*", last) and "_" in last and last not in names:
            continue  # SQL table / column names
        if last in LIBRARY or last in PROSE or tok in LIBRARY or parts[0] in LIBRARY and len(parts) > 1 and last[0].isupper() and last in LIBRARY:
            continue
        if last[0].islower() and last not in names:
            # lower-case single words are prose unless some plan exports them (hooks, helpers)
            if not re.match(r"^(use|format|parse|create|render|push|focus|is|visible|apply|safe|register|reload|resolve|ensure|watch|fetch|on|load|save|write|open|go|tab|next|validate|quote|instrument|display|exchange|search|subscriptions|subscribe|toneOf|colourTokens|shellChildren|rootChildren|buildRoutes)(?=[A-Z_]|$)", last):
                continue
        if tok in quals or tok in names or last in names:
            continue
        if len(parts) >= 2 and f"{parts[-2]}.{last}" in quals:
            continue
        missing.append(f"[name]    {where}: `{tok}` is consumed but not produced anywhere")

    if missing:
        failures += len(missing)
        print(f"MISSING ({len(missing)}):")
        for m in sorted(set(missing)):
            print("  " + m)
    else:
        print("MISSING (0): every consumed Go name, TS binding and event is produced somewhere (or is a library name).")

    dup = 0
    for (pkg, recv, name), variants in sorted(sigs.items()):
        if len(variants) > 1:
            dup += 1
            print(f"SIGNATURE DRIFT: {pkg or '?'} {('(' + recv + ').') if recv else ''}{name}")
            for sig, ws in variants.items():
                print(f"    {sig}   ← {', '.join(sorted(ws))}")
    for b, variants in sorted(bindings.items()):
        if len(variants) > 1:
            dup += 1
            print(f"BINDING DRIFT: {b}")
            for sig, ws in variants.items():
                print(f"    ({sig})   ← {', '.join(sorted(ws))}")
    failures += dup
    print(f"SIGNATURE DRIFT ({dup}): {'none' if dup == 0 else 'see above'}")

    dto = check_dtos()
    failures += len(dto)
    if dto:
        print(f"DTO FIELD DRIFT ({len(dto)}):")
        for d in dto:
            print("  " + d)
    else:
        print("DTO FIELD DRIFT (0): every TS interface named like a P0-3 binding DTO has exactly its JSON field names.")

    if placeholders:
        failures += len(placeholders)
        print(f"PLACEHOLDERS ({len(placeholders)}):")
        for p in placeholders:
            print("  " + p)
    else:
        print("PLACEHOLDERS (0): no TBD / TODO / implement later / similar to Task in any Consumes/Produces/code block.")

    if verbose:
        print(f"\nproduced names: {len(names)}, qualified: {len(quals)}, events: {len(events)}, consumed tokens: {len(consumed)}")
        print("events: " + ", ".join(sorted(events)))
        print("keys: " + ", ".join(sorted(keys)))

    if "--ledger" in sys.argv:
        out = sys.argv[sys.argv.index("--ledger") + 1]
        write_ledger(out, names, quals, events, keys, bindings, consumed)
        print(f"ledger written to {out}")
    print("RESULT:", "PASS" if failures == 0 else f"FAIL ({failures})")
    return 0 if failures == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
