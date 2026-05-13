# Product Overview

csaw (like "see-saw") is a CLI for **multi-source AI workspace governance**. It mounts AI configuration — instructions, rules, agents, skills, and MCP server definitions — from one or more git-backed sources into your projects, with priority-based composition, protected files that can't be overridden, per-project pinning to specific git refs, and forkable lineage between sources.

## Who it's for

You have **more than one source of AI configuration truth**:

- **Engineers in companies with shared standards** — you want company standards, team conventions, and personal preferences composed into every repo, kept live, never committed.
- **Teams or staff engineers publishing AI policy** — you ship rules and skills that lower layers can't silently override, with audit catching drift.
- **Individuals composing personal config** with team or community sources.
- **Contractors and consultants juggling clients** — each engagement has its own MCP servers and conventions that must not bleed across projects.

**csaw is the wrong tool if:** you have one repo, one team, and no personal additions to layer on top. Commit your `AGENTS.md` and stop here.

If a context file belongs to one repo and is safe to commit there, keep it in that repo. csaw is for the cases where repo-local files stop being enough: client isolation, private personal context, team-wide policy, reusable agents and skills, cross-tool projection, independent pinning, and local auditability.

## How it works

You declare one or more **sources** — git repos or local directories containing AI config — and activate named **profiles** with `csaw use source/profile`. csaw symlinks files from sources into your project, hidden from git via `.git/info/exclude`. You can:

- Compose multiple sources with named profiles and **priority** — higher number wins on overlap.
- **List and inspect profiles** before activating them.
- Mark files as **protected** in a source so they cannot be overridden by lower-priority layers.
- **Pin** a source to a branch or tag for a single project without affecting others.
- **Fork** a file from one source into another for personal customization, with the original untouched.
- **Promote** experimental skills to stable when you're ready to share them.
- **Mount selectively by kind** (agents, skills, rules, mcp, instructions).
- **Inspect** the resolved state — which sources, which mounted files grouped by kind, what's protected, what's pinned, what's healthy.
- **Audit** active context against `.csaw/policy.yml` for required sources, blocked sources, required and blocked kinds, blocked mounted paths, mount health, and protected content drift.

Update an already-mounted source file and every project sees the change instantly through the symlinks. Add a new source file, and remount the profile to add its symlink. Unmount, and originals stashed during mount are restored.

## The five kinds

csaw treats AI workspace artifacts as five distinct kinds, each with its own conventions and projection target:

| Kind | Registry path | Projects to | When loaded |
|---|---|---|---|
| Instructions | `AGENTS.md`, `CLAUDE.md` | Project root | Every turn — always in context |
| Rules | `rules/*.md` | `.claude/rules/`, `.cursor/rules/`, etc. | Every turn — always-on coding standards |
| Agents | `agents/*.md` | `.claude/agents/`, `.cursor/agents/`, etc. | When invoked — specialized subagent personas |
| Skills | `skills/*/SKILL.md` | `.claude/skills/`, `.opencode/skills/`, etc. | When relevant — on-demand procedural workflows |
| MCP | `mcp/*.json` | `.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` | Session start — tool/data connectivity |

## Design principles

- **Mount, not install.** Symlinks from a registry, not copies committed to your repo. Reversible, live, clean.
- **Repo-local first.** Project-owned context belongs in the project. csaw manages context that crosses repo, tool, team, client, privacy, or provenance boundaries.
- **No hidden defaults.** `csaw inspect` shows the full resolved state — what's mounted, where it came from, which source, whether it's protected, whether it's healthy.
- **Files, not formats.** csaw manages standard files (AGENTS.md, SKILL.md, plain markdown). Every file in a source is usable without csaw.
- **Multi-source composition with provenance.** Layer company, department, team, personal, client, and community sources. Priority and protection make the policy explicit. Every value annotated with its origin.
- **Local assurance, not hard enforcement.** `csaw audit` detects active context drift and policy violations; it does not sandbox the machine or prevent manual edits outside csaw.
- **Cross-platform.** Linux, macOS, and Windows (junctions for directory symlinks).

## Where to learn more

- [README.md](../../README.md) — install, quick start, scenario-based walkthroughs, command reference.
- [Roadmap](roadmap.md) — current release state, near-term priorities, and longer-term product tracks.
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — package structure and interfaces.
- [Cheat sheet](../cheatsheet.md) — concise command reference.
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — contributor workflow.
- [Distribution strategy](../reference/distribution.md) — how csaw is packaged and released.
