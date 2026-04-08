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
- `internal/linkmode`: cross-platform linking (symlinks with hardlink fallback on Windows)
- `internal/registry`: registry scaffolding (`csaw init`)
- `internal/pinning`: per-project source pinning to branches/tags
- `internal/fork`: file forking between sources
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
- source catalogs, generalized push (any source), and registry scaffolding (`csaw init`)
- profile parsing, inheritance, and cross-source resolution
- mount selection, `.csawignore`, priority-based conflict resolution, auto-unmount, and restore snapshots
- workspace stash, exclude helpers, current mount state, and restore state
- mounted-link discovery, drift inspection, and repair
- cross-platform linking (symlinks with hardlink fallback on Windows)
- per-project source pinning (`csaw pin`/`unpin`) via git worktrees
- file forking between sources (`csaw fork`)
- inspect and status summaries
- repository validation tests

Deferred:

- richer layered provenance in inspect output
- structured context switching (MCP, model, env composition)
- trust model for third-party sources

## Design Rules

- Follow the product docs and architecture before optimizing internals.
- Prefer real filesystem behavior in tests over in-memory abstractions.
- Keep repo docs and agent workflow materials versioned alongside code.
- Treat external registries as explicit sources; nothing should be silently injected.
