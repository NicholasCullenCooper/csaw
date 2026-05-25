# Tool Projection Roadmap

Planning doc for what csaw projects (and doesn't) across AI coding tools. Living document — update as decisions land.

Source of truth for *what csaw actually projects today* is [`docs/reference/tool-projection.json`](../reference/tool-projection.json) (tools with `csaw_in_code: true`). This file is for what's *coming* or explicitly *out of scope*.

## Currently in code (v0.6.0)

See [`tool-projection.json`](../reference/tool-projection.json) tools with `"csaw_in_code": true`. The projection test (`internal/mount/projection_consistency_test.go`) enforces that the JSON's claims match the actual Go `ToolRegistry`.

## Needs structural work before adding

- **vscode-copilot** — Per-subdir filename suffix (`.agent.md`, `.instructions.md`, `.prompt.md`); single-file instruction target (`.github/copilot-instructions.md`); per-tool `CommitToGit` flag for `.github/` paths.
- **copilot-cli** — Same as vscode-copilot. Should be unified `copilot` entry when the work lands.
- **cody** — Project `.vscode/cody.json` content schema is Cody-specific (custom commands JSON).
- **hermes** — csaw already serves via `.cursor/rules/` + AGENTS.md; could add explicit `.hermes.md`/`HERMES.md` instruction target if direct support becomes desirable.
- **pi** — Only project paths are AGENTS.md/CLAUDE.md (already served). MCP is inside `~/.pi/agent/` — out of scope.

## Out of scope with current model

- **zed** — settings.json user-scope only. Project `.rules` is single file.
- **devin** — Cloud-first. Playbooks in Devin UI.
- **plandex** — No dotfile-dir convention documented.
- **factory** — AGENTS.md only — already served by csaw's instructions kind.
- **qodo** — Code review tool; hierarchical `best_practices.md` is a different model.
- **lingma** — Mostly IDE-internal; docs primarily Chinese.

## Explicitly never (privacy / out of scope by design)

- **Settings** as a csaw kind — settings files contain API keys and personal credentials. Mounting them to a team source would leak secrets. Document tool settings paths as reference only.

## Future consideration

- **Memory** as a csaw kind — most "memory" today is session state or user-private. csaw is project-scope. Revisit if a portable memory standard emerges.
- **goose recipes** at `.goose/recipes/` — recipes are YAML files, not csaw's SKILL.md folder pattern. Recipes can be a future kind if there's demand, but the .goosehints file (now recognized as `instructions`) covers the common Goose use case.

## Deprecated

- **gemini** — Google sunset 2026-06-18. Migration target: `antigravity`. Remove `gemini` from `ToolRegistry` post-sunset (~v0.7.0).

## Watch list

- **claw-code** — Reuses `.claude/` paths; auto-served by csaw's Claude projection.
- **joycode** — ECC adapter; not a standalone tool.
- **openclaw** — Reference architecture; ecosystem of derivatives.
