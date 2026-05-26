# csaw Curriculum

This curriculum is the guided path from first use to expert operation of csaw.
Follow it top to bottom and do the exercises in a disposable project.

Maintenance contract: update this document whenever csaw changes user-facing
commands, config files, artifact kinds, projection behavior, audit policy,
drift detection, release flow, or recommended workflows.

## Learning Outcomes

After completing this curriculum, you should be able to:

- Explain when csaw is useful and when repo-local context is better.
- Design company, department, team, personal, client, and community AI workspace sources.
- Adopt existing repo-local AI files into a reusable source.
- Use profiles, priorities, protected files, pins, fork, promote, and restore.
- Predict where instructions, rules, agents, skills, MCP, hooks, and ignore files mount.
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
  skills, MCP server configuration, lifecycle hooks, and context-ignore patterns.

Use generic test directories throughout the curriculum. Do not use real client
or production secrets in exercises.

## Curriculum Project Setup

Create a disposable workspace:

```bash
mkdir -p ~/csaw-lab
cd ~/csaw-lab
git init app
```

Use `~/csaw-lab/app` as the project. Source directories (`personal-ai`,
`company-ai`, `department-ai`, `team-ai`, `client-acme-ai`, `client-globex-ai`)
are created by `csaw init` in their respective modules. Local sources are enough
to learn the behavior; later modules cover remote git sources.

> **Note:** Modules 1-5 work with a single personal source to teach the mechanics. Module 6 introduces the canonical 4-tier stack (`company` → `department` → `team` → `personal`) and creates the additional source directories. Module 9 covers horizontal client isolation.

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
- A company, department, team, or client owns required context.
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
| Kind           | One of instructions, rules, agents, skills, MCP, hooks, ignore, or other. |
| Policy         | `.csaw/policy.yml`, checked by `csaw audit`.                 |
| Pin            | A per-project source ref set with `csaw pin source@ref`.     |
| Protected file | A file a source marks as mandatory and non-overridable.      |

### Exercise

Answer these before continuing:

- Which AI files in your real work belong in a single repo?
- Which ones cross repo, company, department, team, client, privacy, or tool boundaries?
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

### Two Roles

You just played both roles at once: you **maintained** the personal source (created it, will edit its files) and you'll **consume** it from projects (`csaw use`, `csaw pull`). Most users only consume sources someone else maintains — your team source, your company source. For csaw to work in your org, you need exactly one maintainer per shared source; everyone else can be a passive consumer.

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

`.csawignore` hides files from default enumeration (custom patterns beyond the built-in `experimental/` convention):

```text
drafts/**
archived/**
```

Separately, **any path segment named exactly `experimental` is hidden by csaw's built-in convention** — at any depth, across all kinds (`skills/experimental/`, `rules/experimental/`, `agents/experimental/`, `hooks/experimental/`). No `.csawignore` entry needed.

### Profile Rules

- `include` is a list of paths or glob patterns.
- `exclude` removes paths from the profile.
- `extends` inherits another profile.
- `includeIgnored` (YAML: `includeIgnored: true`) allows files matched by `.csawignore` patterns.
- `includeExperimental` (YAML: `includeExperimental: true`) allows files under any `experimental/` segment.

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
csaw profile list
csaw profile show personal/default
csaw use personal/default
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

### Existing Project Adoption

If a repo already has AI files that should become reusable source-owned
context, adopt them into a registry:

```bash
cd ~/csaw-lab/app-with-existing-ai-files
csaw init --adopt ~/csaw-lab/adopted-ai --name adopted
csaw source add adopted ~/csaw-lab/adopted-ai --priority 20
```

Adoption copies recognized AI files into the registry and reverses tool
projection. For example, `.claude/skills/testing/SKILL.md` becomes
`skills/testing/SKILL.md`, `.claude/rules/go.md` becomes `rules/go.md`, and
`.vscode/mcp.json` becomes `mcp/vscode.json`.

Adoption does not delete the originals from the project. Before mounting the
adopted source back into the same repo, intentionally remove the originals,
mount with `--force`, or mount with `--skip-conflicts`.

## Module 5: Artifact Kinds And Projection

csaw classifies AI workspace files by kind:

