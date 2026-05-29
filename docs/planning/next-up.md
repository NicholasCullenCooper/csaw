# Next Up

Cross-track tactical queue. Ordered by priority — top items get done first. Strikethrough items when shipped; keep them visible until the next major release cuts.

For strategic/vision direction see [`../product/roadmap.md`](../product/roadmap.md). For topic-scoped roadmaps see [`projection-roadmap.md`](projection-roadmap.md) and [`package-manager-lessons.md`](package-manager-lessons.md).

---

## Tier 1 — current focus (post-v0.8.1)

### ~~1. "Minimal intrusion" framing in README~~ ✅ Shipped

**What:** Lead the README pitch with the cleanup story — csaw symlinks mean `csaw unmount` leaves nothing behind, and uninstalling csaw entirely doesn't break any tool that was reading mounted files. Adopted from cc-switch's [positioning](https://github.com/farion1231/cc-switch) ("even if you uninstall the app, your CLI tools will continue to work normally") but csaw's claim is stronger because symlinks are inherent, not engineered.

**Shipped as:** New `## Reversible by design` section in README, placed between the centered intro and `## Who csaw is for` so the framing lands before the problem statement and feature tour.

---

### ~~2. `csaw init --preset <name>` with built-in starters~~ ✅ Shipped (v0.8.0)

**What:** Pre-curated source shapes that scaffold a known-good registry in one command. Per [`package-manager-lessons.md`](package-manager-lessons.md): built-in templates only, no community marketplace; flag parameterization for the common knobs; uv-style discrete named shapes.

**Initial presets (v0.8.0, curated, embedded in binary):**
- `solo-engineer` — Personal source: AGENTS.md + a starter rule + one example skill
- `team-go` — Team source: protected AGENTS.md, Go-specific rules, code-review agent, commit-message skill
- `team-frontend` — Team source: TypeScript/React rules, frontend-focused code-review agent, commit-message skill

Deferred: `consulting` preset (structurally different — about policy, not source content). Revisit after a real consulting user hits the friction.

**Flags (v0.8.0 MVP):** `--preset <name>` and `--list-presets`. Other customization flags (`--description`, `--protected-files`, `--include-mcp`) deferred — users can edit the generated files. The existing `--name` flag still applies.

**Why second:** Highest product impact. Biggest friction-reduction from first-install to first-value. Real product work; needs implementation + content curation.

**Success:**
- `csaw source init --preset team-go` produces a working source that mounts cleanly into a fresh project.
- Doc updated to lead with `--preset` examples for the canonical persona (SWE in dept-of-teams).
- All four presets ship as tests verifying the scaffolded source passes `csaw audit`.

**Status:** Not started. Implementation site: `internal/registry/presets/` (new) + new flag handling in `cmd/csaw/root.go` source-init command.

---

### ~~3a. Source-add shorthand parser~~ ✅ Shipped (v0.8.1)

**What:** pnpm-style host shorthand for `csaw source add`. `csaw source add team gh:co/team-source#main` works alongside the existing long form `csaw source add team https://... --branch main`. Host prefixes: `gh:`, `gl:`, `bb:`. Under the hood, canonical `(url, refKind, refValue)` tuple regardless of input syntax.

**Why split from #3b:** Bounded engineering scope (parser + tests + a couple CLI integration points). Ships independently and is the higher-leverage half.

**Success:**
- Shorthand parser exists in `internal/sources/` with full test coverage (gh/gl/bb prefixes, ref kinds, errors on garbage).
- `csaw source add gh:org/repo#tag` works at the CLI.
- Both syntaxes round-trip through sources.yml.

**Status:** Next up.

### 3b. `csaw://` URL scheme (deferred, coordinates with #4)

**What:** `csaw://add-source/team@gh:co/team-source#main` — paste in Slack, click → csaw opens a confirmation TUI → runs `csaw source add`. Same shorthand grammar as #3a, just URL-encoded.

**Why deferred:** Needs OS-level protocol-handler registration (`.desktop` file on Linux, plist on macOS, registry on Windows) that lives in the packaging layer. Better to bundle with #4's GoReleaser packaging audit so the registration is decided per-channel (Homebrew formula, Scoop manifest, etc.) rather than retrofitted.

**Success criteria + design:** Re-define when implementation begins; the protocol-registration approach changes the URL grammar tradeoffs.

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
- **Refresh `docs/product/roadmap.md`** — Strategic doc is significantly stale: header says "current state: v0.4.0" (actual v0.8.1) and the v0.5–v0.9 sections describe planned work that has mostly shipped (v0.5 client isolation, v0.6 hooks/ignore, v0.7 Copilot, v0.8 presets+shorthand) or been replaced. Not just a version bump — needs a real rewrite of "Current State" and either pruning the per-version sections or replacing them with a 1-year horizon view. Promote to Tier 1 when Tier 1 #4 ships.

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
