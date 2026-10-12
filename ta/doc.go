// Package ta is a pure-Go port of TA-Lib v0.8.1 (BSD-3-Clause, Copyright (c)
// 1999-2026 Mario Fortier): technical-analysis indicators and the 61
// candlestick-pattern recognisers.
//
// Every function exists in two forms:
//
//   - A public, slice-in/slice-out form, e.g. SMA(in, period). Outputs have the
//     same length as the inputs and are index-aligned with them. Bars inside the
//     lookback window (where TA-Lib produces no value) are NaN for float
//     outputs and 0 for integer outputs. Invalid parameters return an error.
//   - An unexported range form, e.g. smaRange(startIdx, endIdx, in, period,
//     out), that mirrors the TA-Lib C call exactly (outBegIdx/outNBElement
//     semantics). Indicators that call other indicators use the range form so
//     the arithmetic matches TA-Lib bit for bit.
//
// Each function also has a Lookback function, e.g. SMALookback(period), which
// returns the number of leading bars that produce no output.
//
// Package-level settings (unstable periods, candle settings) mirror TA-Lib's
// globals. They are safe for concurrent reads; change them at start-up, not
// while indicators are being computed.
//
// The port is verified against the C library: see CONVENTIONS.md.
package ta
