# csaw Curriculum

This curriculum is the guided path from first use to expert operation of csaw.
Follow it top to bottom and do the exercises in a disposable project.

Maintenance contract: update this document whenever csaw changes user-facing
commands, config files, artifact kinds, projection behavior, audit policy,
drift detection, release flow, or recommended workflows.

## Learning Outcomes

After completing this curriculum, you should be able to:

- Explain when csaw is useful and when repo-local context is better.
- Design personal, team, client, and community AI workspace sources.
- Use profiles, priorities, protected files, pins, fork, promote, and restore.
- Predict where instructions, rules, agents, skills, and MCP files mount.
- Audit active context for required sources, blocked sources, required kinds,
  source URL, project pin, mount health, and protected content drift.
- Diagnose and recover from link drift, replaced files, missing sources, and
  stale protected hashes.
- Operate csaw safely across multiple projects, tools, teams, and clients.
- Contribute changes to csaw without breaking its core guarantees.

## Prerequisites

You should already be comfortable with:

- Basic shell navigation.
- Git repositories, branches, remotes, commits, and ignored files.
- The idea of AI coding tool context files such as `AGENTS.md`, rules, agents,
  skills, and MCP server configuration.

Use generic test directories throughout the curriculum. Do not use real client
or production secrets in exercises.

## Curriculum Project Setup

Create a disposable workspace:

```bash
mkdir -p ~/csaw-lab
cd ~/csaw-lab
git init app
mkdir personal-ai team-ai client-acme-ai client-globex-ai
```

Use `~/csaw-lab/app` as the project and the other directories as local csaw
sources. Local sources are enough to learn the behavior; later modules cover
remote git sources.

## Module 1: Product Model

### Core Idea

csaw mounts AI workspace files from one or more sources into a project using
links and local git excludes. The project stays clean. The source stays
editable and git-backed. The mounted files are reversible.

### The Repo-Local Rule

If a context file belongs to exactly one repo and is safe to commit there, keep
it in that repo.

Use csaw when context crosses at least one boundary:

- Multiple repos need the same AI workspace files.
- A team or client owns required context.
- Personal context should layer on top without being committed.
- Different projects need different source refs.
- Multiple AI tools need the same logical files projected to different paths.
- You need local evidence that the right context is mounted.

### Core Objects

| Object         | Meaning                                                      |
| -------------- | ------------------------------------------------------------ |
| Project        | The git repo where AI files are mounted.                     |
| Source         | A local directory or git repo containing AI workspace files. |
| Profile        | A named selection in `csaw.yml` that says what to mount.     |
| Mount          | The linked files placed into the project.                    |
| Kind           | One of instructions, rules, agents, skills, MCP, or other.   |
| Policy         | `.csaw/policy.yml`, checked by `csaw audit`.                 |
| Pin            | A per-project source ref set with `csaw pin source@ref`.     |
| Protected file | A file a source marks as mandatory and non-overridable.      |

### Exercise

Answer these before continuing:

- Which AI files in your real work belong in a single repo?
- Which ones cross repo, team, client, privacy, or tool boundaries?
- Which ones would be risky if the wrong client context were active?

## Module 2: Install And First Source

Install csaw:

```bash
uv tool install csaw
```

Other install paths are available through Homebrew, Scoop, pipx, and Go. Use the
README for platform-specific details.

Create a personal source:

```bash
cd ~/csaw-lab
csaw init personal-ai --name personal
```

`csaw init` scaffolds the source. Registering that source is the next explicit
state change:

```bash
csaw source add personal ~/csaw-lab/personal-ai --priority 10
```

Inspect configured sources:

```bash
csaw source list
csaw config list
```

### Exercise

Open `personal-ai`. Identify:

- `csaw.yml`
- `.csawignore`
- `AGENTS.md`
- `rules/`
- `agents/`
- `skills/`

Explain what each does.

## Module 3: Registry Anatomy

A csaw source is a normal file tree:

```text
my-ai-source/
  csaw.yml
  .csawignore
  AGENTS.md
  rules/
    go.md
  agents/
    reviewer.md
  skills/
    testing/
      SKILL.md
    experimental/
      new-workflow/
        SKILL.md
  mcp/
    claude-code.json
```

`csaw.yml` defines profiles:

```yaml
default:
  description: Default context
  include:
    - AGENTS.md
    - rules/**
    - agents/**
    - skills/**

backend:
  extends: default
  include:
    - mcp/claude-code.json
```

`.csawignore` hides files from default enumeration:

```text
skills/experimental/**
```

### Profile Rules

