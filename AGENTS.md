# AGENTS.md

## Purpose

`csaw` is a Go CLI that mounts AI workspace files from one or more registries into a project using symlinks, local git excludes, and reversible stash or restore behavior.

This file is the agent map, not the whole manual. Read the linked docs before making non-trivial changes.

## Current Milestone

The repo is in bootstrap plus Phase 1 foundation work:

- establish the repo as the system of record for agents
- keep the public CLI surface aligned with the product overview and architecture docs
- build the mount engine incrementally from `dotghost` behavior, not by line-by-line porting

The active plan is [`docs/exec-plans/active/bootstrap-foundation.md`](docs/exec-plans/active/bootstrap-foundation.md).

## Read First

1. [`README.md`](README.md)
2. [`ARCHITECTURE.md`](ARCHITECTURE.md)
3. [`docs/product/overview.md`](docs/product/overview.md)
4. [`docs/reference/project-management.md`](docs/reference/project-management.md)
5. [`docs/reference/public-repo-policy.md`](docs/reference/public-repo-policy.md)

Read these next when relevant:

- `dotghost` parity work: [`docs/reference/dotghost-reference.md`](docs/reference/dotghost-reference.md)
- repo decisions: [`docs/decisions/0001-repo-as-system-of-record.md`](docs/decisions/0001-repo-as-system-of-record.md)
- contributor workflow: [`CONTRIBUTING.md`](CONTRIBUTING.md)

## Source Of Truth

- Public product context: [`docs/product/overview.md`](docs/product/overview.md)
- Active multi-step implementation work: [`docs/exec-plans/active/`](docs/exec-plans/active/)
- Completed plans and historical context: [`docs/exec-plans/completed/`](docs/exec-plans/completed/)
- Architectural direction and package layout: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Technical debt that should not block current delivery: [`docs/tech-debt-tracker.md`](docs/tech-debt-tracker.md)
- Backlog and roadmap: GitHub Issues and the GitHub Project described in [`docs/reference/project-management.md`](docs/reference/project-management.md)

Do not create a repo-root `TODO.md` backlog.

## Repo Layout

- `cmd/csaw/`: CLI entrypoint and command wiring
- `internal/runtime/`: shared constants, paths, normalization helpers
- `internal/git/`: git execution interface
- `internal/sources/`: global source config and registry checkout behavior
- `internal/profiles/`: `csaw.yml` parsing and inheritance
- `internal/mount/`: mount selection and glob planning
- `internal/workspace/`: stash state, exclude file management, mounted-link discovery
- `internal/drift/`: mounted link health inspection
- `internal/inspect/`: summary and markdown preview rendering
- `internal/docs/`: repository validation helpers and tests
- `docs/`: product, planning, decisions, and reference docs
- `skills/`: repo-local skills using the `SKILL.md` convention

## Working Rules

- Keep `AGENTS.md` short. Put durable detail in linked docs.
- Prefer behavior-level changes over tool-specific hacks.
- Treat `dotghost` as behavioral reference only.
- Keep cross-tool compatibility. `AGENTS.md` is the primary instruction surface.
- Use issue templates, exec plans, and the tech debt tracker instead of ad hoc notes.
- If you change workflows, architecture, or validation commands, update the docs in the same change.
- Do not commit secrets, credentials, private keys, or local workstation paths.
- Use generic examples in docs and issues. Prefer placeholders like `git@example.com:org/repo.git` over personal or machine-specific paths.

## Git Workflow

### Branching

All work happens on feature branches, never directly on `main`. Feature branches use
descriptive names with a prefix:

```
feat/player-profiles
feat/data-layer-duckdb
fix/shot-chart-dimensions
```

### Merging to Main

1. **Ensure checks pass** (lint, typecheck, tests)
2. **Create a pull request**
3. **Stop and tell the user** — give them the PR URL and wait for them to merge it
4. When the user confirms the PR is merged:
   - `git checkout main && git pull origin main`
   - `git push origin --delete feat/branch-name`
   - `git branch -d feat/branch-name`

Never merge locally — always through a PR so the merge is tracked. Never merge the PR yourself —
the user reviews and merges.

### Starting a New Phase

1. `git checkout main && git pull`
2. `git checkout -b feat/description`
3. `git push -u origin feat/description` (on first commit)


## Validation Commands

Run the smallest relevant set first, then the full baseline before closing work:

```bash
make fmt
make test
make vet
make docs-check
go run ./cmd/csaw --help
```

Useful package-level test targets:

```bash
go test ./internal/profiles ./internal/mount ./internal/workspace ./internal/docs
```

## Skills

Use the repo-local skills when the task matches:

- [`skills/go-cli-bootstrap/SKILL.md`](skills/go-cli-bootstrap/SKILL.md)
- [`skills/mount-engine/SKILL.md`](skills/mount-engine/SKILL.md)
- [`skills/dotghost-parity/SKILL.md`](skills/dotghost-parity/SKILL.md)
- [`skills/exec-plan-maintenance/SKILL.md`](skills/exec-plan-maintenance/SKILL.md)
- [`skills/doc-gardening/SKILL.md`](skills/doc-gardening/SKILL.md)

## Hard Constraints

- Preserve the Phase 1 public command surface from the brief.
- Preserve naming from the brief: `~/.csaw`, `csaw.yml`, `.csawignore`, `.csaw-stash`, `# csaw-managed`.
- Prefer stdlib unless the brief explicitly justifies a dependency.
- Add tests for profile resolution, glob behavior, path normalization, and workspace-state logic.
- Do not silently invent new configuration formats.
