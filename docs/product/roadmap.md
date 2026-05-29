# Roadmap

## Product Thesis

csaw is the git-native context control plane for AI-assisted development.

Repos remain the source of truth for project-owned context. csaw is for context that crosses repo, tool, team, client, privacy, or provenance boundaries: shared rules, client policy, personal skills, MCP configuration, reusable agents, and local assurance that the right context is active.

The core product question is: why not just keep the context in the relevant repo? The answer should stay narrow. If a file belongs to exactly one project and is safe to commit there, it should live in that repo. csaw earns its place when the context must be private, composed from multiple owners, reused across repos, switched by engagement, projected into multiple AI tools, pinned independently of app code, or audited as active local state.

## Current State (v0.8.3)

csaw today is a working tool for the canonical persona — a software engineer in a department of teams in a company with engineering standards. The four-tier source stack (company/department/team/personal) composes via priority and protection; mounted files are symlinks to git-tracked source files; nothing csaw produces survives `csaw unmount`.

### What's in code today

**Mount + composition:** Multi-source mounting with priority-based composition, protected files (SHA-256 verified at check/audit time), per-project source pinning (and source-level default `Ref` set via shorthand). Fork and promote workflows. `csaw inspect`, `check`, `status`, `diff`. `csaw status` surfaces uncommitted edits in source checkouts so the silent-edit-via-symlink problem is visible.

**Artifact kinds (7):** instructions, rules, agents, skills, MCP, hooks, ignore. Settings and memory are deliberately *not* projected (privacy / out of scope; documented in [`projection-roadmap.md`](../planning/projection-roadmap.md)).

**Tool projection (7 in-code):** Claude Code, Cursor, Codex, OpenCode, GitHub Copilot (with mandatory-suffix rewriting and CommitToGit revert per the no-hidden-defaults principle), Antigravity, Goose. JSON↔code consistency test guards against drift. Codegen renders `tool-projection.md` from JSON; CI enforces.

**Source-add ergonomics:** Long-form URLs (`csaw source add team https://...`), local paths, and pnpm-style host shorthand (`csaw source add team gh:org/repo#v1.2.0`). `csaw init --preset {solo-engineer,team-go,team-frontend}` scaffolds curated starters.

**Conventions:** `experimental/` as a built-in convention (any path segment hides files by default; `--include-experimental` to mount). `.csawignore` for custom hide patterns. Project-level policy via `.csaw/policy.yml` with required sources/kinds, blocked sources/kinds/paths, strict mode, JSON output. `csaw audit --init` scaffolds.

**Distribution:** GitHub Releases (tar.gz/zip), Homebrew cask, Scoop, PyPI. Per the [packaging audit](../planning/packaging-audit.md), additional Linux/Windows channels are *pre-staged but not added* — they wait for real user-reported install friction.

### What shipped vs. what was planned

The detailed per-version planning from the original v0.4.0 roadmap didn't survive contact with reality. The themes that actually emerged through v0.5–v0.8:

- **Tool surface honesty** dominated v0.5–v0.7: adding tools, removing sunset ones (Gemini CLI), trimming speculative entries, building codegen for the projection map, catching and reverting feature parity (Copilot CommitToGit), and being honest about "auto-served via AGENTS.md" only covers ~20% of each tool's surface.
- **Convention promotion** (v0.6–v0.7.2): hooks and ignore became first-class kinds; `experimental/` became a built-in convention.
- **Source ergonomics** (v0.8.x): `--preset`, shorthand, edit-while-mounted visibility — friction reducers driven by real first-install and shareable-link use cases.

Things from the original v0.5–v0.9 plan that did *not* ship and aren't immediate priorities: the "context" vocabulary refactor (`csaw context status/use/leave`), `csaw enter` project onboarding summary, pause/resume/handoff, a context ledger, source metadata/validation, scanner protocol, npm distribution. These weren't wrong ideas — they weren't user-demanded yet.

Granular v1.0 readiness (per-feature matrix + workflow passes) tracked in [`v1.0-readiness.md`](v1.0-readiness.md), which is the operational companion to the v1.0 Criteria section below.

## Tactical queue

Current near-term work lives in [`../planning/next-up.md`](../planning/next-up.md) as a three-tier prioritized queue (current focus / known follow-ups / explicitly deferred). This roadmap stays directional; the tactical queue stays operational. Don't duplicate them — when promoting something from queue to roadmap, write it as a theme, not a version slot.