| Kind         | Registry path                                                | Project target              |
| ------------ | ------------------------------------------------------------ | --------------------------- |
| Instructions | `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.goosehints`         | Project root                |
| Rules        | `rules/*.md`                                                 | Tool rule directories       |
| Agents       | `agents/*.md`                                                | Tool agent directories      |
| Skills       | `skills/*/SKILL.md`                                          | Tool skill directories      |
| MCP          | `mcp/*.json`                                                 | Tool MCP config paths       |
| Hooks        | `hooks/*`                                                    | Tool hook directories       |
| Ignore       | `ignore/*` (one file per tool)                               | Tool-specific ignore paths  |

csaw deliberately does **not** project two additional kinds you might expect: `settings` (contains API keys and credentials — never project to a team source) and `memory` (today this is session state or user-private — revisit if a portable standard emerges). See [planning/projection-roadmap.md](planning/projection-roadmap.md).

Tool projection means one registry shape can support multiple tools. For
example, `skills/review/SKILL.md` can mount to `.claude/skills/review/SKILL.md`
and `.opencode/skills/review/SKILL.md`. **GitHub Copilot is the rewriting case**: `rules/security.md` projects to `.github/instructions/security.instructions.md` (suffix is mandatory for Copilot; csaw applies it automatically).

The starter `default` profile includes instructions, rules, agents, and skills.
It does not include MCP, hooks, or ignore files. Mount those by adding
`mcp/**`, `hooks/**`, or `ignore/**` to a profile or by using explicit
qualified patterns.

Set preferred tools:

```bash
csaw config set tools claude,cursor,codex
```

Mount only some kinds:

```bash
csaw use personal/default --kind agents
csaw use personal/default --kind agents,skills
csaw mount paths 'personal/mcp/**' --tools claude
```

Control git visibility:

```bash
csaw show AGENTS.md
csaw hide AGENTS.md
```

### Exercise

Create one file of each kind in `personal-ai`:

```
personal-ai/
├── AGENTS.md                          # instructions
├── rules/security.md                  # rules
├── agents/code-reviewer.md            # agents
├── skills/code-review/SKILL.md        # skills
├── mcp/claude-code.json               # mcp
├── hooks/pre-commit.sh                # hooks
└── ignore/cursor                      # ignore (gitignore-style patterns)
```

Mount the starter profile for instructions, rules, agents, and skills. Then mount the rest explicitly: `csaw mount paths 'personal/mcp/**' 'personal/hooks/**' 'personal/ignore/**'` — or add `mcp/**`, `hooks/**`, `ignore/**` to `personal-ai/csaw.yml`. Predict where each file should appear before running `csaw inspect`.

The interesting projections to predict:
- `hooks/pre-commit.sh` → `.claude/hooks/pre-commit.sh` (only claude has `HooksSubdir` today).
- `ignore/cursor` → `.cursorignore` (single-file alias, like MCP).
- If you have copilot in your tools config, `rules/security.md` *also* lands at `.github/instructions/security.instructions.md` with the suffix rewritten. Like every other csaw projection, it's hidden from git by default. To make it visible for PR review (the conventional Copilot pattern), run `csaw show .github/instructions/security.instructions.md` as an explicit opt-in.

## Module 6: Multi-Source Composition

Configured sources are inventory, not active context. A source becomes active only when a mount selection includes files from it. `csaw inspect` and `csaw audit` report the active mounted context, not every registered source.

### The Canonical Stack

The typical real-world csaw setup is four nested layers, each owned by someone different:

| Layer      | Priority         | Typical owner                     |
| ---------- | ---------------- | --------------------------------- |
| company    | 100, protected   | Platform / engineering ops        |
| department | 80, protected    | Department head or staff engineer |
| team       | 50               | Your team                         |
| personal   | 10               | You                               |

Higher tiers outrank lower ones on conflict; protected files in higher tiers cannot be overridden at all (Module 7). You already have `personal-ai` from Module 2 — set up the other three:

```bash
cd ~/csaw-lab
csaw init company-ai --name company
csaw init department-ai --name department
csaw init team-ai --name team
csaw source add company ~/csaw-lab/company-ai --priority 100
csaw source add department ~/csaw-lab/department-ai --priority 80
csaw source add team ~/csaw-lab/team-ai --priority 50
```

### Activating One Source

Mounting one source's profile mounts only that source:

```bash
cd ~/csaw-lab/app
csaw profile list
csaw use team/default
```

### Composing the Whole Stack

To mount all four layers in one work mode, define a composed profile in your personal source's `csaw.yml`:

```yaml
work:
  description: Company + department + team + personal helpers
  extends:
    - company/default
    - department/default
    - team/default
  include:
    - skills/**
    - agents/**
```

Activate it:

```bash
csaw use personal/work
```

Profile names are source-qualified by owner: `team/default` resolves against the `team` source. Because `work` lives in `personal`, `skills/**` resolves to `personal/skills/**`. The day-to-day string is `personal/work` — self-documenting.

For one-off debugging, `csaw mount paths 'company/**' 'team/**' 'personal/**'` still works. Quote glob patterns so your shell does not expand them. Treat raw patterns as an escape hatch, not the normal workflow.

When two sources provide different files, both mount. When they provide the same target path, priority decides.

### Horizontal Sources

The four-tier canonical is one **vertical** hierarchy — each layer nests inside the one above it. Sources can also be **horizontal** — opted into by responsibility rather than position: a `client` source for consultants, a `staff-eng` source for cross-team cost-awareness, a `community` source for shared open-source skills.

Horizontal composition uses the same pattern. For client work:

```yaml
client-extras:
  description: Personal helpers safe for client projects
  include:
    - skills/code-review/**
    - agents/planner.md

acme-work:
  description: Acme client context with personal helpers
  extends:
    - client-acme/default
    - client-extras
```

Then:

```bash
csaw use personal/acme-work
```

Module 9 covers client isolation in detail.

### Priority Mechanics

Higher priority wins on conflicts:

```bash
csaw source add company ~/csaw-lab/company-ai --priority 100
csaw source add team ~/csaw-lab/team-ai --priority 50
csaw source add personal ~/csaw-lab/personal-ai --priority 10
```

If two sources have equal priority for the same target, csaw refuses the mount until you make the decision explicit.

### Exercise

1. Put a different `AGENTS.md` in each of `company-ai`, `team-ai`, and `personal-ai`. Mount `personal/work` and confirm company's wins (highest priority).
2. Set `personal` priority to 200. Re-mount. Confirm personal now wins.
3. Set personal and company to equal priority. Re-mount. Confirm csaw refuses the ambiguous mount.

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
csaw use team/default
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
policy file: commit it when the policy represents shared company, team, client,
or CI requirements.

Edit `.csaw/policy.yml`. The canonical case is "this repo must mount the company and team sources, and not personal-experimental":

```yaml
required_sources:
  - company
  - name: team
    url: git@example.com:org/team-ai.git
    ref: main
blocked_sources:
  - personal-experimental
blocked_kinds:
  - mcp
required_kinds:
  - instructions
  - rules
```

Module 9 shows the stricter consultant variant with `blocked_paths` and multiple client sources.

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
- Blocked mounted kinds.
- Blocked mounted project paths.
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

Client isolation is the canonical **horizontal** csaw scenario — sources opted into by responsibility (here, "which client am I working for") rather than by vertical org position. The same compose-and-activate pattern from earlier modules applies; what makes it distinct is using project policy to prove the right client is mounted and the wrong ones aren't.

The key distinction is configured versus active. It is fine to have both Acme
and Globex registered globally. They should not both be active in the same
project unless a mount selection explicitly includes both. Project policy is
the local proof that the correct client source is active and the wrong client
source is absent.

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
blocked_kinds:
  - mcp
