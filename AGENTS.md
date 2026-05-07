# AGENTS.md

## Purpose

`csaw` is a Go CLI that mounts AI workspace files from one or more registries into a project using symlinks, local git excludes, and reversible stash or restore behavior.

This file is the agent map, not the whole manual. Read the linked docs before making non-trivial changes.

## Current Milestone

The CLI is post-v0.2 with core mount engine, multi-source composition (priority + protected files + fork + promote + pin), kind classification (instructions/rules/agents/skills/mcp) with `--kind` filtering, and distribution (PyPI, Homebrew, Scoop). Positioning is multi-source AI workspace governance for staff engineers, contractors, and teams with mandated AI policy. Active work focuses on first-class agent treatment, security audit (`csaw audit`), and continued cross-platform/UX polish.

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
- `internal/sources/`: source config, catalog, push, pull, worktree checkout
- `internal/profiles/`: `csaw.yml` parsing and inheritance
- `internal/mount/`: mount selection, glob planning, priority resolution, tool projection
- `internal/workspace/`: stash state, exclude file management, mounted-link discovery
- `internal/drift/`: mounted link health inspection
- `internal/linkmode/`: cross-platform linking (symlinks with hardlink fallback on Windows)
- `internal/registry/`: registry scaffolding (`csaw init`, `--adopt`)
- `internal/pinning/`: per-project source pinning to branches/tags
- `internal/fork/`: file forking between sources
- `internal/tui/`: bubbletea wizard, profile picker, multi-select, styled panels
- `internal/inspect/`: summary and markdown preview rendering
- `internal/output/`: terminal styling helpers
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

Solo development commits directly to `main`. For collaborative work or risky changes, use feature branches with PRs.

### Before committing

1. `gofmt -w .` — fix formatting
2. `go test ./...` — all tests pass
3. `go vet ./...` — no warnings
4. Write a clear commit message explaining **why**, not just what


## Testing

### When to write tests

- **Every new function with logic** gets a test. If it has branching, error cases, or non-trivial behavior, test it.
- **Every bug fix** gets a regression test that fails before the fix and passes after.
- **New commands** need at least one test verifying the happy path. Interactive TUI flows are hard to test — at minimum, test the underlying logic functions they call.
- **Refactors** must not break existing tests. If you change a function signature, update its callers AND its tests.

### When tests must pass

- **Before every commit.** Run `go test ./...` and verify all tests pass. Do not commit with failing tests.
- **Before every tag/release.** Run the full validation suite below. Do not tag if anything fails.
- **In CI.** The CI workflow runs tests on Linux, macOS, and Windows. All three must pass.

### Test style

- Use real filesystems (`t.TempDir()`), not in-memory abstractions. Symlinks are a core mechanism and must be tested against real OS behavior.
- Use `recordingGit` (in `internal/sources/catalog_test.go`) to mock git operations without hitting the network.
- Table-driven tests for functions with multiple input/output cases.
- No assertion libraries — use stdlib `testing` with `t.Fatalf`/`t.Errorf`.

## Validation Commands

Run the smallest relevant set first, then the full baseline before closing work:

```bash
gofmt -l .          # must produce no output
go vet ./...        # must pass
go test ./...       # must pass
go build ./...      # must compile
```

Package-level test targets for faster iteration:

```bash
go test ./internal/mount ./internal/sources ./internal/registry
```

## Skills

Use the repo-local skills when the task matches:

- [`skills/go-cli-bootstrap/SKILL.md`](skills/go-cli-bootstrap/SKILL.md)
- [`skills/mount-engine/SKILL.md`](skills/mount-engine/SKILL.md)
- [`skills/dotghost-parity/SKILL.md`](skills/dotghost-parity/SKILL.md)
- [`skills/exec-plan-maintenance/SKILL.md`](skills/exec-plan-maintenance/SKILL.md)
- [`skills/doc-gardening/SKILL.md`](skills/doc-gardening/SKILL.md)

## Releasing

Releases are cut by tagging main. See [`docs/reference/distribution.md`](docs/reference/distribution.md) for full details.

When the user asks to release, or when a meaningful set of changes has landed and the user
confirms they want a release:

1. Verify main is pushed and CI has passed.
2. Find the current version: `git describe --tags --abbrev=0`
3. Determine the next version using semver:
   - **Patch** (bug fixes, small non-breaking additions)
   - **Minor** (new features, new config surface, new commands)
   - **Major** (breaking changes to CLI, config format, or mount behavior)
4. Tag with an annotated tag and a short description:
   ```bash
   git tag -a v0.X.Y -m "v0.X.Y: short description"
   git push origin v0.X.Y
   ```
5. Confirm the GitHub Actions release workflow triggered successfully.

Do not tag without the user's approval. Do not skip the version bump rationale.

## Hard Constraints

- Preserve naming conventions: `~/.csaw`, `csaw.yml`, `.csawignore`, `.csaw-stash`, `# csaw-managed`.
- Prefer stdlib unless explicitly justified. Current approved deps: cobra, lipgloss, bubbletea, bubbles, glamour, doublestar, yaml.v3.
- All new logic must have tests. All tests must pass before commit.
- Do not silently invent new configuration formats.
- Cross-platform: all code must work on Linux, macOS, and Windows. Use `internal/linkmode` for symlink/hardlink abstraction.