- `include` is a list of paths or glob patterns.
- `exclude` removes paths from the profile.
- `extends` inherits another profile.
- `include_ignored` allows files hidden by `.csawignore`.

### Exercise

Add this to `personal-ai/rules/personal.md`:

```markdown
# Personal Rule

Prefer small, testable changes.
```

Leave the profile unchanged and verify it still parses by mounting in the next
module. The starter `default` profile already includes `rules/**`, so this new
file is selected without editing `csaw.yml`.

## Module 4: First Mount

Mount the personal profile into the project:

```bash
cd ~/csaw-lab/app
csaw mount --profile personal/default
```

Inspect the result:

```bash
csaw status
csaw inspect
csaw check
git status --short
```

You should see mounted files, but `git status` should stay clean because csaw
adds project-local entries to `.git/info/exclude`.

Unmount:

```bash
csaw unmount
```

Restore the previous mount:

```bash
csaw mount --restore
```

### What To Learn

- Mounts are reversible.
- The project is not polluted with committed AI config.
- `status` is quick.
- `inspect` is detailed.
- `check` verifies mount health.

### Exercise

Before unmounting, create a local file at a path csaw wants to mount, then
mount with `--force` in a disposable repo. Confirm csaw stashes and restores the
original file when unmounting.

## Module 5: Artifact Kinds And Projection

csaw classifies AI workspace files by kind:

| Kind         | Registry path            | Project target         |
| ------------ | ------------------------ | ---------------------- |
| Instructions | `AGENTS.md`, `CLAUDE.md` | Project root           |
| Rules        | `rules/*.md`             | Tool rule directories  |
| Agents       | `agents/*.md`            | Tool agent directories |
| Skills       | `skills/*/SKILL.md`      | Tool skill directories |
| MCP          | `mcp/*.json`             | Tool MCP config paths  |

Tool projection means one registry shape can support multiple tools. For
example, `skills/review/SKILL.md` can mount to `.claude/skills/review/SKILL.md`
and `.opencode/skills/review/SKILL.md`.

The starter `default` profile includes instructions, rules, agents, and skills.
It does not include MCP files. Mount MCP by adding `mcp/**` to a profile or by
using an explicit qualified pattern.

Set preferred tools:

```bash
csaw config set tools claude,cursor,codex
```

Mount only some kinds:

```bash
csaw mount --profile personal/default --kind agents
csaw mount --profile personal/default --kind agents,skills
csaw mount 'personal/mcp/**' --tools claude
```

Control git visibility:

```bash
csaw show AGENTS.md
csaw hide AGENTS.md
```

### Exercise

Create one file of each kind in `personal-ai`. Mount the starter profile for
instructions, rules, agents, and skills. Mount MCP with the explicit
`'personal/mcp/**'` pattern or add `mcp/**` to `personal-ai/csaw.yml`. Predict
where each file should appear before running `csaw inspect`.

## Module 6: Multi-Source Composition

Create a team source:

```bash
cd ~/csaw-lab
csaw init team-ai --name team
csaw source add team ~/csaw-lab/team-ai --priority 0
```

Mount a team profile:

```bash
cd ~/csaw-lab/app
csaw mount --profile team/default
```

Mount personal and team context together with qualified source patterns:

```bash
csaw mount 'personal/**' 'team/**'
```

Quote glob patterns so your shell does not expand them before csaw sees them.
Profiles are source-qualified by owner: `team/default` resolves unqualified
profile includes such as `rules/**` against the `team` source. A profile can
also include qualified source paths, such as `personal/**` and `team/**`, if
you want a named composition profile.

When two sources provide different files, both mount. When they provide the
same target path, priority decides.

### Priority

Higher priority wins on normal conflicts:

```bash
csaw source add personal ~/csaw-lab/personal-ai --priority 10
csaw source add team ~/csaw-lab/team-ai --priority 0
```

If two sources have equal priority for the same target, csaw refuses the mount
until you make the decision explicit.

### Exercise

Put `AGENTS.md` in both `personal-ai` and `team-ai`. Change priorities and
observe which one mounts. Then set equal priorities and confirm csaw refuses the
ambiguous mount.

## Module 7: Protected Files

Protected files are source-owned requirements. Add this to `team-ai/csaw.yml`:

```yaml
csaw:
  protected:
    - AGENTS.md
    - rules/security.md
```

Protected behavior:

- Protected files bypass priority.
- Protected files cannot be forked.
- `inspect` shows protected files.
- Mount state records a SHA-256 hash for protected mounts.
- `check` and `audit` detect protected content drift.

Run:

```bash
csaw mount --profile team/default
csaw inspect
csaw check
```

### Protected Hashes

