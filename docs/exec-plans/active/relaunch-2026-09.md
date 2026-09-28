# Relaunch plan: projection correctness, copy mode, hooks merge, export

Captured 2026-09-16 from the September audit; scope narrowed 2026-09-28. Four releases, each independently shippable. Phase 0 is a bug-fix release; Phases 1 to 3 each change the product model in one deliberate way.

## Scope decision (2026-09-28)

**csaw designs for, tests, and documents two tools: Claude Code and Codex (OpenAI / ChatGPT).** The maintainer uses both and nothing else. Two verified surfaces keep every later phase small: two instruction files, two skills directories, two hooks targets, two plugin formats. Research on Cursor, Copilot, OpenCode, Gemini CLI, Antigravity, and goose stays in `docs/reference/tool-projection.json` with `csaw_in_code: false`; the reads-table model makes any of them one registry entry to add back, plus the filename-suffix feature that was removed with Cursor and Copilot.

What the two tools read (verified 2026-09-14/15):

| Kind | Claude Code | Codex |
|---|---|---|
| instructions | `CLAUDE.md` (not `AGENTS.md`); `@path` imports | `AGENTS.md`, nested, 32 KiB cap, `AGENTS.override.md` |
| rules | `.claude/rules/**/*.md`, optional `paths:` frontmatter; symlinks outside the working directory are external imports and do not load | none (use `AGENTS.md`) |
| agents | `.claude/agents/**/*.md` | `[agents.*]` tables in `config.toml`, not files |
| skills | `.claude/skills/<name>/` (symlinked folders read; does not read `.agents/`) | `.agents/skills/<name>/` from cwd up to repo root; `agents/openai.yaml` sidecar optional |
| hooks | settings JSON (`.claude/settings.json`, `.claude/settings.local.json`, user, managed); no directory scanned | `.codex/hooks.json` / `~/.codex/hooks.json`, same JSON shape; hooks trusted by hash |
| mcp | `.mcp.json` (whole file) | `[mcp_servers.*]` in `.codex/config.toml` (project, trusted) or `~/.codex/config.toml` |
| plugins | `.claude-plugin/plugin.json` + `marketplace.json` | Agent Plugins 1.0 `plugin.json` + `mcp.json` (+ legacy `.codex-plugin/plugin.json`); marketplaces at `.agents/plugins/marketplace.json` |

Evidence the plan rests on:

- Claude Code reads `CLAUDE.md`, not `AGENTS.md`. Before v0.11.0 csaw never wrote a `CLAUDE.md`, so mounted instructions never reached Claude Code.
- Claude Code treats a `.claude/rules/` symlink whose target is outside the working directory as an external import. It does not load until approval, approval is only prompted by an `@path` import, and path-scoped rules never load through such links. Every csaw rule symlink points into `~/.csaw`.
- Claude Code never scans `.claude/hooks/`; hooks live in settings files. Codex hooks live in `.codex/hooks.json` with the same shape.
- Codex reads skills from `.agents/skills`, not `.codex/skills`.
- Before v0.11.0 a skill's `references/` and `scripts/` files were mounted at `<project>/skills/<name>/`, not beside the projected `SKILL.md`.
- Agent Plugins 1.0 (Aug 2026) is the cross-vendor package format Codex and ChatGPT consume; Claude Code keeps its own plugin and marketplace format. Both are export targets.

## Summary

| Phase | Version | Change | Size |
|---|---|---|---|
| 0 | v0.11.0 | Projection correctness: reads table with minimal-set projection, `CLAUDE.md` bridge, whole-skill mounting, registry fixes, honest docs. No mount-model change. **Implemented in the working tree.** | done |
| 1 | v0.12.0 | Copy mode becomes the default mount mechanism, with per-file ownership hashes, a project lockfile, and a real `diff`. Symlink stays as opt-in. | 1 to 2 weeks |
| 2 | v0.13.0 | Hooks become a real kind: one `hooks/hooks.json` merged into Claude Code settings and Codex `.codex/hooks.json`, scripts copied beside them. Generalizes the Codex MCP merge engine to JSON. | 1 week |
| 3 | v0.14.0 | Export a registry as a Claude Code plugin/marketplace or an Agent Plugins directory. Vendor sync runs cheap content checks and an optional external scanner. Skill validation. | 1 week |

