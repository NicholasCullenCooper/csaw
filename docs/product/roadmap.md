# Roadmap

## Product Thesis

csaw is the git-native context control plane for AI-assisted development.

Repos remain the source of truth for project-owned context. csaw is for context that crosses repo, tool, team, client, privacy, or provenance boundaries: shared rules, client policy, personal skills, MCP configuration, reusable agents, and local assurance that the right context is active.

The core product question is: why not just keep the context in the relevant repo? The answer should stay narrow. If a file belongs to exactly one project and is safe to commit there, it should live in that repo. csaw earns its place when the context must be private, composed from multiple owners, reused across repos, switched by engagement, projected into multiple AI tools, pinned independently of app code, or audited as active local state.

## Current State

`v0.4.0` establishes the core governance direction:

- multi-source mounting from personal, team, client, and community sources
- priority-based composition and protected files
- first-class artifact kinds: instructions, rules, agents, skills, and MCP
- per-tool projection for Claude Code, Cursor, Codex, OpenCode, Windsurf, and shared fallback directories
- per-project source pinning
- fork and promote workflows
- `csaw inspect`, `check`, `status`, and `diff`
- `csaw audit` with `.csaw/policy.yml`, required sources, blocked sources, required kinds, strict mode, and JSON output
- distribution through GitHub Releases, Homebrew, Scoop, and PyPI

## Idea Map

The brainstormed product directions all fit somewhere, but they should not all become equal product bets.

| Idea | Roadmap treatment | Product question it answers |
|---|---|---|
| Current csaw mount/composition/governance | Core product | How do I compose AI workspace files from personal, team, client, and community sources without committing them into every repo? |
| AI Context Switcher | Core vocabulary in v0.6 | How do I see and change the active AI context across tools without thinking in source/profile internals? |
| Client Isolation Workbench | Primary wedge for v0.5-v0.6 | How do I prove Client A context is active and Client B context is absent before I work? |
| Context Firewall | Assurance now, enforcement research later | How do I detect forbidden sources, kinds, paths, or MCP config without pretending local files are a sandbox? |
| Developer Mode Switcher | Adjacent integration, not core | How much of `direnv`, `mise`, devcontainers, shells, browsers, and task runners should csaw coordinate? |
| Team Memory Router | v0.8 product track | How do staff engineers and platform teams route durable rules, decisions, review checklists, and skills into many repos? |
| Agent Package Manager | v0.8 plus research | How do reusable agents, skills, rules, and MCP bundles get installed, pinned, validated, forked, and promoted? |
| Context Ledger | v0.7 infrastructure | What was active when this work happened, and what changed since then? |
| Work Handoff Tool | v0.7 workflow | How do I pause, resume, or hand work to someone else without polluting the repo with local state? |
| AI Project Onboarding Tool | v0.6-v0.7 UX | How do I enter an unfamiliar repo and quickly learn the stack, commands, policy, and active AI context? |
| Personal Operating System For Work Modes | Research track | Can technical work modes grow from context switching, audit, ledger, pause/resume, and handoff before broad productivity automation? |

## Roadmap Principles

- Prefer repo-local files when context belongs to one repo.
- Build csaw where context must be composed, switched, kept private, projected across tools, or audited.
- Treat client isolation as the sharpest near-term wedge because it makes the repo-local objection concrete.
- Keep security language precise: csaw provides local assurance and detection, not hard endpoint enforcement.
- Make provenance obvious before adding more automation.
- Prefer stable files and JSON schemas over hidden state.
- Keep every new public behavior documented and tested.

## Immediate Follow-Up: v0.4.x

Goal: make the new audit surface practical and trustworthy without expanding the product too far.