The hash is recorded at mount time. If the protected source intentionally
changes, remount to accept the new version and record a fresh hash.

This is local assurance, not hard enforcement. csaw detects drift; it does not
sandbox the operating system.

### Exercise

Mount a protected `AGENTS.md`, then edit the source file directly. Run:

```bash
csaw check
csaw audit
```

Confirm the detail includes `protected-content-drift`. Remount and confirm the
drift clears.

## Module 8: Audit Policy

Create a starter policy in a project:

```bash
cd ~/csaw-lab/app
csaw audit --init
```

This creates `.csaw/policy.yml` in the project. Treat it as a real project
policy file: commit it when the policy represents shared team, client, or CI
requirements.

Edit `.csaw/policy.yml`:

```yaml
required_sources:
  - team
  - name: client-acme
    url: git@example.com:org/client-acme-ai.git
    ref: approved
blocked_sources:
  - other-client-*
  - personal-experimental
required_kinds:
  - instructions
  - rules
  - mcp
```

Run:

```bash
csaw audit
csaw audit --strict
csaw audit --json
```

This example policy is intentionally stricter than the current lab state. Use
the failures to learn what audit checks before you make the policy pass.

`--json` writes the machine-readable report to stdout. If audit fails, the
human failure summary may still be written to stderr.

### What Audit Checks

- Policy file presence.
- Mounted link health.
- Protected content drift.
- Required sources.
- Required source configured URL.
- Required source project pin.
- Blocked source names and glob patterns.
- Required mounted kinds.

`--strict` fails on warnings as well as errors. A missing policy is a warning in
default mode and a failure in strict mode.

### JSON Contract

Use [reference/audit-json.md](reference/audit-json.md) for:

- Report shape.
- Finding IDs.
- Severity meanings.
- Mount health detail strings.
- CI integration expectations.

### Exercise

Make audit fail three different ways:

- Require a source that is not mounted.
- Add a blocked source pattern that matches an active source.
- Require a pin that does not match the project pin.

Then fix each failure.

## Module 9: Client Isolation Workbench

This is the strongest practical csaw workflow.

Create two client sources:

```bash
csaw init ~/csaw-lab/client-acme-ai --name client-acme
csaw init ~/csaw-lab/client-globex-ai --name client-globex
csaw source add client-acme ~/csaw-lab/client-acme-ai --priority 50
csaw source add client-globex ~/csaw-lab/client-globex-ai --priority 50
```

For an Acme project, policy should require Acme and block Globex:

```yaml
required_sources:
  - client-acme
blocked_sources:
  - client-globex
  - other-client-*
required_kinds:
  - instructions
```

Day in the life:

1. Enter the project.
2. Pull the client source.
3. Mount the client profile.
4. Run `csaw audit --strict`.
5. Work only after audit passes.
6. Run audit again before handoff or commit.
7. Unmount or switch context before moving to another client.

```bash
cd ~/work/client-acme-app
csaw pull client-acme
csaw mount --profile client-acme/default
csaw audit --strict
```

### Exercise

Intentionally mount the wrong client source and verify audit catches it. Then
switch to the correct source and verify audit passes.

## Module 10: Pinning Source Refs

Pinning is for remote sources. Local sources already point at a local working
tree, so change them with git directly. To practice pinning in the lab, turn
the team source into a disposable remote and re-register it:

```bash
cd ~/csaw-lab/app
csaw unmount

cd ~/csaw-lab/team-ai
git branch -M main
git add -A
git commit -m "seed team source"

cd ~/csaw-lab
git clone --bare team-ai team-ai.git
csaw source remove team
csaw source add team "file://$HOME/csaw-lab/team-ai.git" --priority 0
```

Pinning lets one project use a source branch or tag without changing other
projects:

```bash
cd ~/csaw-lab/app
csaw pin team@main
csaw mount --profile team/default
csaw inspect
```

In real use, replace `main` with the branch or tag the project should consume.

Unpin:

```bash
csaw unpin team
```

Policy can require the pin:

```yaml
required_sources:
  - name: team
    ref: main
```

The `ref` policy check uses csaw's project pin. It does not infer the current
branch from the source checkout.

### Exercise

Pin a source, require that pin in policy, and run `csaw audit --strict`. Change
the required ref and observe the failure.

## Module 11: Fork And Promote

Fork copies a source file into another source for customization:

```bash
csaw fork team/agents/reviewer.md --into personal
```

Protected files cannot be forked because the owning source marked them as
mandatory. Try `csaw fork team/AGENTS.md --into personal` after Module 7 and
confirm csaw refuses it.

Promote moves an experimental skill into the stable skill tree:

```bash
csaw mount --profile personal/default --include-experimental
csaw promote personal/skills/experimental/debugging
```

### Exercise

Fork an unprotected team file into `personal-ai`. Then create an experimental
skill, mount with `--include-experimental`, promote it, and verify it mounts
without `--include-experimental`.

## Module 12: Source Git Operations

Remote sources are normal git repos managed by csaw.

Add and pull:

```bash
csaw source add team git@example.com:org/team-ai.git
csaw pull team
```

Push a source change:

```bash
csaw push team -m "improve review rules"
```

Clone a source for normal PR workflow:

```bash
csaw source clone team ~/work/team-ai
cd ~/work/team-ai
git checkout -b improve-rules
```

Dirty source behavior:

- `csaw pull` refuses when the source has uncommitted changes.
- `csaw pull --stash` stashes, pulls, then pops the stash.
- Diverged sources need normal git resolution.

### Exercise

Make a local edit in a source and run `csaw pull`. Observe the refusal and the
suggested fix.

## Module 13: Drift, Repair, And Recovery

Run health checks:

```bash
csaw check
```

Common issues:

| Detail                      | Meaning                                    | Typical response                                    |
| --------------------------- | ------------------------------------------ | --------------------------------------------------- |
| `missing-source`            | Source file is gone.                       | Restore the source file or remount another profile. |
| `missing-link`              | Project path is gone.                      | Run `csaw update` or remount.                       |
| `replaced-link`             | Project path is no longer csaw-managed.    | Inspect manually; remount if intentional.           |
| `drifted-link`              | Link points at the wrong source.           | Run `csaw update` or remount.                       |
| `protected-content-drift`   | Protected file hash changed.               | Audit the change; remount if approved.              |
| `protected-hash-unreadable` | Hash verification could not read the file. | Check filesystem permissions and mount state.       |

Repair what csaw can repair:

```bash
csaw update
```

`csaw update` repairs missing and drifted links that can be reconstructed from
mount state.

Unmount and restore originals:

```bash
csaw unmount
```

Use `csaw diff <path>` to compare a mounted file with its source.

### Exercise

Delete a mounted link and run `csaw check`. Then run `csaw update` and confirm
the link is repaired.

## Module 14: Tooling And Visibility

csaw has two visibility layers:

- Project filesystem visibility: mounted files appear where tools expect them.
- Git visibility: mounted files are hidden by `.git/info/exclude` unless shown.

Commands:

```bash
csaw show AGENTS.md
git status --short
csaw hide AGENTS.md
git status --short
```

Use this sparingly. The default model is mounted local context, not committed
project context.

If you show a file under a hidden tool directory, the parent directory can
appear in `git status` until you hide the file again. Project policy files such
as `.csaw/policy.yml` are independent of mount visibility.

### Exercise

Show and hide a mounted file. Inspect `.git/info/exclude` before and after.

## Module 15: Designing Sources

### Personal Source

Use for preferences and reusable skills that should not be committed into every
repo.

Recommended contents:

- Personal coding preferences.
- Reusable skills.
- Optional agents.
- Experimental work under `skills/experimental/`.

### Team Source

Use for shared engineering standards.

Recommended contents:

- `AGENTS.md` with team conventions.
- Review rules.
- Testing standards.
- Common agents and skills.
- Protected security or compliance rules.

### Client Source

Use for engagement-specific constraints.

Recommended contents:

- Client-specific `AGENTS.md`.
- Required MCP config, if allowed.
- Protected policy and security rules.
- Project onboarding skills.

### Community Source

Use cautiously for reusable public workflows.

Recommended contents:

- Generic skills and agents.
- No secrets.
- No client-specific policy.
- Lower priority than team or client sources.

### Exercise

Sketch four source trees: personal, team, client, community. Mark which files
should be protected and which should remain optional.

## Module 16: CI And Automation

Use audit JSON for automation:

```bash
csaw audit --strict --json
```

A CI check should treat nonzero exit as failure and can parse `findings` for
reports. Keep the source of truth in `.csaw/policy.yml`.

Recommended local hooks or scripts:

```bash
csaw audit --strict
csaw check
```

Do not claim hard enforcement. This is a local assurance and detection layer.

### Exercise

Write a small shell script that runs `csaw audit --strict --json` and prints only
finding IDs with severity `error`.

## Module 17: Troubleshooting Playbook

### "No Sources Configured"

Run:

```bash
csaw source list
csaw source add personal ~/csaw-lab/personal-ai
```

### "No Profile Specified"

Use:

```bash
csaw mount --profile source/profile
```

or run interactively in a terminal.

