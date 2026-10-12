# S0-2 Candle store benchmark

Decides OQ-41 (ARCHITECTURE §4.3, §12.1) against FR-DATA-002 AC4: one instrument-day of 1-minute candles in < 50 ms and
"missing ranges for token T between A and B" in < 5 ms with 500 instruments x 2 years loaded; ingest >= 100 000 rows/s.

## Status

**Provisional.** The numbers below were measured on a macOS arm64 development machine, NOT the reference hardware of
REQUIREMENTS §5 (x86_64, 4 cores/8 threads, 16 GB, NVMe, Ubuntu 24.04). **The reference-hardware (Linux) run is pending**; re-run
Step 4 of the Task 13 plan there and replace this record before OQ-41 is closed.

## Machine (this run)

```
Darwin 25.5.0 arm64
Apple M4 Max
cores: 16 physical / 16 logical
RAM: 48 GB
go version go1.25.5 darwin/arm64
storage: internal Apple SSD (APFS)
```

## Procedure

`S02_INSTRUMENTS=500 S02_DAYS=500 go test -tags s02,noorders -run TestS02 ./internal/candles/sqlite/` (add `webkit2_41` on Linux) —
loads 500 x 500 trading days x 375 one-minute candles through `UpsertCovered` in 60-day windows (the backfill path),
then 5 x 200 random instrument-day reads and missing-range queries; worst of each 200, median and max of the 5 runs.

## Measurements (macOS arm64)

- S0-2 ingest: 93750000 rows in 3m55s = 398154 rows/s (gate ≥ 100000)
- S0-2 instrument-day read: worst-of-200 median 493.375µs, max 569µs (gate < 50ms)
- S0-2 missing-ranges: worst-of-200 median 91.5µs, max 163.167µs (gate < 5ms)
- S0-2 size: candles_1m 3605528576 bytes for 93750000 rows = 38.5 B/row; file 3604291584 bytes

Gates: ingest >= 100 000 rows/s, read < 50 ms, missing-ranges < 5 ms. All met on this machine, with wide margin (read ~100x, missing-ranges ~30x under budget).

## Decision

All gates met on the development machine: the SQLite layout of ARCHITECTURE §4.3 is provisionally confirmed for OQ-41; `candles/sqlite` is the `candles.Store` implementation. Final confirmation awaits the Linux reference-hardware run.