blocked_paths:
  - .claude/agents/**
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
csaw use client-acme/default
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
csaw use team/default
csaw inspect
```

In real use, replace `main` with the branch or tag the project should consume.
csaw uses a project-specific detached worktree for the pinned ref.

Unpin:

```bash
csaw unpin team
csaw check
```

Unpin removes the project pin and updates active mounts for that source back to
the default checkout.

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
csaw use personal/default --include-experimental
csaw promote personal/skills/experimental/debugging
```

### Exercise

Fork an unprotected team file into `personal-ai`. Then create an experimental
skill, activate with `--include-experimental`, promote it, and verify it mounts
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

Use `csaw diff <path>` to compare a replaced mounted file with its source.
For a healthy symlink, the project path and source path are the same content,
so there may be no meaningful content diff.

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

**GitHub Copilot doesn't change the rule.** Even though `.github/instructions/` is the GitHub-blessed location for shared team context that many teams want committed to git, csaw still hides projections there by default — consistent with everything else. Opting in is an explicit team decision: run `csaw show .github/instructions/*` (or per-file) to make them visible to PR reviewers.

### Exercise

Show and hide a mounted file. Inspect `.git/info/exclude` before and after.

## Module 15: Designing Sources

The canonical csaw setup is a **vertical** 4-tier stack — `company` → `department` → `team` → `personal`. **Horizontal** sources (`client`, `community`, role-based) layer in by responsibility rather than position. Each source type has a different shape, owner, and protection profile.

### Company Source

Organization-wide engineering standards every repo should reflect. Maintained by platform / engineering ops. Priority 100. Most contents protected.

Recommended contents:

- `AGENTS.md` with company-wide conventions.
- Protected security and compliance rules.
- Required cross-tool standards.

### Department Source

Cross-team conventions inside a department (backend, frontend, data, etc.). Maintained by department head or staff engineer. Priority 80. Mandatory items protected.

Recommended contents:

- Department-specific `AGENTS.md` additions.
- Protected workflow skills like a PR-workflow procedure.
- Department-wide review rules.

### Team Source

Shared engineering standards across team-owned repos. Maintained by team lead. Priority 50.

Recommended contents:

- `AGENTS.md` with team conventions.
- Review rules.
- Testing standards.
- Common agents and skills.
- Protected security or compliance rules where the team has authority to mandate.

### Personal Source

Your own preferences and reusable skills that should not be committed into any repo. Maintained by you. Priority 10.

Recommended contents:

- Personal coding preferences.
- Reusable skills.
- Optional agents.
- Experimental work under `skills/experimental/`.

### Client Source (horizontal)

Engagement-specific constraints when consulting. Maintained by client OR by you per-engagement. Priority varies; pair with project policy that blocks other clients.

Recommended contents:

- Client-specific `AGENTS.md`.
- Required MCP config, if allowed.
- Protected policy and security rules.
- Project onboarding skills.

### Community Source (horizontal)

Reusable public workflows. Use cautiously. Lower priority than company, department, team, or client sources.

Recommended contents:

- Generic skills and agents.
- No secrets.
- No client-specific policy.

### Exercise

Sketch source trees for the canonical vertical stack (company, department, team, personal) plus horizontal sources you'd realistically use (client, community, role-based). Mark which files should be protected, who maintains each source, and which priority they get.

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
csaw use source/profile
```

or run interactively in a terminal.

### Mounted Files Show In Git

Run:

```bash
csaw hide <path>
git check-ignore -v <path>
```

If your team has run `csaw show .github/instructions/*` to make Copilot context visible for PR review, that's expected behavior — `csaw hide` will reverse it.

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
| `cmd/tools-gen`      | Renders `docs/reference/tool-projection.md` from `tool-projection.json`. Run via `go generate ./internal/mount/...`. CI fails if the generated file is out of date. |
| `internal/runtime`   | Paths, constants, normalization helpers.              |
| `internal/sources`   | Source config, git operations, catalogs.              |
| `internal/profiles`  | `csaw.yml` parsing and inheritance.                   |
| `internal/mount`     | Planning, projection, priority, protected resolution. `ToolRegistry` lives here; `cmd/tools-gen` reads its JSON counterpart for docs. |
| `internal/workspace` | Stash, excludes, mount state, hashes.                 |
| `internal/drift`     | Mounted link and protected content health.            |
| `internal/audit`     | Project policy, findings, renderers, exit semantics.  |
| `internal/pinning`   | Per-project source refs.                              |
| `internal/fork`      | Forking files between sources.                        |
| `internal/inspect`   | Human-readable state summaries.                       |

Before committing:

```bash
go generate ./internal/mount/...   # regenerate docs/reference/tool-projection.md
gofmt -l .
go test ./...
go vet ./...
go build ./...
```

If you touched `docs/reference/tool-projection.json` or anything in `internal/mount/tools.go`, the `go generate` step is mandatory — CI verifies the markdown is in sync.

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

- One instruction file (`AGENTS.md`).
- Two rules (`rules/*.md`).
- One agent (`agents/*.md`).
- One stable skill (`skills/<name>/SKILL.md`).
- One experimental skill (under `skills/experimental/<name>/`).
- One hook script (`hooks/*.sh` — e.g., a pre-commit linter).
- One ignore file (`ignore/cursor` — gitignore-style patterns excluding `node_modules/` and `dist/`).

Mount it into a disposable project, inspect it, audit it, promote the
experimental skill, and unmount cleanly. Confirm with `git status` and `.git/info/exclude` that all projected files are hidden by default — including any Copilot projections under `.github/`. If you want Copilot's instructions visible for team PR review, run `csaw show .github/instructions/*` as a separate, explicit step.

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
  Windsurf, OpenCode, Copilot, and Antigravity.
- Review a csaw code change and identify which docs, tests, and workflows need
  updates.
