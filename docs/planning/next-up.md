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

**Status:** Shipped v0.8.0. Implementation at `internal/registry/presets.go` + `--preset` / `--list-presets` flags in `cmd/csaw/root.go`. `consulting` preset deferred per scope cut.

---

### ~~3a. Source-add shorthand parser~~ ✅ Shipped (v0.8.1)

**What:** pnpm-style host shorthand for `csaw source add`. `csaw source add team gh:co/team-source#main` works alongside the existing long form `csaw source add team https://... --branch main`. Host prefixes: `gh:`, `gl:`, `bb:`. Under the hood, canonical `(url, refKind, refValue)` tuple regardless of input syntax.

**Why split from #3b:** Bounded engineering scope (parser + tests + a couple CLI integration points). Ships independently and is the higher-leverage half.

**Success:**
- Shorthand parser exists in `internal/sources/` with full test coverage (gh/gl/bb prefixes, ref kinds, errors on garbage).
- `csaw source add gh:org/repo#tag` works at the CLI.
- Both syntaxes round-trip through sources.yml.

**Status:** Shipped v0.8.1. Parser at `internal/sources/shorthand.go`; `Source.Ref` field added for source-level default ref tracking.

### ~~3b. `csaw://` URL scheme~~ — Dropped from Tier 1, moved to Tier 3

