# Beta Acceptance Matrix

## Beta Narrative

The beta should present csaw as an AI workspace context activation and assurance tool.

The narrow promise is:

> Repos remain the source of truth for project-owned context. csaw activates the right external AI workspace context for a repo, composes it from approved sources, projects it into the tools people use, and audits that the wrong context is absent.

This keeps the product grounded. csaw is not yet a broad personal operating system for work modes, a SaaS control plane, a hard sandbox, or a replacement for git, `direnv`, Nix, `mise`, devcontainers, IDE settings, or task runners.

## Scope Rules

- A configured source is inventory. It must not imply active context.
- Active context is the mounted state in a project plus any source pins.
- Composition is explicit through profile activation (`csaw use`), advanced mount selection, priorities, and protected files.
- Client isolation is assurance: csaw can detect wrong local context and policy drift, but it does not sandbox an AI runtime.
- Context that belongs to exactly one project and can be safely committed should usually stay in that project repo.
- Beta docs must explain when not to use csaw as clearly as when to use it.

## Release Gates

| Gate | Definition | Beta Requirement |
|---|---|---|
| Works | The feature behaves correctly in normal and expected edge cases | Required for all public commands included in the beta story |
| Makes sense | The feature has a clear reason to exist against the repo-local objection | Required for all first-screen README and curriculum examples |
| Explains itself | Output and docs make active context, source provenance, and failure recovery obvious | Required for mount, audit, inspect, status, check, pin, and unmount |
| Protects data | Existing repo files are not overwritten, lost, or hidden without an explicit reversible path | Required for mount, adopt, force, restore, keep, unmount, fork, and promote |
| Is honest | Security and isolation language does not overclaim enforcement | Required across README, roadmap, curriculum, and policy docs |
| Is testable | Behavior has automated tests or a named manual workflow pass | Required before beta tag |

## Acceptance Status

- **Beta must-have:** should be fixed or verified before calling the product beta.
- **Beta decision:** choose whether the beta narrative requires this, then either promote it to must-have or explicitly defer it.
- **Beta caveat:** can ship if docs and command output set the right expectation.
- **Post-beta:** important, but not required for a polished beta.

## Feature Matrix

