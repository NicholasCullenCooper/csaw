# csaw Cheat Sheet

## Setup

```bash
# Install
go install github.com/csaw-ai/csaw/cmd/csaw@latest

# Add a source (git repo or local directory)
csaw source add team git@github.com:org/ai-config.git
csaw pull team

# Or local
csaw source add local ~/my-ai-config
```

## Mount

```bash
csaw mount                              # interactive profile picker
csaw mount --profile team/backend       # mount a named profile
csaw mount agents/go.md                 # mount specific files
csaw mount --profile team/core --force  # overwrite conflicts
csaw mount --restore                    # re-mount previous selection
```

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
csaw source add name url-or-path        # add a source
csaw source remove name                 # remove a source
csaw pull                               # update all remote sources
csaw pull team                          # update one source
csaw push -m "updated rules"            # push personal registry
```

## Repair

```bash
csaw check                              # detect drift
csaw update                             # repair broken links
```

## Where Files Go

```
Skills    →  .claude/skills/   .opencode/skills/   .agents/skills/
AGENTS.md →  project root
CLAUDE.md →  project root
agents/*  →  project root
commands/ →  project root
```

`.agents/skills/` is always created as a fallback. Other tool directories are used only if they already exist.

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

**Sources** — Git repos or local dirs containing agent files. Team + personal + community.

**Tool directories** — Skills mount into `.claude/skills/`, `.opencode/skills/`, etc. where AI tools discover them natively.

**Git exclude** — Mounted files are hidden from git by default. Use `csaw show`/`hide` to control visibility. Files in already-gitignored directories (like `.claude/`) need no extra exclusion.
