# Package Manager Lessons for csaw

Research notes on `cargo`, `uv`, and `pnpm` patterns relevant to two upcoming csaw features: starter sources (`--preset`) and a one-liner / URL scheme for adding sources. Captured 2026-05-26.

This doc exists so future work doesn't re-derive these patterns from scratch. Decisions belong here; rationale belongs in commits.

---

## 1. Why study these three

csaw isn't a package manager and shouldn't become one. But it shares two surface problems with package managers, and these three tools have battle-tested designs worth borrowing:

- **Scaffolding new units**: a new csaw source is shape-equivalent to a new cargo crate / uv project / npm package. They've all solved "make a known-good thing in one command."
- **Pointing at a remote thing**: `csaw source add` ≈ `cargo add --git`, `uv add git+...`, `pnpm add github:...`. The UX/syntax range here is wide; some are friendlier than others.

A third pattern — lockfiles for reproducibility — is noted but deferred (see §5).

---

## 2. Templates / scaffolding patterns

### How each tool does it

| Tool | Command | Template source | Customization | Community templates? |
|---|---|---|---|---|
| `cargo new` | `cargo new name --bin\|--lib` | Built-in only (2 shapes) | Implicit via flags | Via separate `cargo-generate` crate |
| `uv init` | `uv init name [--app\|--lib\|--bare\|--package]` | Built-in only (4 shapes + `--build-backend <name>`) | Flags: `--description`, `--author-from git`, `--vcs`, `--python-pin` | No |
| `pnpm create` | `pnpm create <name>` (resolves to `create-<name>` on npm) | Community packages on npm registry, conventional `create-*` / `@scope/create-*` naming | Whatever the starter package wants | Yes — primary model |

### Patterns worth borrowing