## Themes (12-month directional)

Not version-pinned. Each theme is a question csaw might answer better, with current status. Promotion to active work happens when a real user surfaces friction — not by date or by competitor parity.

### Tool surface coverage (active)

The status: csaw projects to 7 tools in-code today. AGENTS.md serves another ~20 indirectly. But each of those ~20 has native dirs (`.clinerules/`, `.continue/rules/`, `.factory/skills/`, `.augment/rules/`, etc.) csaw doesn't reach, captured honestly in [`projection-roadmap.md`](../planning/projection-roadmap.md) with quick-win candidates ranked by deep-projection value. Adds happen on user demand, not on competitor parity.

### Mount UX polish (active)

The pattern so far: when a real friction surfaces (silent edits via symlinks; first-install boilerplate; share-friendly source-add), csaw absorbs it into the existing model rather than building a new subsystem. Future moves: surface "N experimental files hidden" at mount time (FilterExperimental already returns the count), explainability ("why was this file mounted? from which source? what lost?"), better collision UX. All Tier 2 in `next-up.md`.

### Context vocabulary (deferred — wait for user demand)

Original v0.6 plan was a vocabulary refactor: `csaw context status/use/leave` replacing source/profile language. Hasn't happened. The current vocabulary (`csaw use <source>/<profile>`) works; no user has reported confusion that would justify the breaking-change cost. Revisit if a real user trips over it.

### Continuity (deferred — wait for user demand)

Pause/resume/handoff/context-ledger from the original v0.7 plan haven't shipped. The use cases (contractors switching engagements, async handoffs) are real but no current csaw user has hit the friction. Wait for the request.

### Risk surface (deferred — wait for user demand)

The original v0.9 "scanner protocol" and MCP risk reporting are interesting research directions, but `csaw audit`'s blocked-sources/kinds/paths already covers the highest-risk client-isolation gaps. External scanner integration is the right shape *if* a team builds one and asks.

### Source ecosystem (longer horizon)

If csaw grows past one-source-per-org usage, source discovery/validation/metadata become real questions. Today's shorthand + presets reduce add-friction; the next layer (source quality gates, metadata, agent bundle conventions) requires either (a) a real csaw user community publishing sources, or (b) an explicit decision to seed one. Not active.

## Idea Map

The original brainstorm of product directions. Updated treatment column to reflect what's shipped vs. deferred vs. dropped.

| Idea | Treatment | Product question it answers |
|---|---|---|
| Multi-source mount/composition/governance | **Core product (shipped through v0.8.x)** | How do I compose AI workspace files from personal, team, client, community sources without committing them into every repo? |
| AI Context Switcher (rename to "context") | **Deferred — current vocab works** | How do I see and change the active AI context across tools? |
| Client Isolation Workbench | **Largely shipped via audit + protected files + blocked policies** | How do I prove Client A context is active and Client B context is absent before I work? |
| Context Firewall | **Local assurance shipped; hard enforcement remains research** | How do I detect forbidden sources/kinds/paths/MCP without sandboxing? |
| Developer Mode Switcher | **Out of scope (use direnv/mise/devcontainers)** | How much of dev environment should csaw coordinate? |
| Team Memory Router | **Partially shipped via presets + protected files; full vision deferred** | How do staff engineers route durable rules/decisions/skills into many repos? |
| Agent Package Manager | **Deliberately not chasing — APM is a different product** | How do agent/skill bundles get installed/pinned/validated/promoted? |
| Context Ledger | **Deferred — wait for user demand** | What was active when this work happened? |
| Work Handoff Tool | **Deferred — wait for user demand** | How do I pause/resume/handoff work without polluting the repo? |
| AI Project Onboarding (`csaw enter`) | **Deferred — wait for user demand** | How do I learn an unfamiliar repo quickly? |
| Personal OS for Work Modes | **Research only; never the product** | Can technical work modes precede broad productivity automation? |

## Roadmap Principles