Cross-cutting: retire the stale queue items, reposition the README around the contractor and multi-org persona, split `cmd/csaw/root.go`, and dogfood on this machine before tagging v0.12.0.

### Decisions

1. **Copy is the default mount mode from v0.12.0.** Reasons: Claude Code's external-import rule for rules, Windows without Developer Mode, a per-project lockfile, and alignment with every peer tool. The "reversible by design" promise is kept by ownership hashes: csaw removes only bytes it wrote, stashes anything the user modified, and restores displaced originals. Symlink mode remains via `mount_mode: symlink` for users who want live edits and accept the documented Claude Code rules limitation.
2. **Project each kind to the smallest set of directories that every configured tool reads.** With two tools this is simple and stays simple: `claude,codex` puts skills in both `.claude/skills/` and `.agents/skills/` and everything else under `.claude/`. The algorithm is generic and tested with synthetic specs so a third tool later costs one entry.
3. **Instructions get a generated bridge, not an alias.** `AGENTS.md` stays canonical for Codex. When Claude Code is targeted and the registry has no `CLAUDE.md`, csaw writes a generated `CLAUDE.md` containing `@AGENTS.md`, tracked in mount state like any other entry.
4. **Hooks become a merged kind, not a file drop.** One `hooks/hooks.json` in the shared Claude/Codex shape projects into Claude Code's `.claude/settings.local.json` and Codex's `.codex/hooks.json`, with scripts copied beside them and command paths rewritten.
5. **csaw feeds the two vendors' plugin channels instead of competing with them.** `csaw export` produces a Claude Code plugin and marketplace manifest, or an Agent Plugins directory for Codex and ChatGPT.
6. **No built-in prompt-injection scanner, but a scanner seam and cheap deterministic checks.** Hidden Unicode, `!`command`` injection, `allowed-tools`, embedded hooks, and skill name validity are checked by csaw; anything smarter is delegated to a configured external command.
7. **Retire** the `csaw://` scheme, the GUI, the context vocabulary refactor, continuity and handoff, and the "wait for a user to ask" gate. With zero users, dogfooding is the gate.

## Success Criteria

Phase 0 (v0.11.0) — met in the working tree

- Fresh mount with `tools: claude,codex` produces `CLAUDE.md` (importing `AGENTS.md`), `AGENTS.md`, `.claude/rules/*.md`, `.claude/agents/*.md`, `.claude/skills/<name>/` and `.agents/skills/<name>/` with every file of the skill. Nothing is written under `.codex/`.
- A skill with `references/` and `scripts/` mounts as a complete directory under each projected skills path; nothing lands at `<project>/skills/`.
- Editing the generated `CLAUDE.md` and unmounting preserves the edit under `.csaw-stash/modified/`.
- `ToolRegistry` contains exactly `claude` and `codex`; the consistency test compares every Go read and write directory against the JSON research and logs research older than 120 days.
- `csaw inspect` labels hook scripts "scripts only; wire them in the tool's settings".

Phase 1 (v0.12.0)

- Default mount writes real files. `mount-state.json` records a SHA-256 for every entry. Unmount removes unmodified files, stashes modified ones with a warning, and restores originals. `csaw update` refreshes stale-and-unmodified files and reports modified ones.
- `.csaw/lock.yml` records resolved source commits and per-file hashes; `csaw audit` emits `lock.match` or `lock.mismatch`; `csaw use --frozen` refuses to mount a state that differs from the lock.
- Windows CI runs the full e2e suite in copy mode with no symlink privilege.
- `csaw diff <path>` shows a unified diff between the project copy and the source.
- `cmd/csaw/root.go` is split into per-command files, none over 500 lines.
- Manual pass: in a real Claude Code session, `/context` lists `CLAUDE.md` and every mounted rule, including a path-scoped one, with no external-import prompt.

Phase 2 (v0.13.0)

- `hooks/hooks.json` in a registry projects to Claude Code `.claude/settings.local.json` and Codex `.codex/hooks.json`, with scripts copied and command paths rewritten per tool. `csaw unmount` removes only csaw-owned hook entries and leaves user hooks byte-identical.
- Manual pass: Claude Code lists the hook under `/hooks`; Codex shows it in its hooks view and prompts to trust it once.

Phase 3 (v0.14.0)