| Work | Why | Acceptance |
|---|---|---|
| Document audit JSON schema | Teams need stable CI/reporting integration | `docs/reference/audit-json.md` defines report fields, severities, finding IDs, and exit behavior |
| Add `csaw audit --init` | Users need an easy way to adopt project policy | Command writes a minimal `.csaw/policy.yml` or refuses to overwrite without `--force` |
| Verify required source URL/ref | Policy currently checks active source names only | `required_sources` with `url` and `ref` emits findings when active context does not match |
| Add policy examples | Make team/client use cases concrete | README and cheat sheet include client isolation and team policy examples |
| Clean up local toolchain guidance | Avoid ad hoc local setup files | Decide whether to commit a pinned `mise.toml` or keep it out of repo |

## v0.5: Client Isolation And Context Assurance

Goal: make `csaw audit --strict` the reliable answer to “am I in the right AI workspace, and is the wrong client/team context absent?”

| Work | Why | Acceptance |
|---|---|---|
| Client isolation policy template | The strongest use case is switching between sensitive engagements | `csaw audit --init --template client` creates required/blocked source, kind, and MCP policy examples |
| Protected-file hash verification | Protected files are currently advisory within csaw | Mount records hashes for protected entries; audit/check detects replacement or content drift |
| Required pins | Clients may require an exact source ref | Policy can require a source branch/tag/SHA; audit reports mismatch |
| Blocked path/kind checks | Some projects may forbid MCP or personal agents entirely | Policy supports blocked kinds and blocked project paths |
| Layered provenance in inspect | Users need to know not just what won, but why | Inspect shows winning source, losing candidates, priority/protection reason, and pin state |
| Better collision UX | Fail-fast conflicts are correct but rough | Conflicts explain candidate sources, priorities, protection, and suggested fixes |

## v0.6: Context UX And Onboarding

Goal: make the user-facing vocabulary match the product: contexts and project entry, not just sources and profiles.

| Work | Why | Acceptance |
|---|---|---|
| `csaw context status` | Users need one command for “what mode am I in?” | Shows active sources, profile, policy, pins, audit summary, mounted kinds |
| `csaw context use <name>` | Profiles are implementation detail | Context command resolves to source/profile policy without breaking existing mount behavior |
| `csaw context leave` | Context switching needs a clean exit | Equivalent to safe unmount plus policy-aware summary |
| Context aliases | Client/team names should be ergonomic | Config supports named aliases that point to source/profile combinations |
| Explain mode | Make magic debuggable | `--explain` shows why each file was mounted or skipped |
| `csaw enter` project summary | AI project onboarding is a natural extension of context status | Detects repo root, active branch, known docs, likely build/test commands, mounted kinds, and audit state |
| Onboarding handoff notes | New contributors need durable orientation without committed local state | `csaw enter --format markdown/json` emits a sanitized project entry summary |

## v0.7: Continuity And Handoff

Goal: preserve enough state that users can pause and resume work without polluting repos.

| Work | Why | Acceptance |
|---|---|---|
| `csaw pause` | Context switching is expensive | Captures active mount state, git branch, dirty summary, pins, audit result, and notes |
| `csaw resume` | Returning should be explicit and verifiable | Restores/remounts the recorded context and reports drift since pause |
| Local context ledger | Audit and handoff need history | Append-only local records of context switches and audit summaries |
| Handoff bundle | Contractors and teams need transfer artifacts | Generates a sanitized markdown/json handoff with active context and next steps |

## v0.8: Team Memory And Agent Ecosystem

Goal: make team memory and reusable AI workspace artifacts easier to distribute without inventing a central registry too early.

| Work | Why | Acceptance |
|---|---|---|
| `csaw install <git-url>` alias | Installing sources should feel natural | Adds source, pulls, lists profiles, and suggests mount/context commands |
| Source metadata | Users need to evaluate trust and purpose | Optional metadata file supports description, owner, license, supported tools, and kinds |
| Source validation | Shared sources need quality gates | `csaw source validate` checks profile syntax, paths, kinds, protected entries, and metadata |
| Team memory routing examples | Teams need to understand what belongs in csaw versus repos | Docs show how to route review checklists, engineering standards, architecture notes, incident runbooks, and agent skills |
| Agent bundle conventions | Agents need package-like metadata before any registry exists | Metadata can describe agent purpose, required MCP, supported tools, trust notes, and promotion status |
| Publish workflow polish | Teams need contribution loops | Existing push/fork/promote flows gain clearer status and docs |