| Area | Current Capability | Beta Expectation | Acceptance Criteria | Status |
|---|---|---|---|---|
| Product narrative | README and roadmap describe multi-source AI workspace governance | The first-page story answers "why not keep this in the repo?" | README says repo-local is preferred for single-project context; csaw examples require cross-repo, private, client, tool, pin, or audit needs | Beta must-have; mostly present, needs final narrative pass |
| Source inventory versus active context | Sources can be configured without mounting all of them | Docs and output distinguish available sources from active mounted context | `source list`, `status`, `inspect`, `mount`, and README examples avoid implying all configured sources bleed into every project | Beta must-have; recently improved, verify in workflow pass |
| Source lifecycle | `source add`, `list`, `remove`, `pull`, `push`, clone/catalog behavior | Users can add personal, team, and client sources and understand priority | Source priority is visible; clone failures and missing refs are actionable; remove does not silently leave confusing active state | Beta must-have |
| Registry creation | `init` creates a usable registry; `init --adopt` can migrate project files | Adoption should be trustworthy on existing repos | Adopted project files win over starter files; pre-existing registry files are preserved; output says what moved | Beta must-have; code fix present, verify docs/output |
| Profile configuration | `csaw.yml` profiles, inheritance, kind filters, protected entries, ignore rules, `profile list/show` | Profiles are the explicit composition unit | Profile examples cover personal+team, client-only, and client+personal composition; users can discover and inspect resolved profiles before activation; invalid profile errors identify the bad field | Beta must-have |
| Mount lifecycle | `use`, `mount`, `unmount`, `restore`, `--keep`, `--force`, `--skip-conflicts` | Activating and leaving a context is safe and legible | Existing files are stashed or skipped according to flags; restore/unmount recovers state; partial unmount behaves predictably; noninteractive failures are clean | Beta must-have; noninteractive conflict UX needs review |
| Tool projection | Mounts into known tool directories for Codex, Claude Code, Cursor, OpenCode, Windsurf, and shared fallback dirs | A user can predict which tool receives which files | Tool projection docs match actual paths; mount output counts actual mounted tool dirs; unsupported tools are clearly fallback or not projected | Beta must-have |
| Artifact kinds | Instructions, rules, agents, skills, and MCP are classified and filterable | Kinds are a first-class safety and composition boundary | `--kind` selection works across mount/inspect/audit-relevant flows; examples show why MCP and agents are higher-risk | Beta must-have |
| Priority composition | Multiple sources compose with priority and protected file behavior | Winners, losers, and protected decisions are explainable | Mount output and inspect show active winner; conflicts explain source priority and protected-file reason | Beta decision; current winner visibility exists, layered loser detail is still weak |
| Client isolation | Required and blocked sources can be audited | Client A work can prove Client A context is active and Client B context is absent | `audit --strict` catches missing required client, active blocked client, wrong URL/ref/pin, and protected-file drift | Beta must-have |
| Blocked path and kind policy | Implemented on current `main` | Policies can forbid MCP, personal agents, or specific project paths | `.csaw/policy.yml` supports blocked kinds and blocked mounted paths with stable JSON findings | Beta must-have; verify in next audit pass |
| Source pins | Sources can be pinned per project to branch/tag/SHA worktrees | Pins are safe to enter and leave | Pinning a branch already checked out in the source clone works; unpin remaps active links; audit can require exact ref | Beta must-have; code fix present, verify workflow |
| Fork and promote | Files can be forked between sources and promoted back | Contribution loops are understandable and safe | Fork preserves provenance; promote refuses ambiguous destinations or explains them; docs show personal-to-team and client-safe variants | Beta must-have |
| Drift and repair | `check` detects unhealthy links and protected drift; update/repair paths exist | Users can recover from broken local state | Broken symlinks, replaced protected files, missing source checkouts, and stale pins produce actionable output | Beta must-have |
| Diff | `diff` exists for local mounted files | Diff should either be useful or clearly scoped | Healthy symlink diff semantics are explained or changed so users can compare active context to source/repo state | Beta caveat or must-fix; current behavior is weak |
| Inspect | `inspect` and markdown preview summarize planned/active context | Inspect is the provenance view | Shows active sources, profile, pins, mounted paths, and enough conflict/protection detail to trust composition | Beta decision; current summary is useful but not full provenance |
| Status and check | `status` and `check` describe mounted state and health | One command should answer "what context am I in?" well enough for beta | Status/check output includes active sources, kinds, pins, unhealthy links, and audit pointer | Beta decision; full `context status` can wait if status/check are strong |
| Audit JSON | Audit can emit JSON with finding IDs and exit semantics | CI consumers can depend on stable structure | Docs cover report fields, severity, IDs, strict behavior, and examples; tests lock representative JSON | Beta must-have; largely present |
| Policy templates | `audit --init` exists | Users can create project policy without memorizing schema | Minimal and client-oriented templates exist or docs give exact generated shape; no overwrite without force | Beta decision; client template recommended before beta |
| Error handling | Commands generally return errors | Failure modes should be direct, non-panicky, and scriptable | Missing source, malformed config, bad kind, bad profile, noninteractive conflicts, and invalid pins return actionable messages and non-zero exits | Beta must-have |
| TUI flows | Interactive mount/profile picker exists | TUI should not be the only path for any beta-critical workflow | Noninteractive commands cover all acceptance flows; TUI gets smoke-tested for common cases | Beta caveat |
| Cross-platform linking | Symlink/hardlink abstraction exists; CI covers major OSes | Mount/unmount works on macOS, Linux, and Windows | CI passes on all platforms; Windows hardlink fallback has targeted test coverage or documented manual pass | Beta must-have |
| Distribution | GitHub Releases, Homebrew, Scoop, and PyPI exist | Install paths are believable for beta testers | README install commands work; release workflow is documented; package versions match tag; upgrade path is clear | Beta must-have |
| Documentation | README, roadmap, curriculum, reference docs exist | A beta user can learn csaw without chat history | Curriculum teaches current behavior with no guesses; roadmap and acceptance matrix define next scope; docs avoid local paths and overclaims | Beta must-have |
| Security and trust | Public-repo policy and audit docs exist | Trust model is explicit | Docs distinguish local assurance from enforcement; MCP and personal-agent risks are named; examples use placeholders | Beta must-have |
| Personal work modes | Roadmap research track | Do not position beta as a broad work-mode OS | Technical modes can be described as future combinations of context, audit, ledger, pause/resume, and handoff | Post-beta |
| Team memory routing | Roadmap track | Not needed for beta beyond examples | Docs can show team rules and skills as a source type; no central registry promise | Post-beta |
| Agent package management | Git-backed sources, fork, promote, future metadata | Avoid registry claims until validation and metadata mature | `source validate` and metadata are designed before any hosted registry language | Post-beta |
| Context ledger and handoff | Roadmap track | Useful but not beta-critical | No beta promise beyond possible audit/status output | Post-beta |

## Recommended Beta Scope

Beta should include:

- install and release reliability
- source lifecycle for personal, team, and client registries
- explicit profile activation and profile-based composition
- safe mount, unmount, restore, force, keep, and skip-conflict flows
- kind-aware mounting for instructions, rules, agents, skills, and MCP
- tool projection into supported AI workspace directories
- priority/protected-file behavior with understandable provenance
- project adoption from existing AI workspace files
- client isolation through required and blocked sources
- source URL/ref/pin audit checks
- protected-file drift detection
- fork and promote contribution loops
- status/check/inspect output good enough to explain the active context
- docs and curriculum that can be followed without guessing

