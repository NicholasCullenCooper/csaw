# Architecture

## Intent

`csaw` is structured as a small Go CLI with behavior-oriented internal packages. The goal is to make the public command surface stable while the mount engine evolves under clear package boundaries.

## Package Map

- `cmd/csaw`: Cobra wiring for the public CLI surface
- `internal/runtime`: filesystem locations, platform-aware normalization, and repo discovery
- `internal/git`: small interface around `git` shell execution for testability
- `internal/sources`: global config, source registry bookkeeping, and push operations
- `internal/profiles`: `csaw.yml` loading, validation, and inheritance
- `internal/mount`: include or exclude selection, glob matching, and priority-based conflict resolution
- `internal/workspace`: `.git/info/exclude`, `.csaw-stash`, and mounted link inspection
- `internal/drift`: health classification for mounted links
- `internal/linkmode`: cross-platform linking for symlink mode (symlinks with hardlink fallback on Windows)
- `internal/lock`: `.csaw/lock.yml` (per-project record of mounted sources' commits and file hashes)
- `internal/hooks`: merges a registry `hooks/hooks.json` into Claude Code settings and Codex `hooks.json` as csaw-owned matcher groups
- `internal/export`: writes a source or profile as a Claude Code plugin/marketplace or an Agent Plugins 1.0 directory
- `internal/scan`: deterministic content checks over vendored files and the external scanner seam
- `internal/skills`: Agent Skills name and frontmatter validation shared by mount, audit, and export
- `internal/registry`: registry scaffolding (`csaw init`)
- `internal/pinning`: per-project source pinning to branches/tags
- `internal/fork`: file forking between sources
- `internal/audit`: project policy parsing and active-context audit checks
- `internal/inspect`: summary rendering and markdown preview helpers
- `internal/output`: terminal styling helpers shared across commands
- `internal/docs`: repository validation helpers used by CI and local checks

## Interfaces

The bootstrap locks in these seams early so Phase 1 and Phase 2 work can expand without large rewrites:

- `git.Git`: shell-backed git execution
- `profiles.Resolver`: profile resolution with inheritance
- `mount.Planner`: mount selection planning from includes and excludes
- `workspace.StateStore`: stash manifest persistence
- `runtime.PathNormalizer`: path comparison and normalization behavior

## Current Implementation State

Implemented now:

- CLI command surface and command wiring
- source config persistence with priority field
- source catalogs, generalized push (any source), and registry scaffolding (`csaw init`, `--adopt`)
- profile parsing, inheritance, source-level policy (protected paths), and cross-source resolution
- mount selection, `.csawignore`, priority-based conflict resolution, protected-path enforcement, auto-unmount, and restore snapshots
- kind classification (instructions, rules, agents, skills, mcp) with `--kind` filtering and per-kind grouping in `inspect`
- workspace stash, exclude helpers (`csaw show`/`hide`), current mount state, and restore state
- mounted-link discovery, drift inspection, and repair
- copy mode by default: files are placed as hash-tracked copies, drift is classified as stale, edited locally, or both, `csaw update` refreshes what it can, and unmount preserves edits
- symlink mode as an opt-in with cross-platform linking (symlinks with hardlink fallback on Windows)
- `.csaw/lock.yml` written on every mount and update, verified by `csaw audit`, enforced by `csaw use --frozen`
- hooks as a merged kind: `hooks/hooks.json` groups merged into `.claude/settings.local.json` and `.codex/hooks.json`, scripts copied with commands rewritten, only csaw's groups removed on unmount
- `csaw export plugin|marketplace` for Claude Code and Agent Plugins channels; vendor content checks and scanner seam; skill validation at mount, audit, and export
- per-project source pinning (`csaw pin`/`unpin`) via git worktrees
- file forking between sources (`csaw fork`)
- skill lifecycle promotion (`csaw promote` from `skills/experimental/` to `skills/`)
- inspect and status summaries (sources with priority, pins, protected counts, mounted files grouped by kind)
- local context audit (`csaw audit`) with `.csaw/policy.yml`, required sources, blocked sources, required kinds, strict mode, and JSON output
- tool projection for Claude Code and Codex (the supported tools): each declares the directories it reads, and csaw writes each kind into the smallest set of directories that serves every configured tool
- a generated instruction bridge (`CLAUDE.md` importing `AGENTS.md`) tracked in mount state with a content hash; user edits are preserved on unmount
- whole-skill-directory projection (`SKILL.md` with its `references/`, `scripts/`, `assets/`)
- MCP config projection (`mcp/claude-code.json` → `.mcp.json`, etc.)
- repository validation tests

Deferred:

- richer layered provenance in inspect output (per-value source attribution beyond per-file)
- structured context switching (MCP composition with merge semantics, env vars, model preferences)
- trust model for third-party sources (signing, content-hash verification of protected files)
- content-security scanners for prompt-injection and exfiltration heuristics

## Design Rules

- Follow the product docs and architecture before optimizing internals.
- Prefer real filesystem behavior in tests over in-memory abstractions.
- Keep repo docs and agent workflow materials versioned alongside code.
- Treat external registries as explicit sources; nothing should be silently injected.
