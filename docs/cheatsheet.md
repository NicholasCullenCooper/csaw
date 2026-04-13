# csaw Cheat Sheet

## Setup

```bash
# Install
uv tool install csaw

# Add a team source (auto-clones)
csaw source add team git@github.com:org/ai-config.git

# Or create your own registry
csaw init ~/my-ai-config
csaw source add personal ~/my-ai-config --priority 10
```

## Mount

```bash
csaw mount                              # interactive profile picker
csaw mount --profile team/backend       # mount a named profile
csaw mount agents/go.md                 # mount specific files
csaw mount --profile team/core --force  # overwrite conflicts
csaw mount --restore                    # re-mount previous selection
csaw mount --keep --profile team/extra  # add to existing mount (don't replace)
```

Mounting a profile **replaces** the previous mount by default. Use `--keep` to add on top.

## Unmount

```bash
csaw unmount                            # unmount everything
csaw unmount agents/go.md               # unmount specific files
```

## Inspect

```bash
csaw inspect                            # full state overview
csaw inspect --source team              # browse a source
csaw status                             # quick summary
csaw check                              # find broken links
csaw diff AGENTS.md                     # compare mounted vs source
```

## Git Visibility

```bash
csaw show AGENTS.md                     # make visible to git
csaw hide AGENTS.md                     # hide from git again
```

## Sources

```bash
csaw source list                        # show configured sources
csaw source add name url-or-path        # add a source (auto-clones remote)
csaw source remove name                 # remove a source
csaw source clone team ~/Developer/team # clone remote locally to contribute
csaw pull                               # update all remote sources
csaw pull team                          # update one source
csaw push team -m "updated rules"       # push source changes
csaw push                               # auto-detect dirty source and push
```

## Pin a Branch

```bash
csaw pin team@feature/new-rules         # pin this project to a branch
csaw pull team                          # pulls that branch
csaw mount --profile team/backend       # mounts from the branch
csaw unpin team                         # back to default branch
```

## Fork a File

```bash
csaw fork team/agents/base.md --into personal  # copy for personal editing
```

## Source Priority

When two sources provide the same file, higher priority wins:

```bash
csaw source add personal ~/my-config --priority 10  # wins over default (0)
csaw source add team git@github.com:org/config.git   # priority 0 (default)
```

## Create a Registry

```bash
csaw init ~/my-ai-config                # scaffold with csaw.yml, agents/, skills/
csaw init ~/my-ai-config --name myteam  # custom name
```

## Repair

```bash
csaw check                              # detect drift
csaw update                             # repair broken links
```

## Where Files Go

```
AGENTS.md       →  project root (project guidance, the standard)
CLAUDE.md       →  project root (Claude Code)
rules/*.md      →  .claude/rules/   .cursor/rules/   (always-on standards)
agents/*.md     →  .claude/agents/  .cursor/agents/  (subagent definitions)
skills/         →  .claude/skills/  .agents/skills/  (on-demand workflows)
mcp/            →  .mcp.json  .vscode/mcp.json  .cursor/mcp.json
commands/       →  project root
```

`.agents/skills/` is always created as a fallback. Other tool directories are used only if they already exist or are configured via `csaw config set tools`.

## Profile Format (`csaw.yml`)

```yaml
base:
  description: Foundation rules
  include:
    - AGENTS.md
    - skills/code-review/**

backend:
  extends: base
  description: Go backend
  include:
    - agents/go.md
    - skills/go-patterns/**

full:
  include:
    - "**/*"
```

## Registry Layout

```
my-registry/
  csaw.yml          # profiles
  .csawignore       # hide from default mounts
  AGENTS.md
  agents/
    base.md
    go.md
  skills/
    code-review/
      SKILL.md
    go-patterns/
      SKILL.md
```

## Key Concepts

**Mount, not install** — Symlinks from a registry. Reversible. Your repo stays clean.

**Profiles** — Named file selections with glob patterns. Can inherit from each other.

**Sources** — Git repos or local dirs containing agent files. Add as many as you need.

**Priority** — When sources overlap, higher priority wins. Set with `--priority` on `source add`.

**Pinning** — Lock a source to a branch/tag per project with `csaw pin`. Uses git worktrees.

**Fork** — Copy a team file into your own source for personal editing with `csaw fork`.

**Tool directories** — Skills mount into `.claude/skills/`, `.opencode/skills/`, etc. where AI tools discover them natively.

**Git exclude** — Mounted files are hidden from git by default. Use `csaw show`/`hide` to control visibility. Files in already-gitignored directories (like `.claude/`) need no extra exclusion.
