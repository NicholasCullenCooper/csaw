<p align="center">
  <h1 align="center">csaw</h1>
  <p align="center">
    <strong>Mount, not install.</strong> One registry of AI rules, skills, and configs — mounted into every project, never committed, never drifted.
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

Your AI tools need configuration — AGENTS.md, skills, rules, MCP configs. Today those files are copy-pasted into every repo, they drift between projects, they clutter your git history, and onboarding a teammate means "copy these files from the wiki."

**csaw fixes this.** You keep one registry of AI config. csaw symlinks it into your projects. Update the registry, every project sees the change instantly. Unmount, and your repo is clean — no files left behind, no commits to revert.

```
your-registry/                         your-project/
  AGENTS.md                     ──→      AGENTS.md (symlink)
  skills/                                .claude/skills/
    code-review/SKILL.md        ──→        code-review/SKILL.md (symlink)
    testing/SKILL.md            ──→        testing/SKILL.md (symlink)
  agents/                                .claude/rules/
    go.md                       ──→        go.md (symlink)
```

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

## Get Started in 60 Seconds

```bash
# Add your team's AI config registry
csaw source add team git@github.com:your-org/ai-config.git
# ✔ registered source "team"
# ✔ cloned team

# Mount into your project
cd ~/my-project
csaw mount --profile team/backend
```

```
╭──────────────────────────────────────────────╮
│                                              │
│  mounted                                     │
│                                              │
│  team                                        │
│   ✔ AGENTS.md                                │
│   ✔ .claude/skills/code-review/SKILL.md      │
│   ✔ .claude/skills/testing/SKILL.md          │
│   ✔ .claude/rules/go.md                      │
│                                              │
│  4 files mounted · 2 tool dirs               │
│                                              │
│  Inspect: csaw inspect                       │
│  Unmount: csaw unmount                       │
│                                              │
╰──────────────────────────────────────────────╯
```

That's it. Your AI tools see the files. Your git history doesn't.

## Why Not Just Copy the Files?

**They drift.** You update your team's AGENTS.md. Now it's different in 12 repos. Which version does each repo have? Nobody knows.

**They clutter.** Every AI tool wants its own config files — `.cursorrules`, `copilot-instructions.md`, AGENTS.md, skills in `.claude/skills/`. That's 10+ files committed to every repo, creating PR noise and merge conflicts.

**They're fragile.** New person joins. "Copy these files from the wiki." They miss one. Their AI gives bad advice. Nobody notices for a week.

**They're permanent.** You tried a new AI config. It didn't work. Now you have to find and delete 6 files across 3 tool directories — and hope you didn't miss one.

csaw solves all of this:

- **One source of truth.** Update your registry, every project gets the change via symlinks — instantly, no reinstall.
- **Nothing in git.** Mounted files are hidden from git automatically. No commits, no PRs, no merge conflicts.
- **Clean undo.** `csaw unmount` removes everything and restores any files that were there before.
- **Onboard in one command.** `csaw source add team <url>` — the whole team's config, ready to mount.
- **Drift detection.** `csaw check` finds broken links and files that don't match their source.

## Create Your Own Registry

```bash
csaw init ~/my-ai-config
# ✔ initialized registry "my-ai-config"
# Creates: csaw.yml, AGENTS.md, skills/code-review/, skills/commit-message/
```

A registry is just a git repo with markdown files:

```
my-ai-config/
  csaw.yml              ← profiles (which files to mount)
  AGENTS.md             ← your coding rules and preferences
  skills/
    code-review/
      SKILL.md          ← reusable skill
    commit-message/
      SKILL.md
```

Every file is standard markdown — usable with or without csaw.

## Profiles

Profiles define named sets of files. Put them in `csaw.yml`:

```yaml
backend:
  description: Go backend development
  include:
    - AGENTS.md
    - agents/go.md
    - skills/code-review/**
    - skills/testing/**

frontend:
  extends: backend
  include:
    - agents/react.md
    - skills/react-patterns/**
```

Mount one: `csaw mount --profile team/backend`

Or just `csaw mount` for an interactive picker.

## Where Files Get Mounted

csaw automatically mounts files where each AI tool expects them:

```
Registry path        →   Project path                    →   Discovered by
─────────────────────────────────────────────────────────────────────────────
AGENTS.md            →   AGENTS.md                       →   Codex, Cursor, Copilot, Windsurf
CLAUDE.md            →   CLAUDE.md                       →   Claude Code
agents/go.md         →   .claude/rules/go.md             →   Claude Code
                         .cursor/rules/go.md             →   Cursor
skills/foo/SKILL.md  →   .claude/skills/foo/SKILL.md     →   Claude Code
                         .agents/skills/foo/SKILL.md     →   Codex, Copilot
mcp/claude-code.json →   .mcp.json                       →   Claude Code
```

You write files once in registry-standard paths. csaw projects them into every tool's native directory.

Mounted files are automatically hidden from git via `.git/info/exclude`. Use `csaw show <path>` to make a file visible, `csaw hide <path>` to hide it again.

<details>
<summary><strong>Full command reference</strong></summary>

### Commands

| Command | What it does |
|---|---|
| `csaw init [dir]` | Scaffold a new registry. `--adopt` to import from existing project. |
| `csaw source add name url` | Add a source (auto-clones remote). `--priority n` for conflict resolution. |
| `csaw source remove name` | Remove a source. |
| `csaw source clone name dir` | Clone a remote source locally for contributing. |
| `csaw source list` | List configured sources. |
| `csaw mount [patterns]` | Mount files. Replaces previous mount. Interactive picker if no args. |
| `csaw mount --profile name` | Mount a named profile. |
| `csaw mount --restore` | Re-mount the previous selection. |
| `csaw unmount [patterns]` | Remove mounted files, restore originals. |
| `csaw inspect` | Show full state: sources, mounts, priorities, pins. |
| `csaw check` | Detect broken or drifted symlinks. |
| `csaw update` | Repair drifted links. |
| `csaw diff path` | Diff a mounted file against its source. |
| `csaw pull [source]` | Pull latest from remote sources. `--stash` to handle dirty state. |
| `csaw push [source] -m "msg"` | Commit and push source changes. |
| `csaw pin source@ref` | Pin a source to a branch or tag for this project. |
| `csaw unpin source` | Unpin, return to default branch. |
| `csaw fork source/path` | Copy a file into another source for editing. `--into target`. |
| `csaw config set key value` | Set config (tools, default_fork_target). |
| `csaw config list` | Show all configuration. |
| `csaw show / hide path` | Control git visibility of mounted files. |
| `csaw status` | Quick summary of sources and mounts. |

### Key Flags

| Flag | Commands | What it does |
|---|---|---|
| `--profile name` | mount | Named profile to mount. |
| `--force` | mount | Overwrite conflicts, stash originals. |
| `--keep` | mount | Add to existing mounts instead of replacing. |
| `--tools list` | mount | Target tools (e.g., `--tools claude,cursor`). |
| `--restore` | mount | Re-mount the previous selection. |
| `--adopt` | init | Import existing AI config files from current project. |
| `--stash` | pull | Stash uncommitted changes before pulling. |
| `--priority n` | source add | Source priority (higher wins on conflict). |
| `--into source` | fork | Target source to fork into. |

</details>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for workflow, validation, and repo standards.

## License

MIT
