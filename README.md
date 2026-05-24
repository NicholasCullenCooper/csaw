<p align="center">
  <h1 align="center">csaw</h1>
  <p align="center">
    <strong>Multi-source AI workspace governance.</strong><br>
    Mount layered AI config — company standards, team conventions, personal preferences — into every repo, without copy-paste or repo pollution.
  </p>
  <p align="center">
    <a href="https://github.com/NicholasCullenCooper/csaw/actions"><img src="https://github.com/NicholasCullenCooper/csaw/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://github.com/NicholasCullenCooper/csaw/releases"><img src="https://img.shields.io/github/v/release/NicholasCullenCooper/csaw?include_prereleases&label=version" alt="Version"></a>
    <a href="https://pypi.org/project/csaw/"><img src="https://img.shields.io/pypi/v/csaw" alt="PyPI"></a>
  </p>
  <p align="center">
    Works with: Claude Code · Cursor · Codex · OpenCode · Windsurf · Gemini CLI
  </p>
</p>

---

## Who csaw is for

You have **more than one source of AI configuration**:

- **Engineers in companies with shared standards** — you want company standards, team conventions, and personal preferences composed into every repo, kept live, never committed.
- **Teams or staff engineers publishing AI policy** — you ship rules and skills that lower layers can't silently override, with audit catching drift.
- **Individuals composing personal config** with team or community sources.
- **Contractors and consultants juggling clients** — each engagement has its own MCP servers and conventions that must not bleed across projects.

For team or org use, csaw needs one person to **maintain** each shared source — a small but real role. Everyone else is a passive consumer.

**csaw is the wrong tool if:** you have one repo, one team, and no personal additions to layer on top. Commit your `AGENTS.md` and stop here.

Want the full learning path? See the [csaw curriculum](docs/curriculum.md).

## The problem

Multi-stakeholder AI config is a governance problem:

- **No source of truth across projects.** A team's `AGENTS.md` gets copy-pasted into every repo. Each copy drifts independently. The "real" version is whoever pushed last.
- **No way to enforce policy.** Security mandates a rule. A developer overrides it locally. Nobody notices.
- **No isolation between contexts.** Personal config in `~/.claude/` applies globally to every project — including client repos that shouldn't see it. Experimental skills dropped into a project to try them out get accidentally committed via git. No way to say "this config belongs here, not there."
- **No layering.** Team has shared rules; you want personal additions on top. Composing them per repo is manual, so nobody bothers.
- **No lineage.** Fork a team skill, customize for your style, push improvements back? Manual copy-paste, no record of what diverged.
- **Cleanup is impossible.** Tried an experimental config; now hunting through 3 tool directories and 6 files.

## How csaw works

### The model

A **source** is a git repo (or local directory) of AI config — instructions, rules, agents, skills, MCP files. A **project** mounts files from one or more sources via symlinks. The mount is reversible, live (edits to a mounted file propagate instantly), and invisible to the project's git.

```
your-registry/                         your-project/
  AGENTS.md                     ──→      AGENTS.md (symlink)
  rules/                                 .claude/rules/
    go-conventions.md           ──→        go-conventions.md (symlink)
  agents/                                .claude/agents/
    code-reviewer.md            ──→        code-reviewer.md (symlink)
  skills/                                .claude/skills/
    code-review/SKILL.md        ──→        code-review/SKILL.md (symlink)
  mcp/                                   .mcp.json (symlink)
    claude-code.json            ──→
```

Layer multiple sources with priority. Mark files **protected** so lower-priority sources can't override them. **Pin** a source to a branch or tag for one project. **Fork** a file from one source into another. **Audit** a project to confirm the right context is active and nothing forbidden is mounted.

### The full stack

Imagine you're a software engineer in a department of teams in a company with engineering standards. Your AI workspace has four layers, each owned by someone different:

```
sources
  company    [priority 100, protected]   AGENTS.md, security rules
  department [priority  80, protected]   PR-workflow skill
  team       [priority  50]              Go conventions, code-review
  personal   [priority  10]              debugging, note-capture
                                                │
                                                │  composed into
                                                ▼
  your-project/        ← live symlinks into all four sources
  your-other-project/  ← same composition, also live
  …every repo you work in
```

What each layer gives you:

- **Company** (mandatory, protected). Engineering standards that apply to every repo. Marked protected so no lower layer can silently shadow them. Updated centrally — every repo you've mounted in sees the change on the next `csaw pull`.
- **Department** (mandatory, protected). Cross-team conventions like a PR-workflow skill. Higher priority than team, also protected. Add it later without rewriting anything below it.
- **Team** (shared, not protected). Your team's rules and skills, reused across every repo your team owns. Edit once in the team source, every team repo sees it instantly.
- **Personal** (just you). Your own debugging skills, note-capture preferences, anything personal. Symlinked into every project but hidden from git — never committed, never visible to teammates, never in any repo.

Two properties emerge from stacking these that no single layer would give you on its own:

- **Always in sync.** Because mounted files are symlinks, every layer updates live across every repo that consumes it. No PR fan-out, no manual copy, no "we forgot to update repo seven."
- **Properly governed.** Because layers have explicit priority and some files are protected, the company and department mandates can't be silently overridden by a team or personal layer. `csaw audit` proves it in CI.

These four tiers form one **vertical** hierarchy — each layer nests inside the one above it (your team is in your department is in your company). They're not a fixed schema. You can also add **horizontal** sources that cut across the org chart, opted into by responsibility rather than position.

*Example:* as a staff engineer, you want every repo you touch — regardless of team — to have cost-awareness rules and cost-analysis skills. That concern isn't company-wide, isn't team-specific, isn't purely personal — it's shared with other staff engineers and managers who have similar responsibilities. Add a `staff-eng` source at whatever priority fits, and only the people who need it mount it.

Other horizontal sources you might add: **client** (for consultants — must not bleed across projects), **community** (open-source skill libraries), **incident-response** (for on-call rotations), **security-team** (for security-sensitive reviews).

### The two roles

- **Maintainer** — owns a source. Curates its files, defines its profiles, marks protected files, accepts contributions back. One per source.
- **Consumer** — adds the source (`csaw source add`), activates a profile (`csaw use`), pulls updates (`csaw pull`). Never edits the source they consume.

A single person can play both roles across different sources (you maintain `personal`, you consume `team`). For csaw to work in your org, you need exactly one maintainer per shared source — everyone else can be passive.

### The three config files

| File | Where | Purpose | Edited by |
|---|---|---|---|
| `csaw.yml` | inside a source registry | Defines that source's profiles and protected files | Source maintainer |
| `.csaw/policy.yml` | inside a project repo (committed) | Declares what context the project requires/blocks/audits | Project lead |
| `~/.csaw/config.yml` | each developer's machine | Personal: which AI tools you use, which sources you've added | Each user, locally |

When each file appears in the walkthrough below, this is what it is. They are three different files with three different lifecycles — don't conflate them.

### Why not just commit your AI config to the project?

If a context file belongs to one repo and is safe to commit there, **commit it**. csaw is for the cases that don't fit:

- **Multiple repos** sharing the same standards, without copy-paste drift between them.
- **Personal additions** layered on top of team config, without polluting the project repo for everyone else.
- **Client isolation** — keeping personal MCP servers, notes integrations, and one client's config out of another client's project.
- **Non-overridable mandates** — a security or platform team publishing rules that lower layers can't silently shadow, with audit catching drift.
- **Cross-tool projection** — write the file once, project it into `.claude/`, `.cursor/`, `.opencode/`, `.codex/` automatically.

Each section below earns its keep against this question. Mounted files are invisible to the project's git; unmount removes every symlink and restores any originals.

---

## Install

```bash
uv tool install csaw
```

<details>
<summary>Other install methods</summary>

```bash
# macOS / Linux
brew install --cask NicholasCullenCooper/tap/csaw

# Windows
scoop bucket add csaw https://github.com/NicholasCullenCooper/scoop-bucket
scoop install csaw

# pipx
pipx install csaw

# Go from source
go install github.com/NicholasCullenCooper/csaw/cmd/csaw@latest
```

> **macOS note (Homebrew):** If you see "Apple could not verify", run:
> ```bash
> xattr -d com.apple.quarantine "$(which csaw)"
> ```
> This is normal for unsigned CLI tools distributed via Homebrew casks.

</details>

---

## Starting a New Project

You have a brand new repo with no AI config. Create a personal registry:

```bash
csaw init ~/my-ai-config
```

