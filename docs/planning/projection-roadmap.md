# Tool Projection Roadmap

Planning doc for what csaw projects (and doesn't) across AI coding tools. Living document — update as decisions land.

Source of truth for *what csaw actually projects today* is [`docs/reference/tool-projection.json`](../reference/tool-projection.json) (tools with `csaw_in_code: true`). This file is for what's *coming* or explicitly *out of scope*.

## Currently in code (v0.7.0)

Seven tools with `"csaw_in_code": true`: **claude, cursor, codex, opencode, copilot, antigravity, goose**. The projection test (`internal/mount/projection_consistency_test.go`) enforces that the JSON's claims match the actual Go `ToolRegistry`.

## Coming up

- **`.github/prompts/` (Copilot single-file prompts)** — Deferred from v0.7.0. csaw's skills are folder-based (`<name>/SKILL.md`); Copilot prompts are single files (`<name>.prompt.md`). Either map skills folders to prompt files (semantically awkward) or add a new `prompts` kind (8th kind, big surface). Revisit if Copilot users ask.
- **`.github/copilot-instructions.md` alias** — Deferred from v0.7.0. Would project the project's `AGENTS.md` to this canonical Copilot location as a second symlink. Today Copilot reads `AGENTS.md` at project root, so the alias is nice-to-have, not required.

## Auto-served via AGENTS.md (no setup needed)

These tools read the cross-tool `AGENTS.md` standard; csaw's instructions kind projects to project root by default. No `ToolRegistry` entry needed:

- **GitHub Copilot** (universal coverage today; deep `.github/` projection still coming)
- **Factory Droid** — `AGENTS.md` only
- **Pi** — `AGENTS.md` + `CLAUDE.md` only (MCP lives in `~/.pi/agent/`, out of scope)
- **Hermes** — reads `.cursor/rules/`, `.cursorrules`, `AGENTS.md`, `CLAUDE.md`
- **Cline, Aider, Continue, Amp, Augment, Devin, Factory, Plandex** and ~20 others

## Out of scope with current model

- **zed** — settings.json user-scope only. Project `.rules` is single file.
- **cody** — Project `.vscode/cody.json` content schema is Cody-specific (custom commands JSON).
- **qodo** — Code review tool; hierarchical `best_practices.md` is a different model.
- **lingma** — Mostly IDE-internal; docs primarily Chinese.

## Explicitly never (privacy / out of scope by design)

- **Settings** as a csaw kind — settings files contain API keys and personal credentials. Mounting them to a team source would leak secrets. Document tool settings paths as reference only.

## Future consideration

- **Memory** as a csaw kind — most "memory" today is session state or user-private. csaw is project-scope. Revisit if a portable memory standard emerges.
- **goose recipes** at `.goose/recipes/` — recipes are YAML files, not csaw's SKILL.md folder pattern. Recipes can be a future kind if there's demand, but the .goosehints file (now recognized as `instructions`) covers the common Goose use case.

## Removed

- **gemini** (removed v0.6.0) — Google sunset 2026-06-18. Migration target: `antigravity` (`.agents/`, also csaw's StandardFallback). `GEMINI.md` is still recognized as an instruction file because Antigravity reads it.
- **amazon-q, kiro, openhands, windsurf, codebuddy** (removed v0.6.1) — Trimmed from `ToolRegistry` to keep projection focused on the tools users actually ask about. All five remain catalogued in `tool-projection.json` (with `csaw_in_code: false`) and can be re-added in a single commit if user demand emerges.

## Watch list

- **claw-code** — Reuses `.claude/` paths; auto-served by csaw's Claude projection.
- **joycode** — ECC adapter; not a standalone tool.
- **openclaw** — Reference architecture; ecosystem of derivatives.
