# Porting conventions for package `ta`

Package `ta` is a pure-Go port of TA-Lib **v0.8.1** (BSD-3-Clause). Every TA-Lib function is
ported as a set of three files and verified against the real C library (the "oracle").
`sma.go`, `sma_test.go` and `sma_oracle_test.go` are the reference example. Copy their shape.

## Sources for function `XXX` (lower-case name `xxx`)

All under `../third_party/ta-lib/`:

| File | Use it for |
|---|---|
| `ta_codegen/input/xxx/xxx.yaml` | Inputs, optional inputs (**range**, default), outputs, flags |
| `ta_codegen/input/xxx/xxx.md` | Formula and notes |
| `ta_codegen/input/xxx/xxx.c` | Readable algorithm source |
| `src/ta_func/ta_XXX.c` | **Ground truth**: the compiled C, including the validation prologue. Port the batch function `TA_XXX` and `TA_XXX_Lookback`. Ignore the `_Stream*` functions and the `TA_S_XXX` single-precision variant. |
| `ta_codegen/output/rust/library/src/ta_func/xxx.rs` | A second rendering of the same logic, if you need one |

When the sources disagree, the compiled C in `src/ta_func` wins, because the oracle *is* that code.

## Naming

- Go name = TA-Lib name with underscores removed, upper case: `HT_DCPERIOD` → `HTDCPERIOD`,
  `MINUS_DI` → `MINUSDI`, `CDL3BLACKCROWS` → `CDL3BLACKCROWS`.
- Unexported identifiers use the lower-case Go name as a prefix: `htdcperiodLookback`,
  `htdcperiodRange`. **Every** unexported helper or constant you add must start with that prefix
  (e.g. `htdcperiodSmooth`), so 200 parallel files never collide.
- Files: `<lowergoname>.go`, `<lowergoname>_test.go`, `<lowergoname>_oracle_test.go`.
- Parameter names keep the TA-Lib names (`optInTimePeriod`, `inHigh`, ...).

## Required API per function

```go
// XXXLookback: validated; returns -1 on an invalid parameter (like C).
func XXXLookback(<opt params>) int
func xxxLookback(<opt params>) int          // unvalidated, used internally

// Public form: index-aligned outputs, len(out) == len(inputs).
// Float outputs: NaN inside the lookback. Integer outputs: []int, 0 inside the lookback.
// Multiple outputs are returned in TA-Lib output order, then error.
func XXX(<inputs []float64>, <opt params>) (<outputs>, error)

// Range form: mirrors TA_XXX exactly (startIdx/endIdx/outBegIdx/outNBElement).
func xxxRange(startIdx, endIdx int, <inputs>, <opt params>, <outputs>) (outBegIdx, outNBElement int, err error)
```

Parameter types: integer opt inputs → `int`; real → `float64`; `enum:MAType` → `MAType`.
Price inputs (`type: price`) expand to the listed components in OHLCV order
(`inOpen, inHigh, inLow, inClose, inVolume`) and only the components the yaml lists.

### Validation (must match C)

- Range form: `checkRange(startIdx, endIdx)` first, then every opt input against its yaml range with
  `checkInt` / `checkReal` / `checkMAType`, then `checkBuf` for inputs (to `endIdx`) and outputs
  (to `endIdx-startIdx`); `checkIntBuf` for integer outputs.
- Public form: validate opt inputs, then `checkInputs` (equal lengths; nil = empty). Empty input →
  empty outputs, nil error. Then call the range form with `0, n-1` and shape outputs with
  `alignReal` / `alignInt`.
- `TA_INTEGER_DEFAULT` / `TA_REAL_DEFAULT` / `MATypeDefault` substitution: C replaces the sentinel
  with the yaml default. In Go there is no sentinel for ints/reals (callers pass real values); for
  `MAType` parameters, map `MATypeDefault` to what the C prologue substitutes.
- Every validation error must satisfy `errors.Is(err, ErrBadParam)` (use the helpers).

### Porting the arithmetic

- Port the compiled C statement by statement. Keep operation order and accumulation order: the
  oracle tolerance (`relTol` = 1e-9 relative) leaves room only for last-ulp differences.
- Use `cMax`/`cMin`/`cMaxInt`/`cMinInt` for C's `max`/`min` macros (not Go's builtins or
  `math.Max`, which treat NaN differently). Use `math.Abs` for `fabs`.
- Use the shared helpers in `helpers.go`: `isZero`, `isZeroOrNeg`, `isZeroScaled`, `realEq`,
  `trueRange`, the candle helpers (`realBody`, `candleColor`, `upperShadow`, `lowerShadow`,
  `highLowRange`, `realBodyGapUp/Down`, `candleGapUp/Down`, `candleRange`, `candleAverage`), and
  `htSinCos` (httrig.go) for the Hilbert-transform DFT.
  Do **not** edit shared files (`helpers.go`, `httrig.go`, `libm.go`, `errors.go`, `settings.go`, `testutil_test.go`,
  `oracleutil_oracle_test.go`, `internal/oracle`). If you need a missing shared helper, write a
  prefixed private copy in your own file and mention it in your report.