- `csaw export plugin <source>/<profile> --format claude-code --out <dir>` produces `.claude-plugin/plugin.json` with skills, agents, hooks, and `.mcp.json`; `--format agent-plugins` produces a directory that validates against the Agent Plugins 1.0 schema with `skills/` and `mcp.json`. `csaw export marketplace <source> --out <dir>` produces a Claude Code `marketplace.json` listing each profile as a plugin.
- `csaw vendor sync` records check findings per file in `vendor.lock.yaml`; `csaw vendor promote` refuses files with findings unless `--force`. A configured scanner command runs when present and its exit status and report path are recorded.
- Skill directories that violate the Agent Skills name rules produce a `skill.invalid` audit finding and are skipped at mount with a warning.

## Workstreams

### Phase 0: projection correctness (v0.11.0) — implemented

Landed in the working tree on 2026-09-28: `ToolSpec` reads/writes table with `PlanTargets` set cover (`internal/mount/tools.go`); whole-skill projection; generated `CLAUDE.md` bridge tracked with a content hash, edits preserved under `.csaw-stash/modified/` on unmount, rewritten by `csaw update` when missing; `.codex/skills` dropped in favour of `.agents/skills`; JSON schema 8 with `checked_at` and per-kind path checks; `tool-projection.md` renders "What csaw writes" from the Go registry; docs, curriculum, walkthrough, cheatsheet, and README updated; registry trimmed to `claude` and `codex` with the Cursor/Copilot suffix and frontmatter machinery removed. Remaining before tagging: the manual pass in a real Claude Code and Codex session.

### Phase 1: copy mode, ownership, lock (v0.12.0) — implemented

**Status (2026-09-28): implemented in the working tree, not yet tagged.** `mount_mode` config and `--mode` flag with copy as the default; `MountedStateEntry.Mode` plus content and source hashes; atomic copy writer preserving the executable bit; drift statuses `stale`, `modified-locally`, `stale-and-modified`, `missing-file`, `replaced-file`, `unreadable`; `csaw update` refreshes what it can and rewrites the lock; `csaw status` shows drift counts; unmount and unpin keep edits; `internal/lock` with `.csaw/lock.yml`, `csaw use --frozen`, `lock.missing|match|mismatch` findings and `require_lock` policy; `cmd/csaw/root.go` split into per-command files; end-to-end tests run in copy mode without a symlink privilege. Remaining: the manual pass in a real Claude Code session and the dogfood week.


**1.1 Mount modes.** `internal/mount/engine.go`, `internal/sources/config.go`

- `Config.MountMode` (`copy` default, `symlink` opt-in), settable with `csaw config set mount_mode symlink`. Per-mount override `--mode`. The mode used is recorded per entry in mount state so mixed states are handled on unmount and update.
- `MountedStateEntry` gains `Mode`, `ContentSHA256` (bytes written), `SourceSHA256` (for all entries, not only protected). `Generated` entries already carry `ContentSHA256`; copy mode generalizes that path to every entry.
- Copy writer: write to a temp file in the target directory and rename (atomic), preserve the executable bit (skill `scripts/`), never follow an existing symlink at the target.
- Conflict semantics unchanged: an existing unmanaged file is stashed or skipped. An existing file that matches the recorded content hash is "already mounted".

**1.2 Unmount and update semantics.** `internal/mount/engine.go`, `internal/drift/drift.go`

- Drift statuses for copy mode: `healthy` (project hash == recorded), `modified-locally`, `stale` (source hash != recorded, project unmodified), `stale-and-modified`, `missing`, `source-missing`. The `generated-modified` status from Phase 0 folds into `modified-locally`.
- Unmount: healthy or stale → remove; modified → move to `.csaw-stash/modified/<path>` (the Phase 0 mechanism) and print the path; then restore any stashed original.
- `csaw update`: refresh stale entries; for stale-and-modified print a three-way hint (`csaw diff`, `csaw fork`, `csaw unmount <path>`). `csaw status` counts each status. `csaw check` reports the same statuses. `csaw diff <path>` runs a unified diff of project copy vs source (stdlib line diff; no new dependency).

**1.3 Lockfile.** New `internal/lock`

