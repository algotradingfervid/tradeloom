#!/usr/bin/env bash
# G-0 check 2 (ARCHITECTURE §2.5): the noorders binary must not link any Kite order-write method.
# Usage: scripts/g0-nm-check.sh <unstripped binary>   (wails build strips with -s -w, so CI builds a
# separate unstripped binary with plain `go build` for this check).
set -euo pipefail
export GOTOOLCHAIN=local
bin="${1:?usage: g0-nm-check.sh <unstripped binary>}"
syms="$(go tool nm -sort name "$bin")"
if ! grep -qE ' main\.main$' <<<"$syms"; then
  echo "g0: $bin has no Go symbol table (stripped?) — refusing to pass" >&2
  exit 2
fi
if grep -E 'gokiteconnect/v4\.\(\*Client\)\.(PlaceOrder|ModifyOrder|CancelOrder|ExitOrder|ConvertPosition|PlaceGTT|ModifyGTT|DeleteGTT)$' <<<"$syms"; then
  echo "g0: order-write methods are linked into $bin" >&2
  exit 1
fi
echo "g0: ok — no order-write methods in $bin"
