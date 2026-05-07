# Product Overview

csaw (like "see-saw") is a CLI for **multi-source AI workspace governance**. It mounts AI configuration — instructions, rules, agents, skills, and MCP server definitions — from one or more git-backed sources into your projects, with priority-based composition, protected files that can't be overridden, per-project pinning to specific git refs, and forkable lineage between sources.

## Who it's for

You have **more than one source of AI configuration truth**:

- **Staff engineers across multiple clients or product teams** — each codebase has its own rules, skills, and policies you must respect.
- **Contractors and consultants juggling clients** — each engagement has its own MCP servers, conventions, and security posture, and these must not bleed across projects.
- **Teams with mandated AI policy** — a security or platform team publishes config that engineers must use, with personal preferences layered on top without breaking the mandate.
- **Individuals composing personal config** with team or community sources.

If you only manage one set of AI files, simpler tools work fine. csaw earns its complexity when you have multiple stakeholders in your AI workspace.

## How it works

You declare one or more **sources** — git repos or local directories containing AI config. csaw symlinks files from sources into your project, hidden from git via `.git/info/exclude`. You can:

- Compose multiple sources with **priority** — higher number wins on overlap.
- Mark files as **protected** in a source so they cannot be overridden by lower-priority layers.
- **Pin** a source to a branch or tag for a single project without affecting others.
- **Fork** a file from one source into another for personal customization, with the original untouched.
- **Promote** experimental skills to stable when you're ready to share them.
- **Mount selectively by kind** (agents, skills, rules, mcp, instructions).
- **Inspect** the resolved state — which sources, which mounted files grouped by kind, what's protected, what's pinned, what's healthy.

Update a source — every project sees the change instantly through the symlinks. Unmount, and originals stashed during mount are restored.

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
- **No hidden defaults.** `csaw inspect` shows the full resolved state — what's mounted, where it came from, which source, whether it's protected, whether it's healthy.
- **Files, not formats.** csaw manages standard files (AGENTS.md, SKILL.md, plain markdown). Every file in a source is usable without csaw.
- **Multi-source composition with provenance.** Layer team, client, personal, and community sources. Priority and protection make the policy explicit. Every value annotated with its origin.
- **Cross-platform.** Linux, macOS, and Windows (junctions for directory symlinks).

## Where to learn more

- [README.md](../../README.md) — install, quick start, scenario-based walkthroughs, command reference.
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — package structure and interfaces.
- [Cheat sheet](../cheatsheet.md) — concise command reference.
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — contributor workflow.
- [Distribution strategy](../reference/distribution.md) — how csaw is packaged and released.
