# Architecture

## Intent

`csaw` is structured as a small Go CLI with behavior-oriented internal packages. The goal is to make the public command surface stable while the mount engine evolves under clear package boundaries.

## Package Map

- `cmd/csaw`: Cobra wiring for the public CLI surface
- `internal/runtime`: filesystem locations, platform-aware normalization, and repo discovery
- `internal/git`: small interface around `git` shell execution for testability
- `internal/sources`: global config and source registry bookkeeping
- `internal/profiles`: `csaw.yml` loading, validation, and inheritance
- `internal/mount`: include or exclude selection and glob matching
- `internal/workspace`: `.git/info/exclude`, `.csaw-stash`, and mounted symlink inspection
- `internal/drift`: health classification for mounted links
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
- source config persistence
- source catalogs and personal-registry push
- profile parsing, inheritance, and cross-source resolution
- mount selection, `.csawignore`, duplicate-target detection, and restore snapshots
- workspace stash, exclude helpers, current mount state, and restore state
- mounted-link discovery, drift inspection, and repair
- inspect and status summaries
- repository validation tests

Deferred in the active plan:

- richer layered provenance in inspect output
- clearer UX for source collisions and other ambiguous multi-source selections
- structured context switching

## Design Rules

- Follow the product docs and architecture before optimizing internals.
- Prefer real filesystem behavior in tests over in-memory abstractions.
- Keep repo docs and agent workflow materials versioned alongside code.
- Treat external registries as explicit sources; nothing should be silently injected.