**(a) Start built-in only.** Both cargo and uv ship a small, curated set of templates and stop there. Community-template ecosystems require a security model (npm's trust policies, semver release-age gates), and that's a v2 problem.

**(b) Flag parameterization for the common knobs.** uv's `--description`, `--author-from git`, `--vcs` lets users tweak generated content without forking a template. This is cheap to add and high-value.

**(c) Discrete named shapes, not slots-in-a-config.** Templates aren't a parameterized DSL — they're a handful of clearly-named choices (`--bin`, `--lib`, `--app`, `--bare`). Easy to discover, no decision fatigue.

### Anti-patterns to avoid

- **pnpm-style npm-registry resolution for csaw.** csaw doesn't have a registry equivalent. Adopting "look up `create-foo` somewhere remote" requires building both the registry and the trust model. Out of scope.
- **cargo-generate-style external templates with arbitrary code execution.** Templates that run `build.rs`-equivalent setup scripts are powerful but create supply-chain risk. csaw templates should be pure file copies + variable substitution, nothing executable.

### Proposed csaw shape

`csaw source init --preset <name>` produces a known-good source structure. Initial presets (in code, curated):

- `solo-engineer` — Personal source: AGENTS.md + a few starter rules + one example skill
- `team-go` — Team source: protected AGENTS.md, Go-specific rules, code-review agent, commit-message skill
- `team-frontend` — Team source: TypeScript/React rules, perf-review agent, PR-template skill
- `consulting` — Horizontal/client source shape: client-isolation policy starter, blocked_paths examples

Parameterized via flags: `--name`, `--description`, `--protected-files` (comma list), `--include-mcp`.

Presets live as Go string constants or embedded files in `internal/registry/presets/` — same delivery model as the current `starterIgnore`/`starterAgents` content.

`csaw source init --list-presets` shows what's available. Discovery, not magic.

---

## 3. Git dependency URL syntax — the one-liner game

This is where the three tools diverge sharply. csaw's `csaw source add <name> <url> [--branch X]` is closest to cargo's verbose declarative form; the one-liner game (Slack-pasteable, share-friendly) is where pnpm leads.

### How each tool does it

**cargo** — declarative, all in `Cargo.toml`:
```toml
regex = { git = "https://github.com/rust-lang/regex.git", branch = "next" }
regex = { git = "https://github.com/rust-lang/regex.git", tag = "1.10.3" }
regex = { git = "https://github.com/rust-lang/regex.git", rev = "0c0990..." }
regex = { git = "https://github.com/rust-lang/regex.git", rev = "refs/pull/493/head" }
```
- `.git` suffix optional
- `rev` accepts SHA or any `refs/...` form (including PR refs)
- Default: latest commit on default branch
- Git submodules auto-fetched

**uv** — split: URL is the protocol, refs are flags:
```bash
uv add git+https://github.com/encode/httpx --tag 0.27.0
uv add git+https://github.com/encode/httpx --branch main
uv add git+https://github.com/encode/httpx --rev 326b9431
uv add git+https://github.com/langchain-ai/langchain#subdirectory=libs/langchain
uv add --lfs git+https://github.com/astral-sh/lfs-cowsay
```
- `git+https://` or `git+ssh://` required
- Refs are CLI flags, not URL parts
- Subdirectory via URL fragment

**pnpm** — most compact, most share-friendly:
```bash
pnpm add github:zkochan/is-negative
pnpm add bitbucket:pnpmjs/git-resolver
pnpm add gitlab:pnpm/git-resolver
pnpm add github:zkochan/is-negative#2.0.1            # tag
pnpm add github:zkochan/is-negative#master           # branch
pnpm add github:zkochan/is-negative#97edff6f         # commit
pnpm add github:zkochan/is-negative#semver:^2.0.0    # semver range
pnpm add github:zkochan/is-negative#beta&path:/packages/simple-react-app
```
- Host shorthands: `github:`, `bitbucket:`, `gitlab:`
- Refs in URL fragment after `#`
- Combined options with `&`
- One-line copy-paste shareable

### What's actually worth borrowing for csaw

1. **Adopt pnpm-style host shorthand for one-liners.** It's the friction-killer. Both `csaw source add team https://github.com/co/team-source.git --branch main` and `csaw source add team gh:co/team-source#main` should work. The shorthand is faster, paste-friendly, and self-documenting.

2. **Match cargo's ref-key semantics under the hood.** Whatever the surface syntax, the resolved spec should be (URL, refKind, refValue) where refKind ∈ {branch, tag, rev, default}. pnpm's `#semver:^2.0.0` is too clever for csaw — sources aren't semver-published artifacts. Skip it.

3. **Keep both surfaces.** Declarative long form (in sources.yml / config) for clarity; one-liner shorthand for CLI / share links. Same way cargo has `Cargo.toml` declarative AND `cargo add --git ...` ergonomic.

4. **Subdirectory support is real but defer it.** uv's `#subdirectory=` and pnpm's `&path:` cover the "I want one source dir inside a monorepo." Worth supporting eventually; not needed for v0.8.

### Proposed csaw shape

**Shorthand parser** in `internal/sources/`:
- Input: `gh:org/repo[#ref]`, `gl:org/repo[#ref]`, `bb:org/repo[#ref]`, or full URL with optional `--branch`/`--tag`/`--rev`
- Output: canonical `(url, refKind, refValue)` tuple stored in sources.yml
- The CLI accepts shorthand; the config file stays declarative

**Deep link / URL scheme** `csaw://add-source/<encoded-spec>`:
- Encodes the same shorthand: `csaw://add-source/team@gh:co/team-source#main`
- On click: csaw opens, shows a confirmation TUI ("add source `team` from `https://github.com/co/team-source` at branch `main`?"), then runs `csaw source add`
- Mirrors `ccswitch://` UX; lower friction than copy-pasting a CLI command
- Requires OS-level protocol registration (Tauri/Electron territory; for Go CLI, document the install-time registration step)

**Bikeshed for later:** whether `@` separates name from spec, or another delimiter is cleaner. `team@gh:co/repo#main` is readable but `@` is already overloaded in source-ref grammars.

---

## 4. Lockfile / reproducibility — defer, but know the shape

csaw already supports per-source pinning (`csaw pin source@ref`). What's missing is an aggregate **`csaw.lock`** for byte-identical mount reproduction across machines.

### How cargo / uv do it

- **cargo**: locks commit hash at dep-addition time; updates only on `cargo update`. Auto-fetches submodules.
- **uv**: `uv lock` regenerates; `uv sync` applies. Drift checked by comparing to project metadata, not upstream releases. `uv lock --check` for CI.

Both share two principles worth noting:
1. **Lockfile pins what was resolved, not what's available.** If upstream releases a new version, the lockfile doesn't know — you opt in with an explicit update command.
2. **Two commands, not one.** Generate (cargo update / uv lock) vs. apply (cargo build / uv sync). csaw's `csaw mount --restore` already mirrors the "apply" half.

### Why defer for csaw

csaw's current `csaw pin source@ref` already locks individual sources. An aggregate `csaw.lock` would matter most for:
- Reproducing a colleague's exact mount state (today: requires them to share the full sources.yml + every pin command)
- CI workflows that mount csaw context as part of a build (rare today)

Neither is hurting users yet. Add when demanded. The cargo model is the one to copy.

---

## 5. Cross-cutting principles to internalize

From studying all three:

1. **Two surfaces: declarative file + ergonomic CLI.** Every winning tool has both. csaw already does this (sources.yml + `csaw source add`); don't lose it by chasing CLI-only or config-only purity.

2. **Generate vs. apply separation.** Locking ≠ syncing; init ≠ build. Verb pairs matter; combining them in one command loses control.

3. **Curate built-ins; gate community contribution behind security.** None of cargo, uv, or pnpm shipped community templates on day one. Community comes after the trust model.

4. **Shorthand for sharing, longhand for clarity.** pnpm's `github:user/repo` is one-line shareable; cargo's `{ git = "...", tag = "..." }` is unambiguous in config. csaw wants both.

5. **Anti-pattern: too-clever syntax in URLs.** pnpm's `#semver:^2.0.0&path:/...` is admirable but mostly unused. Stick to refs (branch/tag/rev) and one optional subdirectory; resist further compression.

---

## 6. Recommendations summary

For the four upcoming v0.8 quick wins:

| Win | Lessons applied |
|---|---|
| "Minimal intrusion" framing in README | None directly — pure copywriting from cc-switch research |
| `--preset` for starter sources | uv-style built-in templates, flag parameterization; skip community-template marketplace for now |
| URL scheme for one-click source add | pnpm-style `gh:org/repo#ref` shorthand + `csaw://add-source/...` deep link; canonical (url, refKind, refValue) under the hood |
| Edit-while-mounted UX + packaging audit | Mostly orthogonal; lockfile pattern noted as future work if reproducibility friction appears |