- `.csaw/lock.yml`: version, generated timestamp, per source `{name, url, ref_requested, ref_resolved (commit), mode}`, per entry `{path, source, registry_path, source_sha256, content_sha256, generated}`. Written on every successful mount and update. Committed by choice; `.csaw/policy.yml` gains `require_lock: true` so audit fails when the lock is missing or mismatched.
- Audit findings: `lock.missing` (warn, error if required), `lock.match`, `lock.mismatch` (error) with the differing paths in `detail`.
- `csaw use --frozen`: resolve sources at the locked commits via the existing worktree checkout and fail if the planned entry set or hashes differ from the lock.

**1.4 Windows.** `internal/linkmode`, CI

- Copy mode needs no privilege. The hardlink fallback is used only in symlink mode. Add an e2e run on Windows CI with `mount_mode: copy` and, separately, symlink mode marked `t.Skip` when the privilege is missing.

**1.5 Command split.** `cmd/csaw`

- Move commands out of `root.go` into per-command files. Behavior-preserving; do it before 1.1 so the engine diff is reviewable.

**1.6 Docs for Phase 1.** README "Reversible by design" rewritten around ownership hashes; a "Mount modes" section; `docs/reference/audit-json.md` gains the lock findings; `docs/curriculum.md` lab on `check`/`update`/`diff`; `ARCHITECTURE.md` package map adds `lock`.

### Phase 2: hooks as a merged kind (v0.13.0)

**2.1 Generalize the merge engine.** `internal/mcpmerge` → `internal/configmerge` (keep `mcpmerge` as a thin wrapper for the existing CLI)

- Two strategies: `section` (TOML, text markers, exists today for Codex MCP) and `owned-keys` (plain JSON: parse, insert csaw-owned entries under the natural key, record owned key paths and their hashes in a per-target manifest under `~/.csaw/state/`, remove only owned keys on rollback, refuse to overwrite a key that exists and is not owned unless `--force`). JSON is re-serialized with two-space indent; the manifest records the pre-write hash so `csaw check` can flag hand edits to owned entries.
- Targets: `claude-settings` (`.claude/settings.local.json`, owned keys under `hooks`) and `codex-hooks` (`.codex/hooks.json`, whole-file if absent else owned keys). Both files use the same event and matcher vocabulary, so one registry hooks file needs no translation table.

**2.2 Hooks kind.** `internal/mount/tools.go`, new `internal/hooks`

- Registry layout: `hooks/hooks.json` (Claude/Codex shape) and `hooks/scripts/*`. Old `hooks/*.sh` still mounts as scripts with the Phase 0 label.
- Projection per configured tool: scripts copied to `.claude/hooks/scripts/` and `.codex/hooks/scripts/`; `hooks.json` entries rewritten so `command` points at the copied script through the tool's project-root variable (`${CLAUDE_PROJECT_DIR}` for Claude Code, a documented relative path for Codex), then merged via 2.1.
- Unmount removes owned hook entries and scripts. `csaw audit` gains `hook.unmanaged` (warn) when a hooks file contains entries csaw does not own but the policy blocks the `hooks` kind.
- Codex trusts non-managed hooks by hash and re-prompts when they change; document that a remount of a changed hook triggers one prompt.

**2.3 Docs for Phase 2.** Walkthrough "Lifecycle Hooks" rewritten with the `hooks.json` example; `mcp-merge-design.md` updated to describe both strategies; curriculum kinds table.

### Phase 3: export and trust (v0.14.0)

**3.1 Export.** New `internal/export`, `cmd/csaw/export_cmd.go`

