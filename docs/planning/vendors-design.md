# Vendors: Design

Captured 2026-06-09. Architecture and design rationale for csaw's vendor feature: safely consuming external agent-context catalogs (skills.sh, APM packages, awesome-copilot, internal bundle manifests, any git repo) without letting upstream layouts become active mounted context.

## Problem statement

Today csaw recognizes two states for a file:

1. **In a configured source** → projects to mounted project context
2. **Not in any source** → doesn't exist as far as csaw is concerned

Both states assume the file was *authored or curated by a trusted source maintainer*. That breaks when teams want to consume external catalogs:

- **skills.sh** distributes skills via `npx skills add owner/repo` — an opaque installer that drops files into your config dirs without review
- **APM** packages are registry-resolved manifests with lockfile pinning, but consuming an APM package means buying into APM's ecosystem and runtime
- **awesome-copilot** and similar curated lists are git repos whose layout is designed for browsing, not for direct projection
- **Internal bundle manifests** are similar but org-private

Today, a csaw user who wants any of these has three bad options: run the opaque installer (no review, no audit), manually copy files into their source registry (no provenance, no drift detection), or fork the upstream into their own registry (drift detection fails after the first edit).

The vendor feature adds a third state:

3. **Vendored from upstream** → fetched, hash-locked, never projects, requires explicit `csaw vendor promote` to enter a real source

That third state is the missing primitive.

## Strategic framing

This makes csaw **complementary to (not competitive with) external catalogs**. APM is one ecosystem; csaw becomes the safe consumer of any ecosystem. Pitch line for README:

> Use any external catalog — APM packages, skills.sh, awesome-copilot, even random GitHub repos — vendor it into a locked area, review, then promote selected pieces into your own source registry. csaw doesn't lock you into anyone's package ecosystem; it makes any ecosystem safely consumable.

## Design decisions

The big decisions, confirmed with the user before implementation:

| Decision | Choice | Rationale |
|---|---|---|
| First-class concept or flag on sources? | **First-class top-level concept** | Different lifecycle (no auto-mount), different trust model, different commands. Overloading "source" would confuse the model. |
| Vendor area location? | **Registry-local: `<source>/vendor/<name>/` + `vendor.lock.yaml` in source root** | Git-trackable alongside hand-authored content. Promotion is a single git diff. Lockfile committed to maintainer's source means consumers get exact vendored state. Composes with csaw's git-native model. |
| Phase A scope? | **All four commands: add, sync, list, audit, promote (+ remove)** | User chose full workflow over staged rollout. ~1-2 weeks of work but ships the complete safety + lineage story together. |
| External source format? | **Git URLs + `gh:`/`gl:`/`bb:` shorthand only** | Reuses the v0.8.1 shorthand parser. Covers ~95% of cases (most APM packages, all skills.sh skills, awesome-copilot, internal repos are just git). APM-registry / skills.sh special handling deferred to Phase C if real demand emerges. |
| Promote scope? | **In MVP, copies file + records lineage in lockfile** | Lineage primitive same as `csaw fork`. Phase B drift detection (audit-promoted-vs-vendor) needs promote to exist first. |

## Manifest format

New top-level block in `csaw.yml`:

```yaml
csaw:
  protected:
    - AGENTS.md
    # ... existing csaw config block stays

vendors:
  awesome-copilot:
    url: https://github.com/github/awesome-copilot
    ref: main                      # optional; defaults to default branch
    include:                       # optional; glob patterns to copy from upstream into vendor/
      - "agents/**/*.md"
      - "skills/**/SKILL.md"
    exclude:                       # optional
      - "**/draft/**"
  skills-sh-foo:
    url: gh:owner/skill-foo        # shorthand supported
    ref: v1.2.0

default:
  description: ...
  include:
    # ... profiles unchanged
```

