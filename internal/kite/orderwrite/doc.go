// Package orderwrite is the only package allowed to call Kite order-write endpoints
// (PlaceOrder, ModifyOrder, CancelOrder, ExitOrder, ConvertPosition, PlaceGTT, ModifyGTT,
// DeleteGTT) — ARCHITECTURE §2.5, ROADMAP §6 gate G-0.
//
// Files:
//   - iface.go        (no build tag) the Writer interface and ErrOrdersNotCompiled.
//   - writer_stub.go  //go:build !orders || noorders — every method returns ErrOrdersNotCompiled.
//   - writer.go       //go:build orders && !noorders — the real implementation; added in P4.
//
// P0–P3 builds pass -tags webkit2_41,noorders. Building with neither tag, or with both, yields the
// stub. CI enforces the gate three ways: internal/archtest (AST: order-write calls on
// *kiteconnect.Client only in tagged files of this package; only internal/orders may import this
// package), `go tool nm` on the noorders binary (scripts/g0-nm-check.sh), and the mock-Kite
// recording suite (P0-3).
package orderwrite