### Mounted Files Show In Git

Run:

```bash
csaw hide <path>
git check-ignore -v <path>
```

### Audit Says Policy Missing

Run:

```bash
csaw audit --init
```

Then edit `.csaw/policy.yml`.

### Protected Drift Appears After Source Update

Review the source update. If approved:

```bash
csaw mount --restore
```

or remount the intended profile.

### Windows Link Issues

csaw uses symlinks where available and hardlinks as a fallback. Hardlinks can
drift when the source file is replaced. Use `csaw check` and `csaw update`.

## Module 18: Expert Mental Models

### Mount, Not Install

Mounted files are links to source-owned files. They are not copied into the
project as durable project files.

### Sources Are Normal Repos

Use git for review, history, branching, and remote collaboration. csaw does not
replace git; it makes AI workspace files composable and project-local.

### Profiles Are Product Surfaces

A profile should map to real work: backend, frontend, incident, client, writing,
review, onboarding.

### Protected Means "Must Win And Must Match"

Protected files win composition and are hash-verified after mount. They are
still local files on a user-controlled machine.

### Audit Is Evidence

Audit answers "what context is active and does it match policy?" It does not
prevent all possible misuse.

### Repo-Local First

The cleanest solution is often a committed project file. Use csaw only where
there is a cross-boundary reason.

## Module 19: Contributing To csaw

Read first:

- [../README.md](../README.md)
- [../ARCHITECTURE.md](../ARCHITECTURE.md)
- [../AGENTS.md](../AGENTS.md)
- [reference/project-management.md](reference/project-management.md)

Package map:

| Package              | Responsibility                                        |
| -------------------- | ----------------------------------------------------- |
| `cmd/csaw`           | CLI wiring and command behavior.                      |
| `internal/runtime`   | Paths, constants, normalization helpers.              |
| `internal/sources`   | Source config, git operations, catalogs.              |
| `internal/profiles`  | `csaw.yml` parsing and inheritance.                   |
| `internal/mount`     | Planning, projection, priority, protected resolution. |
| `internal/workspace` | Stash, excludes, mount state, hashes.                 |
| `internal/drift`     | Mounted link and protected content health.            |
| `internal/audit`     | Project policy, findings, renderers, exit semantics.  |
| `internal/pinning`   | Per-project source refs.                              |
| `internal/fork`      | Forking files between sources.                        |
| `internal/inspect`   | Human-readable state summaries.                       |

Before committing:

```bash
gofmt -l .
go test ./...
go vet ./...
go build ./...
```

Use the repo-local skills in `skills/` when a task matches.

### Curriculum Maintenance Checklist

When a feature changes, update this curriculum if any answer changes for:

- What problem csaw solves.
- How a user initializes, mounts, audits, repairs, or removes context.
- What files belong in a source.
- What `csaw.yml`, `.csawignore`, or `.csaw/policy.yml` supports.
- What `inspect`, `check`, `audit`, or JSON output reports.
- What commands or flags exist.
- What the recommended client/team workflow is.

## Capstone 1: Personal AI Workspace

Build a personal source with:

- One instruction file.
- Two rules.
- One agent.
- One stable skill.
- One experimental skill.

Mount it into a disposable project, inspect it, audit it, promote the
experimental skill, and unmount cleanly.

## Capstone 2: Team Governance

Build a team source with:

- Protected `AGENTS.md`.
- Protected security rule.
- Review agent.
- Testing skill.

Mount it with a personal source that tries to override the same files. Confirm
protected files win, inspect shows protection, and audit/check can detect
protected content drift.

## Capstone 3: Client Isolation

Build two client sources and one project policy. Prove:

- The correct client source is required.
- The wrong client source is blocked.
- Required kinds are present.
- Audit fails before work when context is wrong.
- Audit passes only after the correct mount.

## Capstone 4: Source Lifecycle

Using a remote or disposable git source:

- Add the source.
- Pull it.
- Pin it to a branch or tag.
- Require the pin in policy.
- Fork a non-protected file into personal.
- Promote a skill.
- Push a source change.
- Unpin and return to default.

## Expert Rubric

You are operating at expert level when you can:

- Explain csaw's value without hand-waving over "why not just git?"
- Predict mount output before running `csaw mount`.
- Debug every `csaw check` issue without data loss.
- Write a client isolation policy from memory.
- Decide when to protect, pin, fork, promote, or keep repo-local.
- Design a source layout that works across Claude Code, Codex, Cursor,
  Windsurf, OpenCode, Copilot, and Gemini CLI.
- Review a csaw code change and identify which docs, tests, and workflows need
  updates.
