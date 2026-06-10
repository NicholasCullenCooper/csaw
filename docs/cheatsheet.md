# csaw Cheat Sheet

## Setup

```bash
# Install
uv tool install csaw

# Add a team source (auto-clones)
csaw source add team git@github.com:org/ai-config.git
csaw source add team gh:org/ai-config            # shorthand
csaw source add team gh:org/ai-config#v1.2.0     # shorthand + pin
csaw profile list

# Or create your own registry
csaw init ~/my-ai-config                              # default scaffold
csaw init ~/my-ai-config --preset team-go             # curated starter (see --list-presets)
csaw source add personal ~/my-ai-config --priority 10
```

## Mount

```bash
csaw mount                                       # interactive profile picker
csaw use team/backend                            # activate a named profile
csaw mount profile team/backend                  # same operation, mechanical form
csaw mount paths agents/go.md                    # advanced: mount specific files
csaw use team/core --force                       # overwrite conflicts
csaw mount --restore                             # re-mount previous selection
csaw use team/extra --keep                       # add to existing mount (don't replace)
csaw use team/backend --kind agents              # only mount agent definitions
csaw use team/full --kind agents,skills,rules    # subset of kinds
```

Activating a profile **replaces** the previous mount by default. Use `--keep`
to add on top. Use `--kind` to restrict by kind (`agents`, `skills`, `rules`,
`mcp`, `instructions`). `csaw mount paths ...` is the raw path/glob escape hatch
for one-off selection; normal workflows should live in named profiles.

## Unmount

```bash
csaw unmount                            # unmount everything
csaw unmount agents/go.md               # unmount specific files
```

## Inspect

```bash
csaw profile list                       # list available work modes
csaw profile show team/backend          # show resolved includes/excludes
csaw inspect                            # full state overview
csaw inspect --source team              # browse a source
csaw audit --init                       # create .csaw/policy.yml
csaw audit                              # verify active context against policy
csaw audit --strict                     # fail on warnings and errors
csaw audit --json                       # machine-readable report
csaw status                             # quick summary + uncommitted edits in source checkouts
csaw check                              # find broken links and protected drift
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
csaw source add team gh:org/repo        # host shorthand (gh, gl, bb)
csaw source add team gh:org/repo#v1     # shorthand + default ref (Source.Ref)
csaw source remove name                 # remove a source
csaw source clone team ~/Developer/team # clone remote locally to contribute
csaw pull                               # update all remote sources
csaw pull team                          # update one source
csaw push team -m "updated rules"       # push source changes
csaw push                               # auto-detect dirty source and push
```

`Source.Ref` (set via shorthand `#ref`) is the source's default ref across all projects. Per-project pins (`csaw pin team@ref`) still take precedence.

## Pin a Branch (Per-Project)

```bash
csaw pin team@feature/new-rules         # pin THIS project to a branch
csaw pull team                          # pulls that branch
csaw use team/backend                   # mounts from the branch
csaw unpin team                         # back to source default (Source.Ref or main)
```

## Fork a File

```bash
csaw fork team/agents/base.md --into personal  # copy for personal editing
```

## Vendor External Catalogs Safely

For consuming external agent-context catalogs (skills.sh, APM packages, awesome-copilot, internal bundles, any git repo) without letting upstream layouts become active mounted context.

```bash
csaw vendor add awesome-copilot gh:github/awesome-copilot --include "agents/**"
csaw vendor sync                              # fetch declared vendors into vendor/<name>/ with hashes
csaw vendor list                              # show declared vendors + sync state
csaw vendor audit                             # detect drift: vendor-local, upstream, promotion
csaw vendor promote awesome-copilot/agents/reviewer.md --into agents/reviewer.md
csaw vendor remove awesome-copilot
```

Nothing under `vendor/` ever projects to mounted context. Only files copied into real kind directories (`skills/`, `agents/`, `rules/`, etc.) via `promote` (or by hand authoring) will mount. The bounded `vendor/` area is git-tracked; `vendor.lock.yaml` records per-file SHAs and promotion lineage.

## Merge MCP Into a Shared-Config Tool (Codex)