Shape rationale:
- `url` + `ref` mirror Source struct fields (consistency)
- `include`/`exclude` glob filters keep the vendor area scoped (many upstream repos contain docs, CI configs, etc. csaw doesn't care about)
- Defaults: no filter means vendor everything; works for small focused repos

## Storage layout

In the source registry's working tree:

```
<registry>/
├── csaw.yml               # declares vendors:
├── vendor.lock.yaml       # per-file hashes, per-vendor ref state, promotion lineage
├── vendor/                # git-tracked
│   ├── awesome-copilot/
│   │   ├── .csaw-vendor-meta.yaml    # url, ref synced, timestamp
│   │   ├── agents/
│   │   │   └── code-reviewer.md
│   │   └── skills/
│   │       └── pr-review/
│   │           └── SKILL.md
│   └── skills-sh-foo/
│       └── ...
├── agents/                # hand-authored or promoted from vendor/
├── rules/
├── skills/
└── ...
```

The vendor working tree is git-tracked because csaw's whole model is git-as-source-of-truth. Consumers of the registry get bit-identical vendored content with cryptographic integrity.

## Lockfile schema

`vendor.lock.yaml` lives at the registry root, gets committed:

```yaml
# csaw vendor.lock.yaml — generated by csaw vendor sync
# Do not edit by hand. Run csaw vendor sync to update.
version: 1

vendors:
  awesome-copilot:
    url: https://github.com/github/awesome-copilot
    ref_requested: main
    ref_resolved: a1b2c3d...    # exact SHA at sync time
    synced_at: 2026-06-09T12:00:00Z
    files:
      "agents/code-reviewer.md":
        sha256: deadbeef...
        size: 1234
      "skills/pr-review/SKILL.md":
        sha256: cafebabe...
        size: 2345

promotions:
  - vendor: awesome-copilot
    vendor_path: "agents/code-reviewer.md"
    promoted_to: "agents/code-reviewer.md"
    promoted_at: 2026-06-09T12:05:00Z
    vendor_sha256_at_promote: deadbeef...    # for promotion-drift detection
```

Two top-level blocks:
- `vendors:` — per-vendor sync state (ref, file inventory + hashes)
- `promotions:` — append-only log of `csaw vendor promote` actions

The `vendor_sha256_at_promote` is the lineage anchor — drift detection compares this against (a) current vendored file's hash (did vendor change since promotion?) and (b) current promoted file's hash (did maintainer edit after promote?).

## CLI surface

All under `csaw vendor`:

```bash
csaw vendor add <name> <url-or-shorthand> [--ref <ref>] [--include <glob>] [--exclude <glob>]
csaw vendor list                          # show declared vendors + sync state
csaw vendor sync [<name>]                 # all vendors or one; updates vendor/ + vendor.lock.yaml
csaw vendor audit                         # drift detection (3 types below)
csaw vendor promote <vendor>/<path> --into <dest-path>
csaw vendor remove <name>                 # remove from manifest + delete vendor/<name>/
```

Behaviors:

- `add` writes to `csaw.yml` (alphabetical insertion under `vendors:`). Doesn't sync — user runs `sync` explicitly.
- `sync` clones to a temp area, walks files matching include/exclude, copies into `vendor/<name>/`, writes/updates `vendor.lock.yaml`. Idempotent: re-running on a clean tree is a no-op.
- `list` reads `vendor.lock.yaml` and prints: name, url, ref_resolved, file count, time since last sync.
- `audit` checks three drift types:
  1. **Vendor-local drift**: file in `vendor/<name>/` whose current SHA differs from `vendor.lock.yaml`. Means someone edited inside the vendor area.
  2. **Upstream drift**: `git ls-remote` for vendor's ref returns a SHA different from `ref_resolved` in lockfile. Means upstream has new commits.
  3. **Promotion drift**: for each entry in `promotions:`, compare current vendor file SHA vs `vendor_sha256_at_promote`, AND current promoted file SHA vs vendor file SHA. Surfaces either "vendor moved since promote" or "maintainer edited promoted file."
- `promote` copies `vendor/<name>/<path>` to the destination in the source's working tree, records lineage. Refuses to overwrite without `--force`. Doesn't auto-mount (existing csaw mount/profile flow does that).
- `remove` removes from `csaw.yml`, deletes `vendor/<name>/` directory, removes lockfile entry. Promotions are preserved (those files are now hand-authored — same as if they'd been originally hand-written).

## Vendored content NEVER mounts

This is the load-bearing safety property. csaw's mount/projection pipeline must EXCLUDE `vendor/**` from all source enumeration. The contract:

> A file under `vendor/` projects to a mounted project ONLY if it has been promoted (i.e., copied into a regular csaw kind directory like `skills/`, `agents/`, etc.).

Implementation: add `vendor/**` to the implicit ignore list during `EnumerateSourceEntries`. Document this in the source loading code so future contributors don't accidentally "fix" the exclusion.

## Security considerations

The whole point of vendors is to consume untrusted content safely. Worth being explicit:

- **Content integrity:** lockfile pins SHA-256 per file. `audit` catches tampering.
- **Provenance:** `vendor.lock.yaml` records the upstream URL and resolved ref. Reproducible.
- **No code execution:** `sync` is a git clone + file copy. No upstream scripts run. No `npm install`, no installer hooks, no postinstall.
- **Promotion is opt-in:** vendored files don't enter active context without explicit human action.
- **License visibility:** Phase A doesn't surface licenses; Phase C item.
- **Hidden Unicode (per APM):** Phase A doesn't scan for invisible characters; could be added to audit later if it becomes a real concern.

## What this design declines

Pre-stating non-goals so they don't creep in:

- Not a runtime fetcher — vendors sync to disk; no on-mount fetching.
- Not a package manager — no transitive resolution, no version constraints, no semver. Pin a ref, sync, done.
- Not a marketplace — csaw doesn't host or curate vendors. The user adds whatever git URL they trust.
- Not APM-registry-aware in MVP — just git URLs. APM-resolver as Phase C if there's demand.
- Not script-execution-aware — if upstream has a `postinstall.sh`, csaw doesn't run it. Read-only consumption only.
- Not multi-vendor-per-file — each file in `vendor/` belongs to exactly one vendor.

## Implementation plan

Internal package: `internal/vendor/`

Files:
- `types.go` — Vendor, VendorEntry, Lockfile, Promotion types
- `manifest.go` — read/write the `vendors:` block in csaw.yml
- `lockfile.go` — read/write `vendor.lock.yaml`
- `sync.go` — clone, walk, hash, write
- `audit.go` — three drift types
- `promote.go` — copy + lineage
- Each with `*_test.go`

CLI surface: `cmd/csaw/vendor.go` mirroring the `mcp.go` pattern from v0.9.0.

Mount-pipeline integration: one-line change in `internal/sources` (or `internal/mount/planner.go`) to exclude `vendor/**` from `EnumerateSourceEntries`.

Docs updates: README command reference, cheatsheet section, walkthrough scenario (vendoring awesome-copilot end-to-end), v1.0-readiness matrix row, next-up.md entry promoted from Tier 3 to "shipped."

## Open questions deferred to implementation time

Not blocking; document the decision when reached:

1. **What if a vendor's git URL requires auth?** Reuse the existing source `git credential` flow. Same auth surface as `csaw source add`.
2. **What if `vendor/<name>/` has uncommitted changes when `sync` runs?** Refuse with a clear error; user can `git stash` or commit. Same pattern as `csaw pull` with dirty source.
3. **What happens if `promote` target path already has a different file?** Refuse without `--force`. Lineage of the overwritten file goes into the promotion log as `replaced`.
4. **What if upstream removes a file we've vendored?** `sync` should remove the file from `vendor/<name>/` and surface it in the next `audit` (with a note that any promotion based on it is now orphaned).
5. **Should vendored files be `.gitignore`d in consumer projects?** No — they're maintainer-owned source content, just like hand-authored files. Consumer projects don't see `vendor/` because csaw doesn't mount it.

## Path forward

1. Implement `internal/vendor/` package layer by layer with tests
2. Wire CLI subcommands in `cmd/csaw/vendor.go`
3. Add `vendor/**` exclusion to source enumeration
4. Update docs (README, cheatsheet, walkthrough scenario, v1.0-readiness)
5. Ship as v0.10.0
