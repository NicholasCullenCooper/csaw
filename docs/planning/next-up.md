# Next Up

Cross-track tactical queue. Ordered by priority — top items get done first. Strikethrough items when shipped; keep them visible until the next major release cuts.

For strategic/vision direction see [`../product/roadmap.md`](../product/roadmap.md). For topic-scoped roadmaps see [`projection-roadmap.md`](projection-roadmap.md) and [`package-manager-lessons.md`](package-manager-lessons.md).

---

## Tier 1 — current focus (post-v0.7.2)

### 1. "Minimal intrusion" framing in README

**What:** Lead the README pitch with the cleanup story — csaw symlinks mean `csaw unmount` leaves nothing behind, and uninstalling csaw entirely doesn't break any tool that was reading mounted files. Adopted from cc-switch's [positioning](https://github.com/farion1231/cc-switch) ("even if you uninstall the app, your CLI tools will continue to work normally") but csaw's claim is stronger because symlinks are inherent, not engineered.

**Why first:** Smallest scope (pure copywriting, ~20 min), sets the framing all other features inherit from, no design decisions to debate.

**Success:** README opening paragraphs mention reversibility/cleanup. A new reader scanning the top of the README understands within 30 seconds that csaw isn't a lock-in tool.

**Status:** Not started.

---

### 2. `csaw source init --preset <name>` with built-in starters

**What:** Pre-curated source shapes that scaffold a known-good registry in one command. Per [`package-manager-lessons.md`](package-manager-lessons.md): built-in templates only, no community marketplace; flag parameterization for the common knobs; uv-style discrete named shapes.

**Initial presets (curated, embedded in binary):**
- `solo-engineer` — Personal source: AGENTS.md + a few starter rules + one example skill
- `team-go` — Team source: protected AGENTS.md, Go-specific rules, code-review agent, commit-message skill
- `team-frontend` — Team source: TypeScript/React rules, perf-review agent, PR-template skill
- `consulting` — Horizontal/client source shape: client-isolation policy starter, blocked_paths examples

**Flags:** `--name`, `--description`, `--protected-files` (comma list), `--include-mcp`. `csaw source init --list-presets` shows what's available.

**Why second:** Highest product impact. Biggest friction-reduction from first-install to first-value. Real product work; needs implementation + content curation.

**Success:**
- `csaw source init --preset team-go` produces a working source that mounts cleanly into a fresh project.
- Doc updated to lead with `--preset` examples for the canonical persona (SWE in dept-of-teams).
- All four presets ship as tests verifying the scaffolded source passes `csaw audit`.

**Status:** Not started. Implementation site: `internal/registry/presets/` (new) + new flag handling in `cmd/csaw/root.go` source-init command.

---

### 3. Source-add shorthand + `csaw://` URL scheme

**What:** Two surfaces for low-friction source addition:
1. **CLI shorthand:** `csaw source add team gh:co/team-source#main` (pnpm-style host prefix), supplementing the existing long form `csaw source add team https://... --branch main`.
2. **Deep-link URL:** `csaw://add-source/team@gh:co/team-source#main` — paste in Slack, click → csaw opens a confirmation TUI → runs `csaw source add`.

Under the hood: canonical `(url, refKind, refValue)` tuple regardless of input syntax. Host prefixes supported: `gh:`, `gl:`, `bb:`. Skip pnpm's `#semver:^X` — too clever for csaw.

**Why third:** Adoption multiplier (low-friction sharing accelerates org rollout) but requires real design + implementation work. Per [`package-manager-lessons.md`](package-manager-lessons.md), the shorthand parser is straightforward; the URL scheme has an OS-protocol-registration concern that needs packaging coordination.

**Success:**
- Shorthand parser exists in `internal/sources/` with full test coverage (gh/gl/bb prefixes, ref kinds, errors on garbage).
- `csaw source add gh:org/repo#tag` works at the CLI.
- `csaw://add-source/...` URL parses correctly; OS-level handler registration documented per platform (Homebrew formula, Scoop manifest, etc.).
- Confirmation TUI matches existing `csaw use` interactive pattern.

**Status:** Not started. Coordinates with the packaging audit (#4) for the protocol-registration step.

---

### 4. Edit-while-mounted UX + cross-platform packaging audit

**Combined two related concerns:**

**4a. Edit-while-mounted UX:** Today, when a user edits a mounted symlink target (e.g., `.claude/rules/security.md` which points at `team-source/rules/security.md`), the edit silently flows to the source repo working tree. No prompt, no warning. cc-switch handles this with "backfill from live when editing active provider" — more deliberate write-back. csaw's equivalent should be a `csaw status` improvement that detects modified source-repo files and surfaces them (probably already partially done — verify).

**4b. Packaging audit:** csaw ships via Homebrew, Scoop, PyPI today. cc-switch ships via DEB, RPM, AppImage, Arch, Flatpak in addition. Audit which channels GoReleaser can add cheaply for csaw without manual ongoing maintenance.

**Why fourth:** Lower marginal impact per hour than #1–#3. 4a is an existing-feature polish; 4b is ops work, not product. Worth doing but not blocking.

**Success:**
- 4a: `csaw status` clearly shows when a mounted symlink target has been modified in its source repo. Documentation explains the recommended workflow (`csaw push` or `csaw fork` based on intent).
- 4b: Decision documented per channel: ship via GoReleaser, defer, or never. New channels added live in subsequent release; gaps documented in roadmap.

**Status:** Not started.

---

## Tier 2 — known follow-ups, not committed

Items that came out of recent work but aren't queued yet. Promote to Tier 1 when prioritized.

- **Mount-output "N experimental files hidden" line** — `FilterExperimental` already returns the count; just needs wiring into the `csaw use` output. ~30 min.
- **Honest "auto-served" deep projection** — Per [`projection-roadmap.md`](projection-roadmap.md): Cline, Continue, Factory Droid, Augment, Amp, Aider all have native dirs csaw doesn't reach. Quick wins available (CONVENTIONS.md for Aider, factory entry for skills). Prioritize on user demand.
- **`.github/copilot-instructions.md` alias** — Project AGENTS.md to this Copilot-canonical location as second symlink. Nice-to-have.
- **`.github/prompts/` for Copilot** — Single-file prompts. Either map skills folders awkwardly or add new `prompts` kind. Revisit if Copilot users ask.
- **Refresh `docs/product/roadmap.md`** — Strategic doc is stale (says v0.4.0 current state, actual is v0.7.2). One-pass refresh after Tier 1 work lands.

## Tier 3 — deferred

Documented for visibility; not on near-term horizon.

- **Aggregate `csaw.lock` for multi-source reproducibility** — Per [`package-manager-lessons.md`](package-manager-lessons.md), cargo's model is the right shape when demand emerges. Today's per-source `csaw pin` covers the common case.
- **Subdirectory selection inside monorepo sources** — uv's `#subdirectory=` / pnpm's `&path:` pattern. Real but niche.
- **GUI / TUI for `csaw inspect`** — cc-switch's GUI adoption signals desire; csaw's CLI-first identity should stay until CLI PMF is clear.
- **Community-template marketplace for `--preset`** — Requires a trust model (registry, signing, version gates). v2 of the preset story.

---

## Working norms (process notes)

- This file gets updated as items ship, get reprioritized, or get scrapped. Don't let it rot — if a Tier 1 item sits unfinished for >2 weeks, demote or kill.
- New strategic ideas land in [`../product/roadmap.md`](../product/roadmap.md) (the idea map). New tactical items land here.
- Research that informs a decision goes in a separate `docs/planning/<topic>.md` (precedent: [`package-manager-lessons.md`](package-manager-lessons.md), [`projection-roadmap.md`](projection-roadmap.md)). Don't bury research inside next-up.