```
✔ initialized registry "my-ai-config"

╭─────────────────────────────────────────────────────╮
│  Register as a source?                              │
│  ▸ Yes       No                                     │
╰─────────────────────────────────────────────────────╯

✔ registered source "my-ai-config" with priority 10
```

This creates a ready-to-use registry:

```
~/my-ai-config/
  csaw.yml              ← default profile
  .csawignore           ← hides skills/experimental/** by default
  AGENTS.md             ← your coding rules
  rules/                ← always-on standards
  agents/               ← subagent definitions
  skills/
    code-review/SKILL.md
    commit-message/SKILL.md
    experimental/       ← work-in-progress skills
```

Now activate it in your project:

```bash
cd ~/my-project
csaw use my-ai-config/default
```

```
╭──────────────────────────────────────────────────╮
│                                                  │
│  mounted                                         │
│                                                  │
│  my-ai-config                                    │
│   ✔ AGENTS.md                                    │
│   ✔ .claude/skills/code-review/SKILL.md          │
│   ✔ .claude/skills/commit-message/SKILL.md       │
│                                                  │
│  3 files mounted · 1 tool dirs                   │
│                                                  │
╰──────────────────────────────────────────────────╯
```

Your project now looks like this:

```
my-project/
  src/
  package.json
  AGENTS.md                              ← symlink to ~/my-ai-config/AGENTS.md
  .claude/
    skills/
      code-review/SKILL.md               ← symlink
      commit-message/SKILL.md            ← symlink
```

Open Claude Code (or Cursor, Codex, OpenCode, Gemini CLI) — it finds the files automatically. Run `git status` — nothing shows up. The files are hidden via `.git/info/exclude`.

---

## I Already Have an AGENTS.md

You have an existing project with AI config files scattered around — an AGENTS.md, maybe some skills in `.claude/skills/`. You want to pull them into a registry instead of leaving them committed.

```bash
cd ~/my-project
csaw init --adopt ~/my-ai-config
```

```
╭───────────────────────────────────────╮
│                                       │
│  adopted 3 files                      │
│                                       │
│   ✔ AGENTS.md                         │
│   ✔ skills/testing/SKILL.md           │
│   ✔ rules/go.md                       │
│                                       │
╰───────────────────────────────────────╯
```

csaw scans your project, finds AI config files, and copies them into the new registry with the correct structure. It reverses the projection — `.claude/skills/testing/SKILL.md` becomes `skills/testing/SKILL.md` in the registry, `.claude/rules/go.md` becomes `rules/go.md`, `.claude/agents/reviewer.md` becomes `agents/reviewer.md`.

Now you can delete the originals from your project, register the source, and activate it instead:

```bash
csaw source add personal ~/my-ai-config --priority 10
csaw use personal/default
```

---

## Using a Team Source

Your team keeps shared AI config in a git repo. One command to get it:

```bash
csaw source add team git@github.com:your-org/ai-config.git
```

```
✔ registered source "team"
✔ cloned team
```

csaw auto-clones the repo. Now activate a named work mode:

```bash
cd ~/my-project
csaw profile list
csaw use team/backend
```

`team/backend` means: use the `backend` profile from the configured `team`
source. Your project gets the team's AGENTS.md, skills, and rules — all
symlinked. Every repo on the team can activate the same named mode. When
someone updates the team config:

```bash
csaw pull team
# ✔ pulled team
```

Every project sees edits to already-mounted files instantly through the symlinks.
If the update added new files, rerun the same activation command to add the new
symlinks.

---

## Composing Multiple Sources

You want company standards, team conventions, and your personal preferences composed into every repo — with priority and protection in the right places.

Adding a source only makes it available. It does not activate that source in a project. The active context is whatever `csaw use` last mounted, and you can verify it with `csaw inspect` or `csaw audit`.

```bash
csaw source add company git@github.com:your-org/eng-standards.git --priority 100
csaw source add team git@github.com:your-team/ai-config.git --priority 50
csaw init ~/my-ai-config
csaw source add personal ~/my-ai-config --priority 10
```

Activate one source's profile and only that gets mounted:

```bash
csaw use team/backend
```

That activates the `backend` profile from `team`, not every configured source. To compose multiple sources into one work mode, define a named profile in your personal source's `csaw.yml`:

```yaml
work:
  description: Company standards + team backend + my personal helpers
  extends:
    - company/required
    - team/backend
  include:
    - skills/debugging/**
    - skills/note-capture/**
```

Then activate it:

```bash
csaw use personal/work
```

Because this profile lives in the `personal` source, `skills/debugging/**` resolves to `personal/skills/debugging/**`. `company/required` and `team/backend` remain source-qualified. The only string you type day-to-day is `personal/work`.

For one-off debugging, the raw pattern command still exists:

```bash
csaw mount paths 'company/**' 'team/**' 'personal/skills/**'
```

It is the advanced escape hatch, not the normal workflow.

### Consultant variant: client isolation

If you're a contractor or consultant, the same compose-and-activate pattern applies — substitute client sources for company+team, and pair activation with project policy that blocks other clients:

```bash
csaw source add client-acme git@github.com:acme/ai-config.git --priority 50
```

Define a profile in your personal source bundling the client with safe-for-client helpers:

```yaml
acme-work:
  description: Acme config plus my safe-for-client helpers
  extends:
    - client-acme/backend
  include:
    - skills/code-review/**
    - skills/debugging/**
```

In the project, declare what's required and what's forbidden:

```yaml
required_sources:
  - client-acme
blocked_sources:
  - client-globex
  - personal-experimental
blocked_kinds:
  - mcp
required_kinds:
  - instructions
```

```bash
csaw audit --strict
```

The contractor's case is the same composition pattern as the canonical — project policy is what does the isolation work.

### What if two sources provide the same file?

**Priority decides.** Higher number wins.

```bash
csaw inspect
```

```
Sources
  company  (remote, priority 100) → ~/.csaw/sources/company
  team     (remote, priority 50)  → ~/.csaw/sources/team
  personal (local,  priority 10)  → ~/my-ai-config
```

You can set priority on any source:

```bash
csaw source add personal ~/my-config --priority 10
csaw source add team git@github.com:org/config.git --priority 50    # team wins over personal on overlap
csaw source add company git@github.com:org/standards.git --priority 100  # company wins over both
```

Protected files in higher-priority sources can't be overridden by lower layers at all (see [Protected Files](#protected-files) below).

If two sources have the same priority and provide the same file, csaw errors and tells you to resolve it explicitly.

---

## Protected Files

When a source needs to enforce that certain files **cannot be overridden** — company-wide security rules, a department's mandatory PR-workflow skill, a client's required `AGENTS.md` — mark them as protected in that source's `csaw.yml`:

```yaml
csaw:
  protected:
    - AGENTS.md
    - rules/security.md

backend:
  include:
    - AGENTS.md
    - rules/**
```

When a file is protected:

- **Priority is bypassed.** Even if personal has priority 100, the protected source wins for that file.
- **Fork is refused.** `csaw fork client-acme/AGENTS.md --into personal` returns an error.
- **Protection is visible.** `csaw inspect` marks protected files with a `*` under the source.
- **Protection is verified.** csaw records a SHA-256 hash for protected mounts and `csaw check` / `csaw audit` detect content drift.

This is the mechanism for layered governance — let any upstream source (company, department, team, or client) publish required files, layer personal preferences on top, and csaw won't let the personal layer break the protected ones.

Protection is **local assurance, not hard enforcement**. csaw prevents its own mechanisms from bypassing protected files and detects if protected mounted content no longer matches the mount-time hash. Remount to accept an intentional protected source update. csaw does not sandbox the machine or stop a user from manually editing files outside csaw.

---

## Auditing Active Context

Create a starter policy:

```bash
csaw audit --init
```

Projects can declare local context requirements in `.csaw/policy.yml`. The canonical case is "this repo must mount the company and team sources, and not personal-experimental":

```yaml
required_sources:
  - company
  - department
  - name: team
    url: git@github.com:my-team/ai-config.git
    ref: main
blocked_sources:
  - personal-experimental
required_kinds:
  - instructions
  - rules
```

Run audit before starting work, before handing off, or in local/CI checks:

```bash
csaw audit
csaw audit --strict
csaw audit --json
```

`csaw audit` checks active mount health, protected file content drift, required sources, required source URLs and project pins, blocked source patterns, blocked kinds, blocked mounted paths, and required artifact kinds. Default mode exits nonzero on errors. `--strict` also exits nonzero on warnings, including a missing project policy.

