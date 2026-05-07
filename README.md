<p align="center">
  <h1 align="center">csaw</h1>
  <p align="center">
    <strong>Multi-source AI workspace governance.</strong><br>
    Mount team and client AI configs alongside personal ones — with protected paths, priority resolution, forkable lineage, and per-project pinning.
  </p>
  <p align="center">
    <a href="https://github.com/NicholasCullenCooper/csaw/actions"><img src="https://github.com/NicholasCullenCooper/csaw/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://github.com/NicholasCullenCooper/csaw/releases"><img src="https://img.shields.io/github/v/release/NicholasCullenCooper/csaw?include_prereleases&label=version" alt="Version"></a>
    <a href="https://pypi.org/project/csaw/"><img src="https://img.shields.io/pypi/v/csaw" alt="PyPI"></a>
  </p>
  <p align="center">
    Works with: Claude Code · Codex · Cursor · Copilot · Windsurf · OpenCode · Gemini CLI
  </p>
</p>

---

## Who csaw is for

You have **more than one source of AI configuration**:

- **Staff engineers across multiple clients or product teams** — each codebase has its own rules, skills, and policies you must respect.
- **Contractors and consultants juggling clients** — each engagement demands its own MCP servers, conventions, and security posture, and these must not bleed across projects.
- **Teams with mandated AI policy** — a security or platform team publishes config that engineers must use, with personal preferences layered on top without being able to break the mandate.
- **Individuals composing personal config** with one or more team or community sources.

If you only have one set of AI files to manage, simpler tools work fine. csaw earns its complexity when you have multiple stakeholders in your AI workspace.

## The problem

Multi-stakeholder AI config is a governance problem:

- **No source of truth across projects.** A team's `AGENTS.md` gets copy-pasted into every repo. Each copy drifts independently. The "real" version is whoever pushed last.
- **No way to enforce policy.** Security mandates a rule. A developer overrides it locally. Nobody notices.
- **No client isolation.** Contractor on Client A's repo runs with personal MCP servers connected to Client B's databases. One slip from data exposure.
- **No layering.** Team has shared rules; you want personal additions on top. Composing them per repo is manual, so nobody bothers.
- **No lineage.** Fork a team skill, customize for your style, push improvements back? Manual copy-paste, no record of what diverged.
- **Cleanup is impossible.** Tried an experimental config; now hunting through 3 tool directories and 6 files.

## What csaw does

Keep your AI config in one or more **git-backed sources** — personal, team, per-client, community. csaw mounts them into your projects via symlinks, composing across sources with priority resolution, protected files that can't be overridden, per-project pinning to specific git refs, and forkable lineage between sources. Update a source — every project sees the change instantly. Unmount, and it's like csaw was never there.

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

Nothing is committed to your project. Git never sees the files. Unmount, and they're gone.

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

Now mount it into your project:

```bash
cd ~/my-project
csaw mount --profile my-ai-config/default
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

Open Claude Code (or Cursor, Codex, Copilot) — it finds the files automatically. Run `git status` — nothing shows up. The files are hidden via `.git/info/exclude`.

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

Now you can delete the originals from your project, register the source, and mount instead:

```bash
csaw source add personal ~/my-ai-config --priority 10
csaw mount --profile personal/default
```

---

## Mounting a Team Source

Your team keeps shared AI config in a git repo. One command to get it:

```bash
csaw source add team git@github.com:your-org/ai-config.git
```

```
✔ registered source "team"
✔ cloned team
```

csaw auto-clones the repo. Now mount:

```bash
cd ~/my-project
csaw mount --profile team/backend
```

Your project gets the team's AGENTS.md, skills, and rules — all symlinked. Every repo on the team mounts the same source. When someone updates the team config:

```bash
csaw pull team
# ✔ pulled team
```

Every project sees the update instantly through the symlinks. No re-mounting needed.

---

## Composing Multiple Sources

You want a team or client's shared config **plus** your personal preferences. Or you're a contractor with `client-acme` and `client-globex` configs, never to bleed across projects. Add multiple sources:

```bash
csaw init ~/my-ai-config
csaw source add personal ~/my-ai-config --priority 10
csaw source add client-acme git@github.com:acme/ai-config.git --priority 50
```

Now mount uses all configured sources:

```bash
csaw mount --profile client-acme/backend
```

If personal has `skills/debug-strategy/SKILL.md` and client-acme has `skills/code-review/SKILL.md`, **both** get mounted — they're different files, no conflict.

### What if two sources provide the same file?

**Priority decides.** Higher number wins.

```bash
csaw inspect
```

```
Sources
  client-acme (remote, priority 50) → ~/.csaw/sources/client-acme
  personal (local, priority 10) → ~/my-ai-config
