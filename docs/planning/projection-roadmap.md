# Tool Projection Roadmap

Planning doc for what csaw projects (and doesn't) across AI coding tools. Living document — update as decisions land.

Source of truth for *what csaw actually projects today* is [`docs/reference/tool-projection.json`](../reference/tool-projection.json) (tools with `csaw_in_code: true`). This file is for what's *coming* or explicitly *out of scope*.

## Currently in code (v0.8.1)

Seven tools with `"csaw_in_code": true`: **claude, cursor, codex, opencode, copilot, antigravity, goose**. The projection test (`internal/mount/projection_consistency_test.go`) enforces that the JSON's claims match the actual Go `ToolRegistry`.

## Coming up

- **`.github/prompts/` (Copilot single-file prompts)** — Deferred from v0.7.0. csaw's skills are folder-based (`<name>/SKILL.md`); Copilot prompts are single files (`<name>.prompt.md`). Either map skills folders to prompt files (semantically awkward) or add a new `prompts` kind (8th kind, big surface). Revisit if Copilot users ask.
- **`.github/copilot-instructions.md` alias** — Deferred from v0.7.0. Would project the project's `AGENTS.md` to this canonical Copilot location as a second symlink. Today Copilot reads `AGENTS.md` at project root, so the alias is nice-to-have, not required.

## Partial coverage via AGENTS.md — deep projection candidates

These tools read the cross-tool `AGENTS.md` standard, so csaw's existing instructions kind gives them *some* coverage. But each has additional native rules/skills/hooks/MCP directories that csaw does NOT yet project to. Listed in rough priority order of deep-projection value (prioritize by user demand):

- **Cline** — *highest value.* Reads `.clinerules/*.md` (multi-file rules dir with YAML frontmatter `paths:` filter), `.clineignore`, `hooks`, custom commands, MCP, skills. csaw covers ~10% of Cline's surface via AGENTS.md alone. Source: [docs.cline.bot/customization/cline-rules](https://docs.cline.bot/customization/cline-rules.md).
- **Continue** — Reads `.continue/rules/*.md`, `.continue/prompts/*.md`, `config.yaml`. csaw covers ~20%. Source: [docs.continue.dev](https://docs.continue.dev/customize/deep-dives/rules).
- **Factory Droid** — Reads `.factory/skills/<name>/SKILL.md` (same agentskills.io standard as csaw's `skills/`), `.factory/rules/`, hooks, MCP, custom droids, plugins. Skills directory is the easiest win — exact same SKILL.md format csaw already uses. Source: [docs.factory.ai](https://docs.factory.ai/cli/configuration/skills.md).
- **Augment** — Reads `.augment/rules/*.md` (multi-file rules with Always/Manual/Auto activation modes), plus hierarchical `AGENTS.md`/`CLAUDE.md` in subdirectories. Source: [docs.augmentcode.com/setup-augment/guidelines](https://docs.augmentcode.com/setup-augment/guidelines).
- **Amp** — Reads `.agents/skills/<name>/SKILL.md` (csaw already covers via antigravity/StandardFallback projection!), `.agents/checks/` (review criteria, novel concept), `.amp/plugins/*.ts`. Skills projection is already free; checks would be a new kind. Source: [ampcode.com/manual](https://ampcode.com/manual).
- **Aider** — Reads `CONVENTIONS.md` (Aider-specific equivalent of AGENTS.md), `.aider.conf.yml`, `.aiderignore`. Quick win: add `CONVENTIONS.md` to the instruction file recognition list. Source: [aider.chat/docs/usage/conventions.html](https://aider.chat/docs/usage/conventions.html).
- **Hermes** — Reads `.cursor/rules/`, `.cursorrules`, `AGENTS.md`, `CLAUDE.md`. csaw's cursor projection covers most of this for free.
- **Pi** — Reads `AGENTS.md` + `CLAUDE.md` only at project scope (MCP lives in `~/.pi/agent/`, out of scope). AGENTS.md is the full project surface — true "already served." (Verify quarterly.)

**Quick wins to consider:**
- Add `CONVENTIONS.md` to instruction file recognition (one-line change, covers Aider's main project file).
- Add `factory` to `ToolRegistry` with `Dir: ".factory", SkillsSubdir: "skills"` — Factory Droid uses the same agentskills.io standard csaw already projects.
- Add `cline` to `ToolRegistry` — but Cline's `.clinerules/` is a top-level dir (not nested under `.cline/`), so the existing `Dir` + `Subdir` shape doesn't quite fit. Either model it as `Dir: ".clinerules", RulesSubdir: ""` (semantic stretch) or add a `RulesDir` field for tools whose rules dir is at root.
- Add `continue` to `ToolRegistry` with `Dir: ".continue", RulesSubdir: "rules"`. Continue's `.continue/prompts/` is single-file prompts — same problem as Copilot's `.github/prompts/` (deferred).
- Add `augment` to `ToolRegistry` with `Dir: ".augment", RulesSubdir: "rules"`.

## Merged-config tools — MCP lives inside shared files

A separate class of gap: tools whose MCP config is a *section* of a larger file that also contains user-owned content (auth, model prefs, sandbox config, OAuth state). csaw's symlink projection can't handle these — symlinking the whole file would overwrite user settings; not symlinking leaves MCP unprojected.

| Tool | MCP file | Format | Scope options |
|---|---|---|---|
| Codex CLI | `[mcp_servers.*]` tables | TOML | `.codex/config.toml` (project) or `~/.codex/config.toml` (user) |
| OpenCode | `mcp` object | JSON/JSONC | `opencode.json` (project) or `~/.config/opencode/opencode.json` (user) |
| Copilot CLI | `mcpServers` object | JSON | `~/.copilot/mcp-config.json` (user only) |
| VS Code workspace (older) | `mcp.servers` keys | JSON | `.vscode/settings.json` (project) — note: the newer dedicated `.vscode/mcp.json` is already symlink-projected |

Three of the four have project-scope file options, so csaw's project-scope identity stays intact for them. Copilot CLI is the user-scope outlier.

**Status: Codex shipped v0.9.0; others pre-staged.** `csaw mcp sync codex --from <source>` merges via a bounded section (user content byte-for-byte preserved; rollback drops the section; drift detected via SHA). OpenCode / Copilot CLI / VS Code workspace use the same architecture documented in [`mcp-merge-design.md`](mcp-merge-design.md) and can be wired in ~1 day each when a real user reports friction on that specific tool.

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
