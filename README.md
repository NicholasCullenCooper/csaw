<p align="center">
  <h1 align="center">csaw</h1>
  <p align="center">
    <strong>Mount, not install.</strong> Your AI workspace — symlinked from registries, reversible, inspectable.
  </p>
  <p align="center">
    <a href="https://github.com/csaw-ai/csaw/actions"><img src="https://github.com/csaw-ai/csaw/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://github.com/csaw-ai/csaw/releases"><img src="https://img.shields.io/github/v/release/csaw-ai/csaw?include_prereleases&label=version" alt="Version"></a>
    <a href="https://pypi.org/project/csaw/"><img src="https://img.shields.io/pypi/v/csaw" alt="PyPI"></a>
  </p>
  <p align="center">
    Works with: Claude Code · OpenCode · Codex · Cursor · Windsurf · Gemini CLI
  </p>
</p>

<p align="center">
  <img src="docs/assets/demo.gif" alt="csaw demo" width="800">
</p>

---

csaw manages agent instructions, skills, and configs from git-backed registries — symlinked into your projects, mounted into tool-native directories, and removed without a trace.

```bash
csaw mount                        # pick a profile interactively
csaw mount --profile team/backend # or specify directly
csaw inspect                      # see everything that's active
csaw unmount                      # clean removal, no trace
```

## Why csaw?

Your AI coding tools (Claude Code, OpenCode, Codex, Cursor) need configuration files — AGENTS.md, skills, rules. Today you copy them between projects, they drift, and switching between setups means manually swapping files.

csaw mounts these files from a central registry using symlinks. When you're done, unmount. When you switch tasks, switch profiles. Your project's git history stays clean.

|  | Copy & paste | csaw |
|---|---|---|
| **Install** | Copy files into repo | Symlink from registry |
| **Update** | Re-copy, hope nothing drifted | Pull registry, links update live |
| **Undo** | Delete files, hope you got them all | `csaw unmount` — originals restored |
| **Switch** | Manual file swapping | `csaw mount --profile frontend` |
| **Git** | Config files in your history | Hidden from git automatically |
| **Tools** | Manually place in each tool's dir | Auto-detected, auto-mounted |

## Install

```bash
# Recommended (any platform)
uv tool install csaw

# macOS / Linux
brew install --cask csaw-ai/tap/csaw

# Windows
scoop bucket add csaw-ai https://github.com/csaw-ai/scoop-bucket
scoop install csaw
```

> **macOS note (Homebrew):** If you see "Apple could not verify", run:
> ```bash
> xattr -d com.apple.quarantine "$(which csaw)"
> ```
> This is normal for unsigned CLI tools distributed via Homebrew casks.

<details>
<summary>Other install methods</summary>

```bash
# pipx
pipx install csaw

# Go from source (requires Go 1.22+)
go install github.com/csaw-ai/csaw/cmd/csaw@latest
```

</details>

## Quick Start

### 1. Create or add a source

A source is any git repo (or local directory) containing agent files and skills.

```bash
# Create a new registry
csaw init ~/my-ai-config
csaw source add personal ~/my-ai-config --priority 10

# Or add an existing team registry
csaw source add team git@github.com:your-org/ai-config.git
csaw pull team
```

### 2. Mount a profile

Profiles are named sets of files defined in a `csaw.yml` in the source:

```yaml
# csaw.yml
backend:
  include:
    - agents/base.md
    - agents/go.md
    - skills/code-review/**
    - skills/testing/**

frontend:
  extends: backend
  include:
    - agents/react.md
    - skills/react-patterns/**
```

Mount one:

```bash
csaw mount --profile team/backend
```

Or just run `csaw mount` — you'll get an interactive picker showing all available profiles with descriptions.

### 3. Inspect what's active

```bash
csaw inspect
```

```
csaw inspect

  project:       /home/you/api-server
  csaw home:     /home/you/.csaw
  sources:       1
  mounted:       12

Sources
  team (remote) → /home/you/.csaw/sources/team

Mounted files

  team
    ✔ .claude/skills/code-review/SKILL.md
    ✔ .claude/skills/testing/SKILL.md
    ✔ .agents/skills/code-review/SKILL.md
    ✔ .agents/skills/testing/SKILL.md
    ✔ AGENTS.md
    ✔ agents/base.md
    ✔ agents/go.md
    ...
```

### 4. Work normally

Open your AI tool. Skills are mounted into tool-native directories (`.claude/skills/`, `.opencode/skills/`, `.agents/skills/`) where they're automatically discovered. AGENTS.md is at your project root.

