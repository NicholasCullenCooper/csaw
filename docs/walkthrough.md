# csaw Walkthrough

Scenario-based guide for csaw beyond the [README](../README.md) quick start. Each section is self-contained — jump to the one that matches your situation.

**Contents:**
- [I Already Have an AGENTS.md](#i-already-have-an-agentsmd) — adopt existing AI config files into a registry
- [Using a Team Source](#using-a-team-source) — share config from a team git repo
- [Composing Multiple Sources](#composing-multiple-sources) — layer company + team + personal
- [Protected Files](#protected-files) — enforce mandates that can't be overridden
- [Auditing Active Context](#auditing-active-context) — declare and verify project policy
- [Experimental Skills](#experimental-skills) — develop skills before promoting
- [Pulling Team Updates](#pulling-team-updates) — keep in sync with team changes
- [Sharing Your Changes](#sharing-your-changes) — push edits back to a source
- [Testing a Branch](#testing-a-branch) — pin a project to a feature branch
- [Forking a Team File](#forking-a-team-file) — customize without breaking the upstream
- [Switching Profiles](#switching-profiles) — change active work mode
- [Sharing MCP With Codex (`csaw mcp sync`)](#sharing-mcp-with-codex-csaw-mcp-sync) — merge team MCP into a shared-config tool file
- [Vendoring External Catalogs Safely (`csaw vendor`)](#vendoring-external-catalogs-safely-csaw-vendor) — consume skills.sh / APM / awesome-copilot without trusting upstream blindly
- [Clean Removal](#clean-removal) — unmount everything cleanly

---

## I Already Have an AGENTS.md

You have an existing project with AI config files scattered around — an AGENTS.md, maybe some skills in `.claude/skills/`. You want to pull them into a registry instead of leaving them committed.

```bash
cd ~/my-project
csaw init --adopt ~/my-ai-config
```

```
╭───────────────────────────────────────╮
│                                       │
│  adopted 3 files                      │
│                                       │
│   ✔ AGENTS.md                         │
│   ✔ skills/testing/SKILL.md           │
│   ✔ rules/go.md                       │
│                                       │
╰───────────────────────────────────────╯
```

csaw scans your project, finds AI config files, and copies them into the new registry with the correct structure. It reverses the projection — `.claude/skills/testing/SKILL.md` becomes `skills/testing/SKILL.md` in the registry, `.claude/rules/go.md` becomes `rules/go.md`, `.claude/agents/reviewer.md` becomes `agents/reviewer.md`.

Now you can delete the originals from your project, register the source, and activate it instead:

```bash
csaw source add personal ~/my-ai-config --priority 10
csaw use personal/default
```

---

## Using a Team Source

Your team keeps shared AI config in a git repo. One command to get it:

```bash
csaw source add team git@github.com:your-org/ai-config.git
```

```
✔ registered source "team"
✔ cloned team
```

csaw auto-clones the repo. Now activate a named work mode:

```bash
cd ~/my-project
csaw profile list
csaw use team/backend
```

`team/backend` means: use the `backend` profile from the configured `team`
source. Your project gets the team's AGENTS.md, skills, and rules — all
copied in. Every repo on the team can activate the same named mode. When
someone updates the team config:

```bash
csaw pull team
# ✔ pulled team
```

Run `csaw pull` then `csaw update` in a project to pick up edits to already-mounted files; `csaw check` lists what is stale.
If the update added new files, rerun the same activation command to add the new
copies.

---

## Composing Multiple Sources

You want company standards, team conventions, and your personal preferences composed into every repo — with priority and protection in the right places.

Adding a source only makes it available. It does not activate that source in a project. The active context is whatever `csaw use` last mounted, and you can verify it with `csaw inspect` or `csaw audit`.

```bash
csaw source add company gh:your-org/eng-standards --priority 100   # shorthand
csaw source add team    gh:your-team/ai-config#v2 --priority 50    # pin team to v2
csaw init ~/my-ai-config --preset solo-engineer                    # curated personal scaffold
csaw source add personal ~/my-ai-config --priority 10
```

Shorthand (`gh:org/repo[#ref]`, also `gl:` and `bb:`) is interchangeable with the long form. The `#ref` suffix pins the source's default ref across all projects — per-project pins (`csaw pin team@feature`) still take precedence when you need a single project on a different branch.

Activate one source's profile and only that gets mounted:

```bash
csaw use team/backend
```

That activates the `backend` profile from `team`, not every configured source. To compose multiple sources into one work mode, define a named profile in your personal source's `csaw.yml`:

```yaml
work:
  description: Company standards + team backend + my personal helpers
  extends:
    - company/required
    - team/backend
  include:
    - skills/debugging/**
    - skills/note-capture/**
```

Then activate it:

```bash
csaw use personal/work
```

Because this profile lives in the `personal` source, `skills/debugging/**` resolves to `personal/skills/debugging/**`. `company/required` and `team/backend` remain source-qualified. The only string you type day-to-day is `personal/work`.

For one-off debugging, the raw pattern command still exists:

```bash
csaw mount paths 'company/**' 'team/**' 'personal/skills/**'
```

It is the advanced escape hatch, not the normal workflow.

### Consultant variant: client isolation

If you're a contractor or consultant, the same compose-and-activate pattern applies — substitute client sources for company+team, and pair activation with project policy that blocks other clients:

```bash
csaw source add client-acme git@github.com:acme/ai-config.git --priority 50
```

Define a profile in your personal source bundling the client with safe-for-client helpers:

```yaml
acme-work:
  description: Acme config plus my safe-for-client helpers
  extends:
    - client-acme/backend
  include:
    - skills/code-review/**
    - skills/debugging/**
```

In the project, declare what's required and what's forbidden:

```yaml
required_sources:
  - client-acme
blocked_sources:
  - client-globex
  - personal-experimental
blocked_kinds:
  - mcp
required_kinds:
  - instructions
```

```bash
csaw audit --strict
```

The contractor's case is the same composition pattern as the canonical — project policy is what does the isolation work.

### What if two sources provide the same file?

**Priority decides.** Higher number wins.

```bash
csaw inspect
```

```
Sources
  company  (remote, priority 100) → ~/.csaw/sources/company
  team     (remote, priority 50)  → ~/.csaw/sources/team
  personal (local,  priority 10)  → ~/my-ai-config
```

You can set priority on any source:

```bash
csaw source add personal ~/my-config --priority 10
csaw source add team git@github.com:org/config.git --priority 50    # team wins over personal on overlap
csaw source add company git@github.com:org/standards.git --priority 100  # company wins over both
```

Protected files in higher-priority sources can't be overridden by lower layers at all (see [Protected Files](#protected-files) below).

If two sources have the same priority and provide the same file, csaw errors and tells you to resolve it explicitly.

---

## Protected Files

When a source needs to enforce that certain files **cannot be overridden** — company-wide security rules, a department's mandatory PR-workflow skill, a client's required `AGENTS.md` — mark them as protected in that source's `csaw.yml`:

```yaml
csaw:
  protected:
    - AGENTS.md
    - rules/security.md

backend:
  include:
    - AGENTS.md
    - rules/**
```

When a file is protected:

- **Priority is bypassed.** Even if personal has priority 100, the protected source wins for that file.
- **Fork is refused.** `csaw fork client-acme/AGENTS.md --into personal` returns an error.
- **Protection is visible.** `csaw inspect` marks protected files with a `*` under the source.
- **Protection is verified.** csaw records a SHA-256 hash for protected mounts and `csaw check` / `csaw audit` detect content drift.

This is the mechanism for layered governance — let any upstream source (company, department, team, or client) publish required files, layer personal preferences on top, and csaw won't let the personal layer break the protected ones.

Protection is **local assurance, not hard enforcement**. csaw prevents its own mechanisms from bypassing protected files and detects if protected mounted content no longer matches the mount-time hash. Remount to accept an intentional protected source update. csaw does not sandbox the machine or stop a user from manually editing files outside csaw.

---

## Auditing Active Context

Create a starter policy:

```bash
csaw audit --init
```

Projects can declare local context requirements in `.csaw/policy.yml`. The canonical case is "this repo must mount the company and team sources, and not personal-experimental":

```yaml
required_sources:
  - company
  - department
  - name: team
    url: git@github.com:my-team/ai-config.git
    ref: main
blocked_sources:
  - personal-experimental
required_kinds:
  - instructions
  - rules
```

Run audit before starting work, before handing off, or in local/CI checks:

```bash
csaw audit
csaw audit --strict
csaw audit --json
```

`csaw audit` checks active mount health, protected file content drift, required sources, required source URLs and project pins, blocked source patterns, blocked kinds, blocked mounted paths, and required artifact kinds. Default mode exits nonzero on errors. `--strict` also exits nonzero on warnings, including a missing project policy.

The `ref` field checks the project pin set by `csaw pin team@main`; it is not inferred from the source checkout's current branch. The JSON output is documented in [docs/reference/audit-json.md](docs/reference/audit-json.md).

This is **local assurance**, not hard prevention. csaw can tell you the right sources are active and forbidden ones are absent, but it does not sandbox your machine or stop a user from manually editing files.

Example client isolation policy:

```yaml
required_sources:
  - name: client-acme
    url: git@example.com:org/client-acme-ai.git
    ref: approved
blocked_sources:
  - other-client-*
  - personal-experimental
blocked_kinds:
  - mcp
blocked_paths:
  - .claude/agents/**
required_kinds:
  - instructions
  - rules
```

Example team policy:

```yaml
required_sources:
  - platform-team
blocked_sources: []
blocked_kinds: []
blocked_paths: []
required_kinds:
  - instructions
  - rules
```

---

## Experimental Work (Skills, Rules, Agents, Hooks)

Working on something new? Put it under an `experimental/` segment — anywhere, in any kind:

```
~/my-ai-config/
  rules/
    security.md                  ← stable, always mounted
    experimental/
      new-policy.md              ← hidden by built-in convention
  agents/
    code-reviewer.md             ← stable
    experimental/
      perf-analyzer.md           ← hidden
  skills/
    code-review/SKILL.md         ← stable
    experimental/
      debug-strategy/SKILL.md    ← hidden
```

csaw treats any path segment exactly named `experimental` as in-progress, at any depth and across all kinds. No `.csawignore` entry needed — it's a built-in convention. (Substring matches like `experimental-features.md` are NOT hidden — only the exact segment.)

To test work-in-progress files alongside your stable mount:

```bash
csaw use personal/default --include-experimental
```

When you're confident a skill is ready, promote it:

```bash
csaw promote personal/skills/experimental/debug-strategy
# ✔ promoted debug-strategy from experimental to stable
#   Push: csaw push personal -m "promote debug-strategy"
```

This moves it from `skills/experimental/debug-strategy/` to `skills/debug-strategy/` — now it mounts by default. (`csaw promote` currently only handles skills; for rules/agents you move the file manually.)

**`.csawignore` is separate.** `.csawignore` is for custom hide patterns (drafts/, archived/, client-specific stuff). The override flag is different too: `--include-ignored` mounts `.csawignore`'d files; `--include-experimental` mounts files under `experimental/`. Use either, both, or neither.

To share a promoted skill with the team:

```bash
csaw fork personal/skills/debug-strategy/SKILL.md --into team
csaw push team -m "add debug-strategy skill"
```

---

## Pulling Team Updates

A teammate updated the team's AGENTS.md. Get the latest:

```bash
csaw pull team
# ✔ pulled team
```

Since your project's `AGENTS.md` is a copy of the team registry's file, edits to
that file reach the project on the next `csaw pull` and `csaw update`. If the
team added new rules, skills, agents, or MCP files, rerun the profile activation
to add those new files.

### What if I edited a mounted file?

If you edited `AGENTS.md` in your project, `csaw check` reports it as edited locally and `csaw update` leaves it alone. In symlink mode the edit lands in the team registry checkout instead, and `csaw status` flags that too:

```
uncommitted edits in source checkouts

  • team (1 file)
      M AGENTS.md

  Next: csaw push <source> -m "..."  to share, or  csaw fork <path> --into personal  to keep private
```

If you try `csaw pull` without committing or stashing first:

```
! team has uncommitted changes
  Commit:  cd ~/.csaw/sources/team && git add -A && git commit -m "..."
  Or stash: csaw pull team --stash
```

**`--stash`** stashes your changes, pulls, then pops the stash:

```bash
csaw pull team --stash
# ✔ pulled team
```

### What if the team and I changed the same file?

If you have local commits and the remote has diverged:

```
! team has diverged (2 local, 5 remote commits)
  Resolve: cd ~/.csaw/sources/team && git pull --rebase
```

This is standard git — csaw tells you what happened and where to fix it. The registry is a normal git repo.

---

## Sharing Your Changes

You updated a skill in the registry checkout (directly, or through a symlink-mode mount). Push it:

```bash
csaw push team -m "improve code review skill"
# ✔ pushed team
```

This runs `git add -A && git commit && git push` in the team registry. Your teammates pull the update with `csaw pull`.

If you're not sure and want to go through a PR instead:

```bash
csaw source clone team ~/Developer/team-config
cd ~/Developer/team-config
git checkout -b improve-code-review
# ... edit files ...
git add -A && git commit -m "improve code review"
git push -u origin improve-code-review
gh pr create
```

`csaw source clone` moves a remote source to a local directory for contribution. Now you can branch, PR, and collaborate like any codebase.

---

## Testing a Branch

You want to try a feature branch of the team config without affecting other projects:

```bash
csaw pin team@feature/new-rules
csaw pull team
csaw use team/backend
```

This project now uses the `feature/new-rules` branch. Other projects stay on main. When you're done:

```bash
csaw unpin team
csaw pull team
```

Back to main.

---

## Forking a Team File

You like the team's `AGENTS.md` but want to customize it. Fork it:

```bash
csaw fork team/AGENTS.md --into personal
```

This copies the file to your personal registry. Since personal has higher priority, your version gets mounted instead of the team's. The team original is untouched.

---

## Switching Profiles

Activating a new profile **replaces** the previous one automatically:

```bash
csaw use team/backend
# ... working on backend ...

csaw use team/frontend
# previous mount removed, frontend mounted
```

To go back to what you had before:

```bash
csaw mount --restore
```

To add files on top of an existing mount without replacing:

```bash
csaw use personal/extras --keep
```

---

## Lifecycle Hooks

Your team ships a pre-commit script that runs `go test ./...` on changed packages before allowing a commit. You want every project using `team/` to run it without each engineer copy-pasting.

In your team source:

```
team/
├── AGENTS.md
└── hooks/
    └── pre-commit-test.sh
```

In a project with claude configured, after `csaw mount`:

```
project/
├── .claude/
│   └── hooks/
│       └── pre-commit-test.sh    ← copy of team/hooks/pre-commit-test.sh
```

csaw places the script. It does not run by itself: no AI tool scans a hooks directory. Claude Code runs hooks declared in its settings files and Codex runs hooks declared in `.codex/hooks.json` (same JSON shape). To wire the mounted script into Claude Code, declare it once in `.claude/settings.json` (or `.claude/settings.local.json` to keep it out of git):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "${CLAUDE_PROJECT_DIR}/.claude/hooks/pre-commit-test.sh" }
        ]
      }
    ]
  }
}
```

Today `.claude/hooks/` is the only script target. Projecting the declaration itself into Claude Code's settings and Codex's `.codex/hooks.json` is planned as the hooks rework in [`docs/exec-plans/active/relaunch-2026-09.md`](exec-plans/active/relaunch-2026-09.md).

`csaw inspect` shows hooks under the **Hooks** group with a "scripts only" reminder, alongside the source they came from.

---

## Ignore Patterns

Your registry also serves teammates on tools that have project ignore files (Cursor, Aider, Cody), and you want them to skip `node_modules/`, `dist/`, and `*.snap` files across every project. Neither Claude Code nor Codex reads an ignore file, so this kind is for those other tools.

In your source:

```
team/
├── AGENTS.md
└── ignore/
    ├── cursor          # gitignore-style patterns
    └── aider           # same content; tools read different files
```

After mount:

```
project/
└── .cursorignore       ← copy of team/ignore/cursor
```

Ignore files project by registry filename regardless of the tools list: `ignore/aider` becomes `.aiderignore`, and so on (`.cursorignore`, `.cody/ignore`, `.aiderignore`, `.tongyiignore`, `.geminiignore`).

If you only want to maintain one set of patterns: put them in `ignore/cursor` and `ignore/aider` as two files with identical content (today there's no single-source-of-truth aliasing — that's a roadmap item).

---

## Other Tools

csaw projects for Claude Code and Codex. Registries often carry files for other tools too (Cursor rules, Copilot instructions, ignore files); csaw does not project those, and unrecognized registry paths mount at the same relative path in the project. Most other agents read `AGENTS.md` at the root and skills under `.agents/skills/`, which csaw already writes for Codex, so they get the universal floor for free. Their researched paths stay catalogued in [`reference/tool-projection.json`](reference/tool-projection.json) for the day one is added back; adding a tool is one registry entry in `internal/mount/tools.go`.

## Sharing MCP With Codex (`csaw mcp sync`)

Codex's MCP servers live inside `.codex/config.toml` — a file that also holds your model preferences, providers, sandbox settings, and other personal config. csaw can't symlink that file (it would overwrite your settings) and can't ignore the MCP entries (then your team's MCP servers go unprojected). The solution is **merged-config projection**: csaw writes its MCP entries into a clearly-marked bounded section at the end of the file, leaving the rest byte-for-byte untouched.

Your team source has a Codex MCP fragment:

```
team/
└── mcp/
    └── codex.toml          # [mcp_servers.github], [mcp_servers.linear], ...
```

In a project with a personal `.codex/config.toml`:

```bash
csaw mcp sync codex --from team           # dry-run: show what would change
csaw mcp sync codex --from team --apply   # write the merge
```

After `--apply`, your file looks like:

```toml
# Your existing user-managed Codex config
model = "gpt-4o"

[mcp_servers.user_thing]
command = "echo"
args = ["hello"]

[providers.openai]
base_url = "https://api.openai.com/v1"

# === csaw managed start (do not edit; use: csaw mcp sync codex --remove) ===
# Source: team · 2 server(s) · regenerate: csaw mcp sync codex --apply · remove: csaw mcp sync codex --remove

[mcp_servers.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env_vars = ["GITHUB_PERSONAL_ACCESS_TOKEN"]

[mcp_servers.linear]
command = "npx"
args = ["-y", "@tacticiq/linear-mcp"]
env_vars = ["LINEAR_API_KEY"]
# === csaw managed end ===
```

Notice three things:

1. **Your user content above the marker is byte-for-byte unchanged.** csaw never parses-and-re-emits the whole file — it appends a bounded section. Comments, key order, quoting all preserved.
2. **Secrets stay out of git.** The fragment uses Codex's `env_vars = ["VAR_NAME"]` pattern, which forwards the env var by *name*; the actual token value never appears in the file. csaw refuses to write fragments with literal secrets in sensitive-named fields (`token`, `password`, `api_key`, etc.) — schema enforcement, not entropy-guessing.
3. **Conflicts are reported, not silently overwritten.** If your `.codex/config.toml` already had `[mcp_servers.github]`, csaw's `github` server would be skipped with a warning. You decide whether to rename, remove the user one, or override.

**Roll back:**

```bash
csaw mcp sync codex --remove
```

csaw verifies the bounded section's SHA matches what it last wrote (drift detection — if you edited inside the markers, it refuses), then deletes the section + restores the file to its pre-merge state.

Codex is the only merge target, and with csaw scoped to Claude Code and Codex it is the only one needed: Claude Code reads a whole-file `.mcp.json` that csaw links directly. The same bounded-section design is documented in [`docs/planning/mcp-merge-design.md`](planning/mcp-merge-design.md) should another target ever be added.

---

## Vendoring External Catalogs Safely (`csaw vendor`)

You want to use skills from a popular community repo (say, [awesome-copilot](https://github.com/github/awesome-copilot)) or consume a few skills from an APM package — without running their opaque installer, manually copying files, or letting upstream's layout become your active mounted context. csaw's vendor feature is the safe primitive for this.

The workflow has four steps: **declare → sync → review → promote.** Vendored content lives under your source registry's `vendor/` directory, is git-tracked, and *never* projects to mounted projects until you explicitly promote selected files into a real kind directory.

### Declare

In your source registry:

```bash
csaw vendor add awesome-copilot gh:github/awesome-copilot --include "agents/**" --include "skills/**"
```

This writes to your registry's `csaw.yml` under a new `vendors:` block. Glob filters keep the vendored area focused on what you care about. Shorthand (`gh:`, `gl:`, `bb:`) is accepted; an embedded `#ref` becomes the tracked ref.

### Sync

```bash
csaw vendor sync           # all declared vendors
csaw vendor sync awesome-copilot   # one specific
```

csaw clones the upstream into a per-URL cache under `~/.csaw/state/vendor-cache/`, resolves the ref to an exact SHA, copies filtered files into `<registry>/vendor/awesome-copilot/`, and writes the SHA-pinned inventory into `<registry>/vendor.lock.yaml`. Subsequent syncs reuse the cache and only re-copy if upstream content changed.

After sync your registry looks like:

```
my-registry/
├── csaw.yml                          # declares vendors:
├── vendor.lock.yaml                  # per-file SHAs, promotion log
├── vendor/
│   └── awesome-copilot/
│       ├── .csaw-vendor-meta.yaml
│       ├── agents/
│       │   └── code-reviewer.md
│       └── skills/
│           └── pr-checklist/
│               └── SKILL.md
├── agents/                           # hand-authored or promoted from vendor/
├── rules/
└── skills/
```

Both `vendor/` and `vendor.lock.yaml` are git-tracked. Consumers of your registry get the exact vendored state you reviewed.

### Review (audit drift, three kinds)

```bash
csaw vendor audit
```

Reports three drift types:

1. **Vendor-local drift** — someone hand-edited a file inside `vendor/<name>/`. Either a mistake (run `csaw vendor sync --force` to restore) or an intentional one-off (move the edit out of `vendor/` into a real kind dir).
2. **Upstream drift** — `git ls-remote` shows upstream has new commits. Refresh with `csaw vendor sync` if you want to track them.
3. **Promotion drift** — a promoted file diverged from its vendor origin (either the vendor moved since promote, or you intentionally customized the promoted copy). Surfaces the choice instead of silently letting them desync.

Use `--no-network` to skip the upstream check in offline / CI environments.

### Promote

```bash
csaw vendor promote awesome-copilot/agents/code-reviewer.md --into agents/code-reviewer.md
```

Copies `vendor/awesome-copilot/agents/code-reviewer.md` to `agents/code-reviewer.md` in your registry and records the lineage (vendor name, vendor path, SHA at promote time) in `vendor.lock.yaml`. The vendored copy stays in place as the immutable upstream record; the promoted copy enters your normal kind directory and projects via existing csaw mount/profile flows. Refuses to overwrite an existing file without `--force`.

### Why this exists

External catalogs (skills.sh, APM packages, awesome-copilot, internal bundles) ship in shapes designed for browsing or for their own installer's runtime — not for csaw's mount layout. Directly mounting them would be messy (junk files everywhere) and unsafe (no SHA pinning, no review step, no rollback). Vendor adds a third state for content: *fetched, hash-locked, never projects, requires explicit promote.* That makes csaw a safe consumer of any external catalog, regardless of ecosystem.

The vendored content can't accidentally become active context — it's explicitly excluded from source enumeration. Mounting only sees the files you've promoted into real kind directories.

---

## Clean Removal

```bash
csaw unmount
```

Every file csaw wrote is removed. If csaw stashed any original files during mount (because they existed before), they're restored. Anything you edited after mounting is moved to `.csaw-stash/modified/` instead of deleted. Your project is exactly as it was.

```
✔ 6 removed · 2 restored

  Remount: csaw mount --restore
```

---

