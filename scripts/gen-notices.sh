#!/usr/bin/env bash
# Writes THIRD_PARTY_NOTICES.md to stdout: Go licences of the cmd/tradeloom graph + npm production deps.
# Requires go-licenses v1.6.0 on PATH and `npm ci` done in frontend/.
set -euo pipefail
export GOTOOLCHAIN=local
cd "$(dirname "$0")/.."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go-licenses report ./cmd/tradeloom \
  --ignore tradeloom --ignore modernc.org/mathutil --template build/notices.tpl 2>/dev/null
# go-licenses cannot classify modernc.org/mathutil's LICENSE (BSD-3-Clause, checked by hand); append it verbatim.
dir="$(go list -m -f '{{.Dir}}' modernc.org/mathutil)"
ver="$(go list -m -f '{{.Version}}' modernc.org/mathutil)"
printf '\n## modernc.org/mathutil %s — BSD-3-Clause\n\n```\n' "$ver"
cat "$dir/LICENSE"
printf '```\n\n# npm production dependencies (frontend/)\n\n'
(cd frontend && npx --yes license-checker@25.0.1 --production --excludePrivatePackages --markdown)