## v0.9: Risk Scanning And Integrations

Goal: extend audit without turning csaw into a bespoke security scanner.

| Work | Why | Acceptance |
|---|---|---|
| Scanner protocol | Content/risk checks should be delegated | External scanners can emit normalized findings consumed by `csaw audit` |
| MCP risk summary | MCP is the highest-risk context surface | Audit reports command/env/path risk from known MCP config files |
| CI examples | Teams need enforcement points | Docs include GitHub Actions examples for `csaw audit --strict --json` |
| Optional npm distribution | Meet frontend users where they are | npm wrapper installs platform-specific binaries without postinstall scripts |

## v1.0 Criteria

`v1.0` should mean csaw is boringly reliable for real team/client use:

- stable `csaw.yml` profile behavior
- stable `.csaw/policy.yml` schema
- stable audit JSON schema and finding IDs
- cross-platform mount/unmount/restore confidence on Linux, macOS, and Windows
- clear context/provenance UX
- release channels working consistently
- no known data-loss bugs in stash/restore or unmount flows
- docs explain when to use repo-local context instead of csaw

## Research Tracks

These are intentionally not the next implementation steps.

### Personal Operating System For Work Modes

The broad idea is compelling: `client-acme`, `deep-work`, `incident-response`, and `writing` modes that switch tools, browser profiles, AI context, notes, tasks, and reminders.

csaw should not chase that directly yet. The credible path is to first win at AI-assisted technical work modes:

- `client-acme`: required client source active, other client sources blocked, exact pin verified, MCP risk visible.
- `deep-work`: personal agents/skills active, notification tooling out of scope, context ledger records the session.
- `incident-response`: incident runbooks, production-safe MCP config, strict audit, pause/resume and handoff artifacts.
- `writing`: writing rules and review agents active, without trying to manage every notes app or calendar.

If these work modes become valuable using only source composition, audit, context status, pause/resume, and handoff, broader productivity automation can be reconsidered. Until then, generic app/browser/calendar switching should remain integration territory.

### Context Firewall

Hard prevention would require shell wrappers, IDE hooks, agent runtime integration, OS users, containers, or endpoint control. csaw should keep building local assurance and policy drift detection before claiming enforcement.

### Agent Package Manager

Agent/skill distribution needs trust, metadata, provenance, and network effects. csaw can grow toward this through git-backed source install and validation before attempting a registry ecosystem.

### Developer Mode Switcher

This is useful only where development environment state intersects AI context. csaw should not compete with `direnv`, `mise`, Nix, devcontainers, shell profiles, or task runners. The right boundary is to read or reference those systems during onboarding, audit, and handoff, while keeping csaw responsible for AI workspace context and provenance.

## Product Assumptions To Revisit

- Client isolation is the best near-term wedge because it turns the abstract “why not just use git?” objection into a concrete user risk.
- “Context” should mean AI workspace context first, not the whole developer environment.
- Git-backed sources are enough for installation, sharing, and provenance until source metadata and validation show the limits.
- Local audit and drift detection should prove useful before csaw claims stronger enforcement.
- Personal work modes should graduate only if technical modes create value without broad OS/browser/calendar automation.

## Not Now

- SaaS control plane
- central hosted registry
- hard sandboxing
- custom prompt-injection scanner built into core
- generic dev environment management that competes with `direnv`, `mise`, Nix, or devcontainers
- broad consumer productivity mode switching

## Next Best Issue

The highest-leverage next implementation issue is:

**Add `csaw audit --init` and document the audit JSON schema.**

This improves adoption of the just-released governance surface, keeps the scope small, and gives future CI/reporting integrations a stable contract.
