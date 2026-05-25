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
    Works with: Claude Code · Cursor · Codex · OpenCode · Windsurf · Antigravity (Google)
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

Open Claude Code (or Cursor, Codex, OpenCode, Antigravity) — it finds the files automatically. Run `git status` — nothing shows up. The files are hidden via `.git/info/exclude`.
---

## Walkthroughs

The full scenario-based walkthrough lives in [docs/walkthrough.md](docs/walkthrough.md). Jump straight to the section that matches your situation:

- **[I Already Have an AGENTS.md](docs/walkthrough.md#i-already-have-an-agentsmd)** — adopt existing AI config into a registry
- **[Using a Team Source](docs/walkthrough.md#using-a-team-source)** — share config from a team git repo
- **[Composing Multiple Sources](docs/walkthrough.md#composing-multiple-sources)** — layer company + team + personal (the canonical case)
- **[Protected Files](docs/walkthrough.md#protected-files)** — enforce mandates that can't be overridden
- **[Auditing Active Context](docs/walkthrough.md#auditing-active-context)** — declare and verify project policy
- **[Experimental Skills](docs/walkthrough.md#experimental-skills)** — develop skills before promoting
- **[Pulling Team Updates](docs/walkthrough.md#pulling-team-updates)** — keep in sync
- **[Sharing Your Changes](docs/walkthrough.md#sharing-your-changes)** — push edits back to a source
- **[Forking a Team File](docs/walkthrough.md#forking-a-team-file)** — customize without breaking upstream
- **[Switching Profiles · Clean Removal · Testing a Branch](docs/walkthrough.md)** — and more

## The Kinds

csaw treats AI workspace artifacts as seven distinct kinds, each with its own conventions and projection target:

| Kind | Registry path | Projects to | When loaded |
|---|---|---|---|
| **Instructions** | `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.goosehints` | Project root | Every turn — always in context |
| **Rules** | `rules/*.md` | `.claude/rules/`, `.cursor/rules/`, `.amazonq/rules/`, `.kiro/steering/`, `.codebuddy/rules/`, `.windsurf/rules/` | Every turn — always-on coding standards |
| **Agents** | `agents/*.md` | `.claude/agents/`, `.opencode/agents/`, `.kiro/agents/`, `.codebuddy/agents/`, `.openhands/microagents/` | When invoked — specialized subagent personas |
| **Skills** | `skills/*/SKILL.md` | `.claude/skills/`, `.opencode/skills/`, `.agents/skills/` (Antigravity + fallback), `.codex/skills/` | When relevant — on-demand procedural workflows |
| **MCP** | `mcp/*.json` | `.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` | Session start — tool/data connectivity |
| **Hooks** | `hooks/*` | `.claude/hooks/`, `.kiro/hooks/` | Tool lifecycle events |
| **Ignore** | `ignore/*` | `.cursorignore`, `.cody/ignore`, `.aiderignore`, `.tongyiignore` | Always — context exclusion |

> csaw does **not** project `settings` (they contain API keys/credentials) or `memory` (session-state, user-private). See [`docs/planning/projection-roadmap.md`](docs/planning/projection-roadmap.md).

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
│  ○ Codex CLI                             │
│  ○ Windsurf                              │
│  ○ Antigravity (Google)                  │
│  ○ Amazon Q Developer                    │
│  ○ Kiro (AWS)                            │
│  ○ CodeBuddy (Tencent)                   │
│  ○ OpenHands                             │
│                                          │
│  space toggle · enter confirm            │
╰──────────────────────────────────────────╯
```

This is saved to `~/.csaw/config.yml` and applies to all projects. You can also set it directly:

```bash
csaw config set tools claude,cursor
```

> **Notes on tool coverage:**
> - **GitHub Copilot (VS Code + CLI):** csaw serves both via the universal `AGENTS.md` convention. Direct `.github/` projection requires per-suffix filename support and per-tool git-visibility flags — tracked as future work.
> - **Gemini CLI:** Removed in v0.6.0 (Google sunset 2026-06-18). Migrate to Antigravity — it uses `.agents/` (csaw's StandardFallback, served automatically) and reads `GEMINI.md`.
>
> Full per-tool projection details — paths, fallbacks, MCP schemas, and tools we don't yet target — are catalogued in [docs/reference/tool-projection.json](docs/reference/tool-projection.json).

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