The `ref` field checks the project pin set by `csaw pin team@main`; it is not inferred from the source checkout's current branch. The JSON output is documented in [docs/reference/audit-json.md](docs/reference/audit-json.md).

This is **local assurance**, not hard prevention. csaw can tell you the right sources are active and forbidden ones are absent, but it does not sandbox your machine or stop a user from manually editing files.

Example client isolation policy:

```yaml
required_sources:
  - name: client-acme
    url: git@example.com:org/client-acme-ai.git
    ref: approved
blocked_sources:
  - other-client-*
  - personal-experimental
blocked_kinds:
  - mcp
blocked_paths:
  - .claude/agents/**
required_kinds:
  - instructions
  - rules
```

Example team policy:

```yaml
required_sources:
  - platform-team
blocked_sources: []
blocked_kinds: []
blocked_paths: []
required_kinds:
  - instructions
  - rules
```

---

## Experimental Skills

Working on a new skill? Put it in `skills/experimental/`:

```
~/my-ai-config/
  skills/
    code-review/SKILL.md         ← stable, always mounted
    experimental/
      debug-strategy/SKILL.md    ← hidden from default mounts
```

The `.csawignore` file hides `skills/experimental/**` by default. To test an experimental skill:

```bash
csaw use personal/default --include-experimental
```

When you're confident it works, promote it:

```bash
csaw promote personal/skills/experimental/debug-strategy
# ✔ promoted debug-strategy from experimental to stable
#   Push: csaw push personal -m "promote debug-strategy"
```

This moves it from `skills/experimental/debug-strategy/` to `skills/debug-strategy/` — now it mounts by default.

To share a promoted skill with the team:

```bash
csaw fork personal/skills/debug-strategy/SKILL.md --into team
csaw push team -m "add debug-strategy skill"
```

---

## Pulling Team Updates

A teammate updated the team's AGENTS.md. Get the latest:

```bash
csaw pull team
# ✔ pulled team
```

Since your project's `AGENTS.md` is a symlink to the team registry, edits to
that file are visible instantly. If the team added new rules, skills, agents, or
MCP files, rerun the profile activation to add those new symlinks.

### What if I edited a mounted file?

If you edited `AGENTS.md` in your project, you actually edited the team registry (through the symlink). Now `csaw pull` detects uncommitted changes:

```
! team has uncommitted changes
  Commit:  cd ~/.csaw/sources/team && git add -A && git commit -m "..."
  Or stash: csaw pull team --stash
```

**`--stash`** stashes your changes, pulls, then pops the stash:

```bash
csaw pull team --stash
# ✔ pulled team
```

### What if the team and I changed the same file?

If you have local commits and the remote has diverged:

```
! team has diverged (2 local, 5 remote commits)
  Resolve: cd ~/.csaw/sources/team && git pull --rebase
```

This is standard git — csaw tells you what happened and where to fix it. The registry is a normal git repo.

---

## Sharing Your Changes

You updated a skill through a symlink (or edited the registry directly). Push it:

```bash
csaw push team -m "improve code review skill"
# ✔ pushed team
```

This runs `git add -A && git commit && git push` in the team registry. Your teammates pull the update with `csaw pull`.

If you're not sure and want to go through a PR instead:

```bash
csaw source clone team ~/Developer/team-config
cd ~/Developer/team-config
git checkout -b improve-code-review
# ... edit files ...
git add -A && git commit -m "improve code review"
git push -u origin improve-code-review
gh pr create
```

`csaw source clone` moves a remote source to a local directory for contribution. Now you can branch, PR, and collaborate like any codebase.

---

## Testing a Branch

You want to try a feature branch of the team config without affecting other projects:

```bash
csaw pin team@feature/new-rules
csaw pull team
csaw use team/backend
```

This project now uses the `feature/new-rules` branch. Other projects stay on main. When you're done:

```bash
csaw unpin team
csaw pull team
```

Back to main.

---

## Forking a Team File

You like the team's `AGENTS.md` but want to customize it. Fork it:

```bash
csaw fork team/AGENTS.md --into personal
```

This copies the file to your personal registry. Since personal has higher priority, your version gets mounted instead of the team's. The team original is untouched.

---

## Switching Profiles

Activating a new profile **replaces** the previous one automatically:

```bash
csaw use team/backend
# ... working on backend ...

csaw use team/frontend
# previous mount removed, frontend mounted
```

To go back to what you had before:

```bash
csaw mount --restore
```

To add files on top of an existing mount without replacing:

```bash
csaw use personal/extras --keep
```

---

## Clean Removal

```bash
csaw unmount
```

Every symlink is removed. If csaw stashed any original files during mount (because they existed before), they're restored. Your project is exactly as it was.

```
✔ 6 removed · 2 restored

  Remount: csaw mount --restore
```

---

## The Kinds

csaw treats AI workspace artifacts as five distinct kinds, each with its own conventions and projection target:

| Kind | Registry path | Projects to | When loaded |
|---|---|---|---|
| **Instructions** | `AGENTS.md`, `CLAUDE.md` | Project root | Every turn — always in context |
| **Rules** | `rules/*.md` | `.claude/rules/`, `.cursor/rules/`, etc. | Every turn — always-on coding standards |
| **Agents** | `agents/*.md` | `.claude/agents/`, `.opencode/agents/`, `.gemini/agents/` | When invoked — specialized subagent personas |
| **Skills** | `skills/*/SKILL.md` | `.claude/skills/`, `.opencode/skills/`, etc. | When relevant — on-demand procedural workflows |
| **MCP** | `mcp/*.json` | `.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` | Session start — tool/data connectivity |