Cost-vs-value analysis after v0.8.1 shipped: shorthand (`gh:co/team-source#v1`) already covers the shareable-text use case at zero packaging cost. The marginal value of a *clickable* `csaw://` URL is saving 2 seconds of copy-paste. The cost is per-channel OS-level protocol registration — Info.plist+CFBundleURLTypes on macOS (csaw is a CLI, not a `.app` — non-trivial), `.desktop` files on Linux (deb/rpm only; Homebrew can't), HKCR registry entries on Windows (Scoop can). Plus brittle cross-platform testing.

Bad ROI for a CLI tool. See Tier 3.

---

### ~~4a. Edit-while-mounted UX~~ ✅ Shipped (v0.8.2)

**What:** When a user edits a mounted symlink target (e.g., `.claude/rules/security.md` which points at `team-source/rules/security.md`), today the edit silently flows to the source repo working tree. No prompt, no warning. cc-switch handles this with "backfill from live when editing active provider" — more deliberate write-back. csaw's equivalent should be `csaw status` clearly surfacing modified source-repo files with a recommended-next-step hint (`csaw push` to share, `csaw fork` to keep private).

**Why ahead of 4b:** Real product value users see immediately; bounded code work (~2 hours).

**Success:**
- `csaw status` clearly shows when a mounted symlink target has been modified in its source repo.
- Documentation explains the recommended workflow (`csaw push` or `csaw fork` based on intent).
- Tests cover the modified-target detection path.

**Status:** Shipped v0.8.2. Implementation: `sources.Manager.DirtyFiles()` + new `csaw status` output section.

### ~~4b. Cross-platform packaging audit~~ ✅ Done (docs-only)

Landscape mapped in [`packaging-audit.md`](packaging-audit.md). **Nothing added to GoReleaser** in this pass — adding channels speculatively because cc-switch ships them is feature parity, not user-driven product work. The audit documents what's cheap to add (nfpms, winget, AUR), what needs significant demand (Snap, Chocolatey, hosted apt/yum repos), what defers to community (Flatpak), and what's never (Docker, Homebrew formula). Decisions are pre-staged so the response to a real user report is "one PR adds it" rather than "let me figure out the landscape."

**What:** csaw ships via Homebrew, Scoop, PyPI today. cc-switch ships via DEB, RPM, AppImage, Arch, Flatpak in addition. Audit `.goreleaser.yaml` to identify which channels GoReleaser supports cheaply without manual ongoing maintenance. Output is a decision per channel (ship now / defer / never), captured in a planning doc.

**Why fourth:** Research/ops pass, not product work. Worth doing for cross-platform reach but doesn't ship user-visible value until decisions land.

**Success:** Decision documented per channel in `docs/planning/packaging-audit.md`. Channels added only when a real user reports install friction — not speculatively.

**Status:** Done (docs-only commit, no version tag). Audit at `docs/planning/packaging-audit.md`. No GoReleaser changes — adding channels because a competing tool ships them is feature parity, not product work.

---

## Tier 2 — known follow-ups, not committed

Items that came out of recent work but aren't queued yet. Promote to Tier 1 when prioritized.

- **Mount-output "N experimental files hidden" line** — `FilterExperimental` already returns the count; just needs wiring into the `csaw use` output. ~30 min.
- **Honest "auto-served" deep projection** — Per [`projection-roadmap.md`](projection-roadmap.md): Cline, Continue, Factory Droid, Augment, Amp, Aider all have native dirs csaw doesn't reach. Quick wins available (CONVENTIONS.md for Aider, factory entry for skills). Prioritize on user demand.
- **`.github/copilot-instructions.md` alias** — Project AGENTS.md to this Copilot-canonical location as second symlink. Nice-to-have.
- **`.github/prompts/` for Copilot** — Single-file prompts. Either map skills folders awkwardly or add new `prompts` kind. Revisit if Copilot users ask.
- ~~**Refresh `docs/product/roadmap.md`**~~ ✅ Done (docs-only commit). Current State rewritten to reflect v0.8.3 honestly; per-version sections (v0.4.x–v0.9) pruned and replaced with directional themes; new principles (no-hidden-defaults, user-driven-over-parity) codified; v1.0 criteria marked with progress; "Not Now" extended with the desktop-GUI and parity-feature exclusions. Idea map's roadmap-treatment column updated to reflect what actually shipped vs. what was deferred and why.
- **Native package channels (nfpms / winget / AUR)** — All pre-staged in [`packaging-audit.md`](packaging-audit.md). Each is ~5 lines of GoReleaser config. Add WHEN a real user reports install friction on that platform, not before. PyPI already covers cross-platform Linux today.

## Tier 3 — deferred

Documented for visibility; not on near-term horizon.

- **Merged-config MCP projection** (Codex `config.toml`, OpenCode `opencode.json`, Copilot CLI `~/.copilot/mcp-config.json`, VS Code `settings.json`) — Design groundwork in [`mcp-merge-design.md`](mcp-merge-design.md). Real user-driven friction (csaw can't project MCP into shared-config files today), but a significant architectural shift from "symlink projector" to "config merger" that complicates the "Reversible by design" promise. Build a scoped prototype (one tool, dedicated command, project-scope, dry-run-first) when a real user reports the friction; until then, stay symlink-only and direct users to manual MCP setup for these tools.
- **`csaw://` URL scheme for one-click source add** (moved from Tier 1 #3b post v0.8.1) — Pattern designed in [`package-manager-lessons.md`](package-manager-lessons.md). Demoted because the shorthand we shipped in v0.8.1 already handles the shareable-text case for ~5% of the cost. Per-channel OS protocol registration (macOS Info.plist, Linux .desktop, Windows registry) is brittle for a CLI tool, and the marginal win over "paste shorthand, run it" is small. Revisit if (a) a real user reports the friction, or (b) csaw ever ships a desktop wrapper where protocol registration is natural.
- **Aggregate `csaw.lock` for multi-source reproducibility** — Per [`package-manager-lessons.md`](package-manager-lessons.md), cargo's model is the right shape when demand emerges. Today's per-source `csaw pin` covers the common case.
- **Subdirectory selection inside monorepo sources** — uv's `#subdirectory=` / pnpm's `&path:` pattern. Real but niche.
- **GUI / TUI for `csaw inspect`** — cc-switch's GUI adoption signals desire; csaw's CLI-first identity should stay until CLI PMF is clear.
- **Community-template marketplace for `--preset`** — Requires a trust model (registry, signing, version gates). v2 of the preset story.

---

## Working norms (process notes)

- This file gets updated as items ship, get reprioritized, or get scrapped. Don't let it rot — if a Tier 1 item sits unfinished for >2 weeks, demote or kill.
- New strategic ideas land in [`../product/roadmap.md`](../product/roadmap.md) (the idea map). New tactical items land here.
- Research that informs a decision goes in a separate `docs/planning/<topic>.md` (precedent: [`package-manager-lessons.md`](package-manager-lessons.md), [`projection-roadmap.md`](projection-roadmap.md)). Don't bury research inside next-up.
- **User-driven over parity-driven.** When studying a competing tool (cc-switch, ECC, APM, etc.), the deliverable is *understanding the landscape*, not a checklist of features to add. Add a feature when a real csaw user reports the friction it would relieve — not because another tool has it. Articulated after the v0.8.3 packaging audit, where the GoReleaser nfpms block was almost shipped because cc-switch ships DEB/RPM/APK, even though PyPI already covers cross-platform Linux install today and no user had reported friction.