For tools whose MCP lives inside a shared file (Codex's `.codex/config.toml`, where user owns model preferences, providers, etc.), csaw appends a bounded section instead of symlinking.

```bash
csaw mcp sync codex --from team           # dry-run: show what would change
csaw mcp sync codex --from team --apply   # write the merge
csaw mcp sync codex --remove              # roll back (refuses if you edited inside the section)
```

Source fragment lives at `<source>/mcp/codex.toml` — a TOML file containing `[mcp_servers.<name>]` tables. csaw refuses literal secrets in sensitive-named fields; use Codex's `env_vars = ["VAR"]` pattern.

## Promote an Experimental Skill

```bash
csaw promote personal/skills/experimental/debug-strategy
# moves skills/experimental/debug-strategy/ → skills/debug-strategy/
csaw push personal -m "promote debug-strategy"
```

Any path segment named `experimental/` is hidden by built-in convention — applies to `rules/experimental/`, `agents/experimental/`, `hooks/experimental/` too. Test in-progress files alongside stable with `--include-experimental`. (`.csawignore` is separate; `--include-ignored` mounts those.)

## Source Priority

When two sources provide the same file, higher priority wins:

```bash
csaw source add personal ~/my-config --priority 10  # wins over default (0)
csaw source add team git@github.com:org/config.git   # priority 0 (default)
```

## Create a Registry

```bash
csaw init ~/my-ai-config                          # default scaffold (csaw.yml, agents/, skills/)
csaw init ~/my-ai-config --name myteam            # custom name
csaw init ~/my-ai-config --preset solo-engineer   # curated personal scaffold
csaw init ~/team-config  --preset team-go         # protected AGENTS.md + Go rules + code-review agent
csaw init ~/team-config  --preset team-frontend   # protected AGENTS.md + TS/React rules + a11y
csaw init --list-presets                          # show available presets + descriptions
csaw init ~/my-ai-config --adopt                  # import existing AI config from current project
```

`--preset` and `--adopt` are mutually exclusive.

## Repair

```bash
csaw check                              # detect link drift and protected content drift
csaw update                             # repair broken links
```

## Where Files Go

| Kind | Registry path | Projects to |
|---|---|---|
| Instructions | `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.goosehints` | Project root |
| Rules | `rules/*.md` | `.claude/rules/`, `.cursor/rules/`, `.github/instructions/*.instructions.md` (Copilot, suffix auto-applied) |
| Agents | `agents/*.md` | `.claude/agents/`, `.opencode/agents/`, `.github/agents/*.agent.md` (Copilot, suffix auto-applied) |
| Skills | `skills/*/SKILL.md` | `.claude/skills/`, `.opencode/skills/`, `.codex/skills/`, `.agents/skills/` |
| MCP | `mcp/*.json` | `.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` |
| Hooks | `hooks/*` | `.claude/hooks/` |
| Ignore | `ignore/*` | `.cursorignore`, `.cody/ignore`, `.aiderignore`, `.tongyiignore` |

`.agents/skills/` is always created as a fallback. Other tool directories are used only if they already exist in the project or are configured via `csaw config set tools claude,cursor`.

Files at unrecognized registry paths are mounted at the same path in the project (no per-tool projection).

**GitHub Copilot** projections get automatic suffix rewriting — you write `rules/security.md` in your registry; Copilot sees `.github/instructions/security.instructions.md`. Like every csaw projection, these files are hidden from git by default. If your team wants them committed for PR review, opt in with `csaw show .github/instructions/*` (or per-file).

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

**Mount, not install** — Symlinks from a source. Reversible. Your repo stays clean.

**Profiles** — Named file selections with glob patterns in a source's `csaw.yml`. Can inherit from each other via `extends:`.

**Sources** — Git repos or local dirs containing AI config. Canonical vertical stack: company, department, team, personal. Horizontal additions: client (consultants), community, role-based (e.g. staff-eng). Add as many as you need. One **maintainer** per shared source; everyone else is a passive **consumer** running `csaw use` and `csaw pull`.

**Priority** — When sources overlap, higher priority wins. Set with `--priority` on `source add`.

**Protected files** — A source can mark files as `protected:` in its `csaw.yml`. Protected files bypass priority (always win) and refuse `csaw fork`. The mechanism behind layered governance (company, department, team, or client mandates).

**Project policy** — A project can declare `.csaw/policy.yml` with `required_sources`, `blocked_sources`, `required_kinds`, `blocked_kinds`, and `blocked_paths`. Use `csaw audit --init` to create a starter policy. `required_sources` can require a source name, configured URL, and project pin. `csaw audit` checks the active mounted context against that policy. `--strict` fails on warnings, including a missing policy.

**Pinning** — Lock a source to a branch/tag per project with `csaw pin`. Uses git worktrees so other projects stay on the default branch.

**Fork** — Copy a file from one source into another for personal editing with `csaw fork`. The original is untouched.

**Promote** — Move a skill from `skills/experimental/` to `skills/` in a source so it mounts by default.

**Kinds** — csaw classifies registry files as one of seven kinds: instructions, rules, agents, skills, mcp, hooks, ignore. Each has its own projection target. Filter with `csaw use team/backend --kind agents,skills`. csaw does **not** project settings (credentials) or memory (session state); see `docs/planning/projection-roadmap.md`.

**Tool directories** — Each kind projects into the right per-tool directory (`.claude/agents/`, `.cursor/rules/`, etc.) where AI tools discover files natively.

**Git exclude** — Mounted files are hidden from git by default. Use `csaw show`/`hide` to control visibility. Files in already-gitignored directories (like `.claude/`) need no extra exclusion.