- Unstable periods: read with `UnstablePeriod(UnstXXX)`.
- Candle settings: take one `cs := candleSettingsSnapshot()` at the top of the range form and of the
  lookback, and use `cs[BodyLong].AvgPeriod`, `candleAverage(cs[BodyLong], sum, o, h, l, c)` etc.
- Calling another indicator: call its range form (`emaRange(...)`), exactly where C calls `TA_EMA`
  or `TA_INT_EMA`. If C calls an internal variant (`TA_INT_EMA` with a precomputed k, etc.) and the
  dependency's file does not expose an equivalent, add a prefixed private copy in your own file
  rather than editing the dependency's file.
- The TA-Lib C library is built with `-ffp-contract=off`: port explicit `fma()` as `math.FMA` (called
  directly; an alias such as `fma := math.FMA` falls back to the ~50x slower software FMA), and wrap
  every other product that is later added or subtracted in `float64(...)`, including a product stored
  in a variable on an earlier line (`x *= k` then `x -= y` fuses too). `TestNoImplicitFMA` disassembles
  the package and fails on any fused multiply-add outside a `math.FMA` line.
- Call transcendental functions through `libm.go` (`libmAtan`, `libmSin`, `libmExp`, ...), never `math.*`
  directly. `go test -tags oracle,libmc` swaps in the C libm, and then every function must match C bit for
  bit: `FUZZ_EXACT=1 go test -tags oracle,libmc -run TestFuzzOracle`.
- C writes outputs into caller buffers, sometimes reusing an input buffer in place; replicate any
  temporary buffers with `make`.

## Required tests

`xxx_test.go` (pure Go, always runs):
1. **Parameters**: each opt input at min, max, min-1, max+1 (and NaN for reals) → error / `-1`
   lookback; `MAType` out of range; mismatched input lengths; nil/empty input → empty output.
2. **Lengths**: for several parameter sets, every `edgeLengths(lookback)` × every `fixtures(n)`
   → `len(out) == n`, and exactly `lookback` leading NaN (or 0 for integer outputs) where the
   function cannot legitimately emit NaN (`nan_inf_output` flag).
3. **Known values**: at least one hand-checked case from the formula in `xxx.md` (a tiny series
   whose answer you can derive), plus behavioural properties: constant input, period-1 identity
   (`period1_identity` flag), bounds (e.g. RSI in [0,100], patterns in {-200,-100,-80,0,80,100,200}).
4. **Range form**: bad startIdx/endIdx → the index errors; a range entirely inside the lookback →
   (0, 0, nil); a too-short buffer → `ErrBadParam`.

`xxx_oracle_test.go` (`//go:build oracle`, compares with C):
1. **Lookback** equals `mustLookback` for every parameter set tested.
2. **Full series**: every `fixtures(n)` × every `edgeLengths(lb)` (skip n = 0) × a spread of
   parameters (min, max where practical, defaults, typical values; every `MAType` for MAType
   params) → outputs equal `r.AlignedReal(i, n)` / `r.AlignedInt(i, n)` with `relTol`.
3. **Bad parameters**: C returns `oracle.BadParam` exactly when Go returns an error.
4. **Sub-ranges**: several `[startIdx, endIdx]` (start inside lookback, single bar, end of series)
   → same `outBegIdx`, `outNBElement` and values as `mustCallRange`.
5. **Settings** (when applicable): with a non-zero unstable period (`SetUnstablePeriod` here and
   `oracle.SetUnstablePeriod`), and for patterns with modified candle settings
   (`SetCandleSettings` + `oracle.SetCandleSettings`). Always restore with `defer`
   (`SetAllUnstablePeriods(0)`, `oracle.ResetUnstablePeriods()`, `RestoreAllCandleDefaultSettings()`,
   `oracle.RestoreCandleDefaults()`).
6. Patterns: also assert, on a long fixture, that the pattern fires at least once in Go and C for
   some fixture, or construct a series that makes it fire; a recogniser that never fires proves
   nothing.

Never use `t.Parallel()` (settings are global). Never loosen `relTol` without a written reason in a
comment next to the override.

## Commands (run from `ta/`)

```
go vet ./... && go vet -tags oracle ./...
go test -count=1 -run 'XXX' ./...
go test -tags oracle -count=1 -run 'XXX' ./...
FUZZ_ONLY=XXX FUZZ_ITERS=2000 go test -tags oracle -run TestFuzzOracle .                 # random differential
FUZZ_EXACT=1 FUZZ_ONLY=XXX go test -tags oracle,libmc -run TestFuzzOracle .              # bit-exact vs C
```

`TestFuzzOracle` (fuzz_oracle_test.go) drives every function in `fuzzregistry_test.go` through
reflection and the C abstract API: random series (NaN/±Inf, ties on a tick grid, extreme magnitudes,
inconsistent OHLC), random valid and invalid parameters, random sub-ranges, in-place output, random
unstable periods and candle settings. A new function must be added to the registry.