- Prefer repo-local files when context belongs to one repo.
- Build csaw where context must be composed, switched, kept private, projected across tools, or audited.
- Treat client isolation as the sharpest near-term wedge because it makes the repo-local objection concrete.
- Keep security language precise: csaw provides local assurance and detection, not hard endpoint enforcement.
- Make provenance obvious before adding more automation.
- Prefer stable files and JSON schemas over hidden state.
- Keep every new public behavior documented and tested.
- **No hidden defaults.** Conventions are named, documented, and surfaced at the point of use; flags do what their name says; tools opt in to behavior changes.
- **User-driven over parity-driven.** When studying a competing tool (cc-switch, APM, ECC, etc.), the deliverable is *understanding the landscape*, not a feature checklist. Add a feature when a real user reports the friction it would relieve — not because a competitor ships it.

## v1.0 Criteria

`v1.0` should mean csaw is boringly reliable for real team/client use:

- ✅ stable `csaw.yml` profile behavior (no breaking changes since v0.5)
- ✅ stable `.csaw/policy.yml` schema (no breaking changes since v0.5)
- ✅ stable audit JSON schema and finding IDs (documented since v0.4.x)
- ⏳ cross-platform mount/unmount/restore confidence on Linux, macOS, and Windows (mac+linux solid; windows less battle-tested in the wild)
- ⏳ clear context/provenance UX (today's inspect is good; layered provenance — "which candidates lost and why" — is the open gap)
- ✅ release channels working consistently (Homebrew/Scoop/PyPI green every release)
- ⏳ no known data-loss bugs in stash/restore or unmount flows (no known bugs; needs more in-the-wild testing)
- ✅ docs explain when to use repo-local context instead of csaw (`README.md`, `curriculum.md`)

The honest gating items for v1.0: Windows confidence (real users on Windows, not just CI passing), layered provenance in inspect, and broader in-the-wild stash/unmount validation.

## Research Tracks

Intentionally not the next implementation steps — captured so they don't get re-derived if someone asks.

### Personal Operating System For Work Modes

Compelling vision: `client-acme`, `deep-work`, `incident-response`, `writing` modes that switch tools, browser profiles, AI context, notes, tasks, and reminders. csaw should not chase that directly. The credible path is to win at AI-assisted technical work modes first — `client-acme` (required source active, others blocked, exact pin verified) is already half-shipped via audit. If technical modes prove valuable using just source composition + audit + status, broader productivity automation can be reconsidered. Until then, generic app/browser/calendar switching is integration territory, not csaw's surface.

### Context Firewall (hard enforcement)

Hard prevention would require shell wrappers, IDE hooks, agent runtime integration, OS users, containers, or endpoint control. csaw should keep building local assurance and policy drift detection before claiming enforcement. The audit JSON is already a contract external enforcement systems can consume.

### Agent Package Manager

Agent/skill distribution needs trust, metadata, provenance, and network effects. csaw can grow toward this through git-backed source install and validation (presets + shorthand are early steps) before attempting a registry ecosystem. Per the [packaging audit](../planning/packaging-audit.md) and [package-manager lessons](../planning/package-manager-lessons.md), built-in templates + git-deps are the right pattern; community marketplace requires a trust model and is a real product decision worth deferring.

### Developer Mode Switcher

Useful only where development environment state intersects AI context. csaw should not compete with `direnv`, `mise`, Nix, devcontainers, shell profiles, or task runners. The right boundary is to read or reference those systems during onboarding, audit, and handoff — while keeping csaw responsible for AI workspace context and provenance.

## Product Assumptions To Revisit

- Client isolation is the best near-term wedge because it turns the abstract "why not just use git?" objection into a concrete user risk.
- "Context" should mean AI workspace context first, not the whole developer environment.
- Git-backed sources are enough for installation, sharing, and provenance until source metadata and validation show the limits.
- Local audit and drift detection should prove useful before csaw claims stronger enforcement.
- Personal work modes should graduate only if technical modes create value without broad OS/browser/calendar automation.
- The 4-tier source stack (company/department/team/personal) is the canonical structure; other shapes (horizontal client/community sources) layer in by responsibility, not by tier position.

## Not Now

- SaaS control plane
- Central hosted registry
- Hard sandboxing
- Custom prompt-injection scanner built into core
- Generic dev environment management that competes with `direnv`, `mise`, Nix, or devcontainers
- Broad consumer productivity mode switching
- Desktop GUI wrapper (CLI-first identity until clear CLI PMF; cc-switch's 81k stars are for a different problem)
- Feature parity with adjacent tools where csaw users haven't reported friction
