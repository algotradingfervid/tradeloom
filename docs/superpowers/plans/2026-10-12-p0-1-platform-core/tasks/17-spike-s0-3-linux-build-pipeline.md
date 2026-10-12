> **P0-1 Platform Core Implementation Plan — Task 17.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 17: Spike S0-3 — Linux build pipeline and install matrix

**Files:**
- Create: `docs/architecture/spikes/S0-3-linux-build.md`
- Modify (only if a decision below requires it): `build/Dockerfile.linux`, `build/nfpm.yaml`, `scripts/install-smoke.sh`, `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Task 16 artefacts and CI; Task 15 `--smoke`.
- Produces: decisions recorded for ARCHITECTURE §12.1 S0-3 — base image (22.04 vs 24.04), the final nfpm `depends` list, AppImage viability without bundled WebKit, the install matrix result per distro, the container-only WebKit environment needed for smoke runs. P0-5's exit demo reuses the `.deb`.

- [ ] **Step 1: Measure the build (Linux host with Docker)**

```bash
make docker-release VERSION=0.0.1-s03 2>&1 | tee /tmp/s03-build.log
docker run --rm -v "$PWD":/src -w /src tradeloom-build:p0 bash -c '
  ls -l build/bin/tradeloom
  objdump -T build/bin/tradeloom | grep -o "GLIBC_[0-9.]*" | sort -Vu | tail -1
  ldd build/bin/tradeloom | grep -E "gtk|webkit|javascriptcore|soup"'
```
Record: Docker image build time, `make release-in-container` time, binary size, highest GLIBC symbol version (expected ≤ GLIBC_2.35 from the 22.04 base), the GTK/WebKit libraries the binary links.

- [ ] **Step 2: Run the install matrix locally (same script as CI)**

```bash
for img in ubuntu:22.04 ubuntu:24.04 debian:12; do
  docker run --rm -v "$PWD":/src -w /src "$img" bash scripts/install-smoke.sh deb 2>&1 | tail -3
done
docker run --rm -v "$PWD":/src -w /src fedora:39 bash scripts/install-smoke.sh appimage 2>&1 | tail -3
```
Expected per image: `smoke ok: <PRETTY_NAME> (deb|appimage)`. Record each result verbatim; on failure record the first error line (missing package name, WebKit sandbox error, `xvfb-run` missing, Fedora 39 archive repos) and the fix applied.

- [ ] **Step 3: Desktop VM check (manual, Ubuntu 24.04 desktop VM)**

Copy `dist/tradeloom_0.0.1-s03_amd64_noorders.deb` to the VM; `sudo apt install ./tradeloom_0.0.1-s03_amd64_noorders.deb`; launch "Tradeloom" from the application menu. Record: icon shown in the menu, window opens with the heading "Tradeloom", `~/.local/share/tradeloom/logs/tradeloom.log` exists, `sudo apt remove tradeloom` removes `/usr/bin/tradeloom`. Repeat with the AppImage (`chmod +x`, double-click) on the same VM.

- [ ] **Step 4: Write `docs/architecture/spikes/S0-3-linux-build.md`**

Use exactly this structure, filling every "Observed" cell from Steps 1–3:

```markdown
# S0-3 — Linux build pipeline (spike result)

Date: <YYYY-MM-DD> · Commit: <git rev-parse --short HEAD> · Decides: ARCHITECTURE §12.1 S0-3, C-11, NFR-01/02

## Build
| Item | Observed |
|---|---|
| Docker image build time | |
| make release-in-container time | |
| Binary size (stripped) | |
| Highest GLIBC symbol | |
| Linked GTK/WebKit libraries | |
| G-0 nm check | |

## Install matrix (`scripts/install-smoke.sh`)
| Image | Package | Result (last line) | Fix applied |
|---|---|---|---|
| ubuntu:22.04 | .deb | | |
| ubuntu:24.04 | .deb | | |
| debian:12 | .deb | | |
| fedora:39 | AppImage | | |

## Desktop VM (Ubuntu 24.04)
| Check | .deb | AppImage |
|---|---|---|
| Menu entry + icon | | |
| Window opens | | |
| Log file created | | |
| Uninstall clean | | |

## Decisions
1. Base image: ubuntu:22.04 is kept if every row above passes (glibc floor 2.35 covers 22.04+, Debian 12 = 2.36, Fedora 39 = 2.38); otherwise ubuntu:24.04 and NFR-01's 22.04 support is re-opened with the user.
2. nfpm depends: the final list as committed in build/nfpm.yaml.
3. AppImage: viable without bundling WebKit if the fedora:39 and VM AppImage rows pass; the per-distro runtime requirement (`webkit2gtk4.1`, `gtk3`) goes into the Help/install notes (P0-4).
4. Container-only environment for smoke runs: `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` (never set in the packaged app).

## Follow-ups
- (one line per failure that was not fixed in this spike, with the owning sub-plan)
```

- [ ] **Step 5: Commit**

```bash
git add docs/architecture/spikes/S0-3-linux-build.md build scripts .github
git commit -m "docs(spike): S0-3 Linux build pipeline and install matrix results" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