**Agents vs skills.** Both are spawnable, both are markdown with frontmatter. The distinction: an *agent* defines a persona (a subagent with its own tools, scope, and prompt — Claude's `.claude/agents/code-reviewer.md`); a *skill* defines a procedure (a step-by-step workflow loaded only when relevant). Use agents when you want a specialist to take over for a focused task; use skills when you want guidance the main agent can pull in mid-task.

**Rules vs instructions.** Both are always loaded. The distinction is conventional: *instructions* (`AGENTS.md`) are the project-level summary every tool reads; *rules* are split-out always-on standards organized by topic (e.g., `rules/go-conventions.md`, `rules/security.md`).

You can mount selectively by kind:

```bash
csaw use team/backend --kind agents          # only agent definitions
csaw use team/backend --kind agents,skills   # agents and skills only
csaw use team/backend                        # all kinds
```

You write files once in your registry. csaw projects them into every tool's native directory. Mounted files are hidden from git via `.git/info/exclude`. Use `csaw show <path>` to make one visible, `csaw hide <path>` to hide it.

`csaw inspect` groups mounted files by kind within each source so you can see at a glance what's loaded.

---

## Configuring Tools

If csaw can't auto-detect any tool directories in your project on first mount, it asks which AI tools you use:

```
╭──────────────────────────────────────────╮
│  Which AI tools do you use?              │
│                                          │
│  ● Claude Code                           │
│  ● Cursor                                │
│  ○ OpenCode                              │
│  ○ Codex                                 │
│  ○ Windsurf                              │
│  ○ Gemini CLI                            │
│                                          │
│  space toggle · enter confirm            │
╰──────────────────────────────────────────╯
```

This is saved to `~/.csaw/config.yml` and applies to all projects. You can also set it directly:

```bash
csaw config set tools claude,cursor
```

> **Not yet supported by direct projection:** VS Code Copilot (`.github/copilot-instructions.md`, `.github/chatmodes/`, `.github/agents/`) and GitHub Copilot CLI (`~/.copilot/`). csaw still serves both via the universal `AGENTS.md` convention at project root, which both tools read. Full first-class support for Copilot's `.github/` projection requires structural changes tracked in [docs/reference/tool-projection.json](docs/reference/tool-projection.json) under `csaw_projection_audit.deferred`.

---

## Registry Structure

A csaw source is just a git repo with markdown files:

```
my-ai-config/
  csaw.yml              ← profiles (which files to mount)
  .csawignore           ← files hidden from default mounts
  AGENTS.md             ← project guidance (the standard)
  rules/                ← always-on coding standards
    go-conventions.md
    testing-standards.md
  agents/               ← subagent definitions (separate context windows)
    code-reviewer.md
    planner.md
  skills/               ← on-demand reusable workflows
    code-review/
      SKILL.md
    testing/
      SKILL.md
    experimental/       ← work in progress (hidden by .csawignore)
      new-idea/
        SKILL.md
  mcp/                  ← MCP server configs
    claude-code.json
```

Every file is standard markdown — usable with or without csaw.

### Profiles

Profiles go in `csaw.yml`. They define which files to mount:

```yaml
backend:
  description: Go backend development
  include:
    - AGENTS.md
    - rules/go-conventions.md
    - skills/code-review/**
    - skills/testing/**

frontend:
  extends: backend
  include:
    - rules/react-patterns.md
    - skills/react-testing/**
```

Profiles support glob patterns and inheritance. `extends` pulls in everything
from the parent. A profile can also compose another source's profile by using a
source-qualified parent:

```yaml
backend-with-my-tools:
  extends:
    - team/backend
  include:
    - skills/debugging/**
```

If this profile lives in your `personal` source, `skills/debugging/**` resolves
inside `personal`, while `team/backend` resolves inside the `team` source.

---

<details>
<summary><strong>Full command reference</strong></summary>

### Commands

| Command | What it does |
|---|---|
| `csaw init [dir]` | Scaffold a new registry. `--adopt` imports from existing project. |
| `csaw source add name url` | Add a source (auto-clones remote). `--priority n` for conflicts. |
| `csaw source remove name` | Remove a source. |
| `csaw source clone name dir` | Clone remote source locally for contributing. |
| `csaw source list` | List configured sources. |
| `csaw profile list` | List available named work modes. |
| `csaw profile show name` | Show the resolved profile recipe. |
| `csaw use name` | Activate a named profile. Replaces previous mount. |
| `csaw mount` | Interactive profile picker. |
| `csaw mount profile name` | Mechanical equivalent of `csaw use name`. |
| `csaw mount paths patterns` | Advanced: mount registry paths or globs directly. |
| `csaw mount --profile name` | Backward-compatible named profile mount. |
| `csaw mount --restore` | Re-mount the previous selection. |
| `csaw unmount [patterns]` | Remove mounted files, restore originals. |
| `csaw inspect` | Full state: sources, mounts, priorities, pins. |
| `csaw audit [path]` | Audit active context against `.csaw/policy.yml`. |
| `csaw audit --init [path]` | Write a starter `.csaw/policy.yml`. |
| `csaw check` | Detect broken links, drifted links, and protected content drift. |
| `csaw update` | Repair drifted links. |
| `csaw diff path` | Diff a mounted file against its source. |
| `csaw pull [source]` | Pull latest from remote sources. `--stash` for dirty state. |
| `csaw push [source] -m "msg"` | Commit and push source changes. |
| `csaw pin source@ref` | Pin source to a branch/tag for this project. |
| `csaw unpin source` | Unpin, return to default branch. |
| `csaw fork source/path` | Copy a file into another source. `--into target`. |
| `csaw promote source/skills/experimental/name` | Promote experimental skill to stable. |
| `csaw config set key value` | Set config (tools, default_fork_target). |
| `csaw config list` | Show configuration. |
| `csaw show / hide path` | Control git visibility of mounted files. |
| `csaw status` | Quick summary. |

### Key Flags

| Flag | Commands | What it does |
|---|---|---|
| `--profile name` | mount | Named profile to mount. |
| `--kind list` | use, mount | Filter by kind: `agents`, `skills`, `rules`, `mcp`, `instructions` (repeatable). |
| `--force` | use, mount | Overwrite conflicts, stash originals. |
| `--keep` | use, mount | Add to existing mount instead of replacing. |
| `--tools list` | use, mount | Target tools (e.g., `--tools claude,cursor`). |
| `--restore` | mount | Re-mount previous selection. |
| `--include-experimental` | use, mount | Include experimental skills (hidden by .csawignore). |
| `--strict` | audit | Fail on warnings as well as errors. |
| `--json` | audit | Emit a machine-readable audit report. |
| `--init` | audit | Write a starter `.csaw/policy.yml`. |
| `--force` | audit | Overwrite an existing policy when used with `--init`. |
| `--adopt` | init | Import existing AI config from current project. |
| `--stash` | pull | Stash uncommitted changes before pulling. |
| `--priority n` | source add | Source priority (higher wins on conflict). |
| `--into source` | fork | Target source to fork into. |

</details>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for workflow, validation, and repo standards.

## License

MIT