Recommended beta must-fix decisions before tagging:

1. Decide whether full layered provenance in `inspect` is required for beta or can be a documented v0.5.x follow-up.
2. Decide whether `diff` should be fixed before beta or explicitly documented as limited.
3. Decide whether a `client` policy template is required before beta or whether exact docs are enough.
4. Decide whether existing `status`/`check` output is sufficient, or whether `context status` is needed before using context-switching language heavily.

## Workflow Pass Plan

### Pass 1: Fresh User Path

Goal: verify a new user can install, initialize, mount, inspect, audit, and unmount without prior csaw knowledge.

Required checks:

- create a fresh project and fresh local registry
- run `init`
- mount default context
- inspect mounted files and active sources
- run `status`, `check`, and `audit`
- unmount and restore
- repeat using only docs/curriculum instructions

### Pass 2: Existing Project Adoption

Goal: verify csaw can adopt a project with existing AI workspace files without losing project context.

Required checks:

- create project-owned `AGENTS.md`, tool rules, skills, and MCP-like files
- run `init --adopt`
- verify adopted files preserve project content
- verify starter files do not overwrite project content
- verify pre-existing registry files are preserved
- mount, unmount, and restore after adoption

### Pass 3: Composition And Client Isolation

Goal: verify the product story that configured sources are inventory and active context is explicit.

Required checks:

- configure personal, team, client-acme, and client-globex sources
- mount client-acme only
- verify client-globex is not mounted
- mount an explicit acme+personal profile
- verify priority and protected behavior
- run audit for correct client, wrong client, missing client, blocked client, and strict mode
- inspect source winners and any hidden composition ambiguity

### Pass 4: Conflict And Data Safety

Goal: prove local files are not lost and users can recover from mistakes.

Required checks:

- mount over existing files with default behavior
- test `--skip-conflicts`
- test `--force`
- test `--keep`
- test restore after unmount
- test partial unmount
- remove or edit mounted links and run `check`/repair flows
- verify `.git/info/exclude` and stash state cleanup

### Pass 5: Remote Source And Pinning

Goal: verify git-backed source behavior without relying on external services.

Required checks:

- use local bare remotes for team/client sources
- add, pull, push, fork, and promote files
- pin source to branch, tag, and SHA where supported
- pin a branch that is already checked out in the source clone
- unpin while files are actively mounted
- audit required URL/ref/pin conditions

### Pass 6: Tool And Kind Projection

Goal: verify each public kind and supported tool path works.

Required checks:

- mount each kind independently
- mount all kinds together
- verify instructions, rules, agents, skills, and MCP land in expected tool directories
- verify kind filters do not accidentally mount omitted higher-risk kinds
- verify output counts actual mounted files and tool dirs

### Pass 7: Error And Malformed Input

Goal: make beta failures feel deliberate instead of accidental.

Required checks:

- malformed `csaw.yml`
- unknown source
- unknown profile
- invalid kind
- bad pin ref
- missing worktree/source checkout
- noninteractive conflict
- policy schema mistakes
- JSON output for audit failures

### Pass 8: Cross-Platform And Release

Goal: verify beta can be installed and run by real testers.

Required checks:

- CI passes on Linux, macOS, and Windows
- `go test ./...`, `go vet ./...`, and `go build ./...` pass locally before tag
- release workflow produces expected artifacts
- Homebrew, Scoop, PyPI, and GitHub Release docs match the tag
- Windows link fallback behavior is covered by CI or a named manual run

## Beta Decision Record

| Decision | Recommended Answer | Rationale |
|---|---|---|
| Should beta be framed as "Personal Operating System For Work Modes"? | No | The idea is compelling but broader than current functionality. Use it as a research track, not beta positioning. |
| Should beta be framed as a context switcher? | Carefully | "AI workspace context activation" is accurate now. Heavier context-switching language should wait for `context status/use/leave` or equivalent UX. |
| Should client isolation be the wedge? | Yes | It gives the clearest answer to "why not keep files in the repo?" because it involves privacy, provenance, pins, and absence checks across projects. |
| Do configured sources bleed across projects? | No | Configured sources are inventory. Only mounted sources/profiles are active. Docs and output should reinforce this everywhere. |
| Is audit enforcement? | No | Audit is local assurance and policy drift detection. Hard enforcement would require runtime, OS, IDE, or container controls outside current csaw. |
| Do we need more workflow passes? | Yes | Run the targeted passes above, starting with composition/client isolation and conflict/data safety because those are highest-risk for beta trust. |

## Exit Criteria

The beta is ready when:

- every beta must-have row is verified or has an issue explicitly blocking the tag
- beta-decision rows are either promoted to must-have or deferred in the roadmap
- the workflow pass plan has been run with notes captured in an exec plan, issue, or release checklist
- README, curriculum, roadmap, audit docs, and this matrix tell the same story
- release validation passes on the commit being tagged
