# TA-Lib → Go port: progress

Goal (user, 2026-10-12): port all of TA-Lib v0.8.1 (201 functions: 140 indicators + 61 candlestick
patterns) to pure Go, one function at a time, tested to the boundary conditions and verified
against the C library. One agent per function, 10 at a time.

## Layout
- `ta/` Go module `tradeloom/ta` (package `ta`). Rules: `CONVENTIONS.md`. Template: `sma*.go`.
- `ta/internal/oracle` cgo bridge to the C library through the abstract API (build tag `oracle`).
- `third_party/ta-lib/` TA-Lib v0.8.1 source (git tag v0.8.1); C oracle built at
  `third_party/ta-lib/build/libta-lib.a` (`cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF && cmake --build build -j16`).
- Agents work in `ta/_work/<name>/` copies and land their 3 files into `ta/` once green.

## Decisions
- Public API: slice in/out, index-aligned, NaN (float) / 0 (int) inside the lookback; error on bad
  params (`errors.Is(err, ErrBadParam)`); nil input = empty. Range form mirrors C exactly.
- Names: TA-Lib name without underscores, upper case (`HT_DCPERIOD` → `HTDCPERIOD`).
- Oracle tolerance 1e-9 relative (arm64 FMA may differ from C in the last ulps).
- Out of scope for this pass: TA-Lib's streaming (incremental) API; single-precision variants.

## Status
- [x] Foundation: module, settings, helpers, errors, oracle bridge, test fixtures, SMA template (passes vs C)
- [x] Pilot: EMA, ATR, CDLHAMMER (all pass vs C; reviewed)
- [x] Remaining 197 functions (dependency-ordered workflow; 0 failures)
- [x] HT_DCPHASE / HT_SINE / HT_TRENDMODE unified on one libm sin/cos table (`httrig.go`); fixed 3 wrong
      entries in HT_DCPHASE's private table and removed HT_SINE's noise-bar test exemption
- [x] Full-suite verification (2026-10-12): gofmt/vet clean, `CGO_ENABLED=0 go build` OK,
      1821 tests pass with `-tags oracle`, 0 fail

- [x] Second verification pass (2026-10-12), see below: randomized differential fuzzer, FMA audit,
      libm-exact mode, concurrency, performance, mutation testing

## Second verification pass (2026-10-12)
- `TestFuzzOracle`: every function vs C on random data/params/ranges/settings. 6 seeds x 2000 cases x
  201 functions, plus 100k-bar series: 0 mismatches.
- Found and fixed: 83 implicit fused multiply-adds in 22 files (WMA, HMA, RSI, CMO, ADX, DX, AD, ADOSC,
  PVT, KC, ACCBANDS, COPPOCK, T3, SAREXT, ULTOSC, CDLDARKCLOUDCOVER, ...). Each gave last-ulp
  differences that MAMA amplified to visible errors (MACDEXT with a MAMA signal, KDJ on 100k bars).
  `TestNoImplicitFMA` now guards this.
- With `-tags oracle,libmc` (C libm instead of Go's math) every function matches C bit for bit on all
  fuzz cases, so the only remaining differences in normal builds are Go-math vs libm last ulps.
- T3 was 49x slower than C (aliased math.FMA ran in software); fixed. MIN/MAX-family functions are
  3-5x slower than C (bounds checks in the block algorithm), ~4 ns/bar: optimisation candidate.
- `TestConcurrentCalls` (-race): 16 goroutines x 201 functions while settings are rewritten: clean.
- Mutation testing (579 random single-operator mutants, 3 per function): 471 killed by the fuzzer
  (strict mode, 2000 cases); of the rest ~95 are equivalent (startIdx clamps, value-only min/max ties,
  `>=` adding zero, symmetric HT even/odd branches) and ~7 are exact-tie comparisons inside rare
  multi-candle patterns, killed only with 20000 cases. Patterns now get 5x fuzz cases by default.

## Known, documented deviations from C (all within the tests)
- Transcendentals (ACOS..TANH, LINEARREG_ANGLE, HT_*, MAMA and anything using MA type MAMA) use Go's
  math, which differs from libm in the last ulp; MAMA's feedback can grow this to ~1e-9 relative on
  very long series. Bit-exact with `-tags libmc`.
- EXP: Go's math.Exp underflows early on darwin/arm64; libmExp uses a pure-Go fdlibm copy below -708.
  May differ from libm by 1 subnormal ulp (~1e-312).
- COSH/SINH: overflow band |x| in (709.78, 710.48] handled in libm.go to stay finite like libm.
- No TA_INTEGER_DEFAULT/TA_REAL_DEFAULT sentinels in Go: those values are rejected, not substituted.
- `httrig.go` holds macOS-arm64 libm sin/cos values; it only affects degenerate (flat/periodic) inputs.

## Next
- Streaming (incremental) API for live ticks.
- Benchmarks vs C.