```

You can set priority on any source:

```bash
csaw source add team git@github.com:org/config.git --priority 0
csaw source add personal ~/my-config --priority 10     # wins on conflicts
```

If two sources have the same priority and provide the same file, csaw errors and tells you to resolve it explicitly.

---

## Protected Files

When a source needs to enforce that certain files **cannot be overridden** — a team's mandatory security rules, a client's required `AGENTS.md` — mark them as protected in that source's `csaw.yml`:

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

This is the mechanism for team and client governance — let a team or client's source publish required files, layer personal preferences on top, and csaw won't let the personal layer break the protected ones.

Protection is **advisory within csaw** — it prevents csaw's own mechanisms from bypassing the rules. A developer can still manually delete a symlink and write their own file in its place. csaw doesn't try to stop that. This is an open-source tool, not an enterprise MDM.

> Future work: content-hash verification. csaw could record the SHA of each protected file at mount time and detect if someone replaces the symlink with a modified copy. `csaw check --strict` would fail the check. Tracked as tech debt.

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
csaw mount --profile personal/default --include-experimental
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

Since your project's `AGENTS.md` is a symlink to the team registry, the update is visible instantly — no remount needed.

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
csaw mount --profile team/backend
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

Mounting a new profile **replaces** the previous one automatically:

```bash
csaw mount --profile team/backend
# ... working on backend ...

csaw mount --profile team/frontend
# previous mount removed, frontend mounted
```

To go back to what you had before:

```bash
csaw mount --restore
```

To add files on top of an existing mount without replacing:

```bash
csaw mount --keep --profile personal/extras
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
| **Agents** | `agents/*.md` | `.claude/agents/`, `.cursor/agents/`, etc. | When invoked — specialized subagent personas |
| **Skills** | `skills/*/SKILL.md` | `.claude/skills/`, `.opencode/skills/`, etc. | When relevant — on-demand procedural workflows |
| **MCP** | `mcp/*.json` | `.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` | Session start — tool/data connectivity |

**Agents vs skills.** Both are spawnable, both are markdown with frontmatter. The distinction: an *agent* defines a persona (a subagent with its own tools, scope, and prompt — Claude's `.claude/agents/code-reviewer.md`); a *skill* defines a procedure (a step-by-step workflow loaded only when relevant). Use agents when you want a specialist to take over for a focused task; use skills when you want guidance the main agent can pull in mid-task.

**Rules vs instructions.** Both are always loaded. The distinction is conventional: *instructions* (`AGENTS.md`) are the project-level summary every tool reads; *rules* are split-out always-on standards organized by topic (e.g., `rules/go-conventions.md`, `rules/security.md`).

You can mount selectively by kind:

```bash
csaw mount --profile team/backend --kind agents          # only agent definitions
csaw mount --profile team/backend --kind agents,skills   # agents and skills only
csaw mount --profile team/backend                        # all kinds
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
│                                          │
│  space toggle · enter confirm            │
╰──────────────────────────────────────────╯
```

This is saved to `~/.csaw/config.yml` and applies to all projects. You can also set it directly:

```bash
csaw config set tools claude,cursor
```

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

Profiles support glob patterns and inheritance. `extends` pulls in everything from the parent.

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
| `csaw mount [patterns]` | Mount files. Replaces previous mount. Picker if no args. |
| `csaw mount --profile name` | Mount a named profile. |
| `csaw mount --restore` | Re-mount the previous selection. |
| `csaw unmount [patterns]` | Remove mounted files, restore originals. |
| `csaw inspect` | Full state: sources, mounts, priorities, pins. |
| `csaw check` | Detect broken or drifted symlinks. |
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
| `--kind list` | mount | Filter by kind: `agents`, `skills`, `rules`, `mcp`, `instructions` (repeatable). |
| `--force` | mount | Overwrite conflicts, stash originals. |
| `--keep` | mount | Add to existing mount instead of replacing. |
| `--tools list` | mount | Target tools (e.g., `--tools claude,cursor`). |
| `--restore` | mount | Re-mount previous selection. |
| `--include-experimental` | mount | Include experimental skills (hidden by .csawignore). |
| `--adopt` | init | Import existing AI config from current project. |
| `--stash` | pull | Stash uncommitted changes before pulling. |
| `--priority n` | source add | Source priority (higher wins on conflict). |
| `--into source` | fork | Target source to fork into. |

</details>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for workflow, validation, and repo standards.

## License

MIT