### 5. Clean up

```bash
csaw unmount           # remove everything, restore originals
csaw mount --restore   # re-mount what was there before
```

## How It Works

csaw uses **symlinks**, not file copies. Your registry is the source of truth:

```
your-project/
  .claude/skills/code-review/SKILL.md  →  ~/.csaw/sources/team/skills/code-review/SKILL.md
  .agents/skills/code-review/SKILL.md  →  ~/.csaw/sources/team/skills/code-review/SKILL.md
  AGENTS.md                            →  ~/.csaw/sources/team/AGENTS.md
```

- **Skills** mount into tool-native directories (`.claude/skills/`, `.opencode/skills/`, `.agents/skills/`). These are typically gitignored, so git stays clean.
- **AGENTS.md** and other non-skill files mount at the project root, with entries in `.git/info/exclude` to keep them out of git status.
- csaw checks `.gitignore` first — if a path is already covered, no extra exclude is needed.

### Git visibility

By default, mounted files are hidden from git. To make a file visible:

```bash
csaw show AGENTS.md    # remove from git exclude → visible to git
csaw hide AGENTS.md    # add back → hidden from git
```

## Commands

| Command | What it does |
|---|---|
| `csaw init [dir]` | Scaffold a new registry with csaw.yml, agents/, skills/. |
| `csaw mount [patterns]` | Mount files. Replaces any previous mount. Interactive picker if no args. |
| `csaw mount --profile name` | Mount a named profile. |
| `csaw mount --restore` | Re-mount the previous selection. |
| `csaw unmount [patterns]` | Remove mounted files, restore originals. |
| `csaw inspect` | Show full state: sources, mounted files, health. |
| `csaw inspect --source name` | Browse a source's contents. |
| `csaw check` | Detect broken or drifted symlinks. |
| `csaw update` | Repair drifted links. |
| `csaw diff path` | Show diff between mounted file and its source. |
| `csaw pull [source]` | Pull latest from remote sources. |
| `csaw push [source] -m "msg"` | Commit and push source changes. |
| `csaw pin source@ref` | Pin a source to a branch or tag for this project. |
| `csaw unpin source` | Unpin, return to default branch. |
| `csaw fork source/path` | Copy a file into another source for personal editing. |
| `csaw show path` | Make a mounted file visible to git. |
| `csaw hide path` | Hide a mounted file from git. |
| `csaw source add name url` | Add a git or local source. |
| `csaw source remove name` | Remove a source. |
| `csaw source list` | List configured sources. |
| `csaw status` | Show mounted files, sources, stashed originals. |
| `csaw version` | Print version. |

### Flags

| Flag | Commands | What it does |
|---|---|---|
| `--profile name` | mount | Use a named profile for file selection. |
| `--exclude glob` | mount | Exclude files matching a pattern. |
| `--include-ignored` | mount | Include files hidden by `.csawignore`. |
| `--force` | mount | Overwrite conflicts, stash originals. |
| `--skip-conflicts` | mount | Skip files that conflict. |
| `--restore` | mount | Re-mount the previous selection. |
| `--keep` | mount | Keep existing mounts instead of replacing them. |
| `--source name` | inspect | Show details for a specific source. |
| `--into source` | fork | Target source to fork into. |
| `--name name` | init | Registry name (defaults to directory name). |
| `--priority n` | source add | Source priority (higher wins on conflict). |

## Profiles

Profiles live in `csaw.yml` at the root of any source. They support glob patterns and inheritance:

```yaml
base:
  description: Shared foundation for all profiles
  include:
    - AGENTS.md
    - skills/code-review/**

backend:
  description: Go backend development
  extends: base
  include:
    - agents/go.md
    - skills/go-patterns/**
    - skills/testing/**

security:
  extends: base
  include:
    - skills/security-review/**
  exclude:
    - skills/testing/**
```

Profiles from different sources can reference each other: `extends: team/base`.

## Registry Structure

A csaw source is just a directory (usually a git repo) with agent files and an optional `csaw.yml`:

```
my-ai-config/
  csaw.yml              # profiles (optional)
  .csawignore           # files hidden from default mounts (optional)
  AGENTS.md             # root agent instructions
  agents/
    base.md
    go.md
    react.md
  skills/
    code-review/
      SKILL.md
    testing/
      SKILL.md
    go-patterns/
      SKILL.md
```

Every file is standard markdown — usable without csaw.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for workflow, validation, and repo standards.

## License

MIT
