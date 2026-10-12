#!/usr/bin/env bash
# Writes THIRD_PARTY_NOTICES.md to stdout: Go licences of the shipped cmd/tradeloom graph + npm production deps.
# The Go graph is the one the release links: GOOS=linux, CGO_ENABLED=1, tags desktop,production,webkit2_41,noorders
# (Ruling 47). go-licenses only lists packages (no cgo compile), so this also runs on macOS.
# Requires go-licenses v1.6.0 on PATH, node, and `npm ci` done in frontend/.
set -euo pipefail
export GOTOOLCHAIN=local
cd "$(dirname "$0")/.."
LICENSE_TAGS="${LICENSE_TAGS:-desktop,production,webkit2_41,noorders}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 GOFLAGS="-tags=${LICENSE_TAGS}" go-licenses report ./cmd/tradeloom \
  --ignore tradeloom --ignore modernc.org/mathutil --template build/notices.tpl 2>/dev/null
# go-licenses cannot classify modernc.org/mathutil's LICENSE (BSD-3-Clause, checked by hand); append it verbatim.
dir="$(go list -m -f '{{.Dir}}' modernc.org/mathutil)"
ver="$(go list -m -f '{{.Version}}' modernc.org/mathutil)"
printf '\n## modernc.org/mathutil %s — BSD-3-Clause\n\n```\n' "$ver"
cat "$dir/LICENSE"
printf '```\n\n# npm production dependencies (frontend/)\n'
# Full licence text of every npm production dependency, sorted by package name then version (deterministic,
# no absolute paths).
(cd frontend && npx --yes license-checker@25.0.1 --production --excludePrivatePackages --json) | node -e '
const fs = require("fs");
const deps = JSON.parse(fs.readFileSync(0, "utf8"));
const split = (k) => { const i = k.lastIndexOf("@"); return [k.slice(0, i), k.slice(i + 1)]; };
const cmp = (a, b) => (a < b ? -1 : a > b ? 1 : 0);
const keys = Object.keys(deps).sort((a, b) => {
  const [na, va] = split(a), [nb, vb] = split(b);
  return cmp(na, nb) || cmp(va, vb);
});
let out = "";
for (const k of keys) {
  const [name, version] = split(k);
  const d = deps[k];
  const lic = Array.isArray(d.licenses) ? d.licenses.join(" OR ") : d.licenses;
  let text = "(the package ships no licence file)";
  if (d.licenseFile) {
    text = fs.readFileSync(d.licenseFile, "utf8").replace(/\r\n/g, "\n").replace(/\s+$/, "");
  }
  out += `\n## ${name} ${version} — ${lic}\n\n\`\`\`\n${text}\n\`\`\`\n`;
}
process.stdout.write(out);
'