- `csaw export plugin <source>[/<profile>] --format claude-code|agent-plugins --out <dir>`.
  - Claude Code: `.claude-plugin/plugin.json`, `skills/`, `agents/`, `hooks/hooks.json` (from the Phase 2 layout), `.mcp.json`. Rules and instructions are not plugin components; print what was omitted.
  - Agent Plugins: `plugin.json` (`$schema`, `name`, `version` from the source's resolved commit or `--version`, `description` from the profile), `skills/<name>/`, `mcp.json` normalized from `mcp/codex.toml` or `mcp/*.json` (`mcpServers`, transports `stdio` and `streamable-http`, reject entries with literal secrets using the existing `ValidateNoLiteralSecrets`). Agents, rules, hooks, and instructions are outside the format; print what was omitted.
- `csaw export marketplace <source> --out <dir>`: `.claude-plugin/marketplace.json` with one plugin per profile, `source` pointing at the exported subdirectory. Fixture-based test, no network.

**3.2 Vendor checks and scanner seam.** `internal/vendor/sync.go`, new `internal/scan`

- Deterministic checks on every synced file: hidden or bidirectional Unicode controls, zero-width characters, `SKILL.md` frontmatter containing `allowed-tools` or `hooks`, backtick-bang command substitution in skill bodies, executable files outside `scripts/`, symlinks inside vendored trees. Findings are recorded per file in `vendor.lock.yaml` and printed by `csaw vendor audit`.
- `~/.csaw/config.yml` `scanner: {command: "...", args: [...]}`; when set, `vendor sync` runs it against the vendor directory, stores exit status and report path, and `promote` refuses on non-zero unless `--force`. No scanner ships with csaw.

**3.3 Skill validation.** `internal/mount/planner.go`, `internal/audit`

- Name rule from the Agent Skills spec: `^[a-z0-9]+(-[a-z0-9]+)*$`, 1 to 64 characters, and `name` in frontmatter must equal the directory name when present. Invalid skills are skipped at mount with a warning and produce `skill.invalid` in audit.

**3.4 Docs for Phase 3.** README section "Publishing to Claude Code and Codex plugin channels"; `vendors-design.md` updated with the check list; `audit-json.md` new IDs.

### Cross-cutting

- **Retired in `next-up.md` and `roadmap.md`:** `csaw://` scheme, GUI, context vocabulary refactor, continuity and handoff, community preset marketplace. Packaging channels stay deferred.
- **Reposition the README.** Lead with the contractor and multi-org persona and the properties teamai does not offer: explicit, no daemon, no auto-injected hooks, no telemetry, multi-source with priority and protection, audit in CI. Add teamai to the comparison table.
- **Dogfood gate.** Before tagging v0.12.0: install the built binary on PATH, register the real personal registry, mount it into at least three repos on this machine in copy mode, use Claude Code and Codex against them for a week, and record friction in this plan under Validation.
- **JSON freshness.** `tool-projection.json` carries `checked_at` per tool; the release checklist in `AGENTS.md` adds "re-verify claude and codex if older than 120 days".
- **Tech debt tracker.** Add: `internal/runtime` and `internal/workspace` coverage below 35 percent; raise with Phase 1 tests.

## Risks

- **Copy mode loses live edits.** Mitigation: `csaw status` and `check` report stale entries; `update` refreshes in one command; symlink mode remains available.
- **A `CLAUDE.md` that is itself a symlink outside the working directory may not load.** Unverified; the bridge is a real file in both modes, and copy mode removes the question for every other file.
- **Owned-keys JSON merge re-serializes `.claude/settings.local.json`.** Key order and indentation may change once. Documented; the TOML target keeps the text-preserving section strategy.
- **Codex re-trust prompts** on every changed hook. Documented; unavoidable by design of Codex.
- **Scope.** Each phase is a release on its own. If Phase 2 or 3 stall, Phases 0 and 1 already make the tool correct on both supported tools.
- **Zero users.** No external signal will arrive. The dogfood gate is the only feedback loop; treat its friction list as the backlog.

## Validation

Per phase, before tagging:

```bash
gofmt -l .
go vet ./...
go test ./...
go build ./...
go run ./cmd/tools-gen   # tool-projection.md regenerated, no diff
```

Manual passes (extend `v1.0-readiness.md` Pass 6):

- Phase 0: mount `tools: claude,codex` into a scratch repo; open Claude Code and confirm `/context` lists `CLAUDE.md` and each rule (expect the external-import limitation on rules until Phase 1); open Codex and confirm `AGENTS.md` loads and the skill appears under `$`; confirm no `.codex/` directory exists.
- Phase 1: repeat in copy mode on macOS and Windows; confirm rules load in Claude Code with no prompt; edit a mounted file, run `check`, `update`, `unmount`, and confirm the edit is stashed, not lost; commit `.csaw/lock.yml`, change a source, confirm `audit` reports `lock.mismatch`.
- Phase 2: mount a `hooks/hooks.json` with a `PreToolUse` command; confirm Claude Code lists it under `/hooks` and Codex under its hooks view; unmount and confirm both files are clean.
- Phase 3: export a preset registry as a Claude Code marketplace and install it from a local path; export the same as an Agent Plugins directory and load it in Codex; vendor a skill containing a zero-width character and confirm `promote` refuses.

Dogfood log (fill in during Phase 1):

- 
