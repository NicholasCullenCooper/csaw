package registry

// Preset describes a curated source scaffold. Each preset is a fixed set of
// files written into a new registry directory. Presets are opt-in via
// `csaw init --preset <name>`; the default `csaw init` (no flag) writes the
// historical solo-style starter set defined in init.go.
//
// Per docs/planning/package-manager-lessons.md, csaw follows the uv/cargo
// "built-in templates only" model. There's no community-template marketplace
// — adding one requires a trust model (signing, version gates) that isn't
// worth the cost at csaw's stage. Add new presets here as needed; PRs welcome.
type Preset struct {
	Name        string
	Description string
	// Files maps registry-relative path → file content. Directories are
	// created implicitly from the paths.
	Files map[string]string
}

var builtinPresets = []Preset{
	soloEngineerPreset(),
	teamGoPreset(),
	teamFrontendPreset(),
}

// GetPreset looks up a preset by name. Returns the preset and true if found,
// zero value and false if the name doesn't match any built-in.
func GetPreset(name string) (Preset, bool) {
	for _, p := range builtinPresets {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

// ListPresets returns all built-in presets in display order.
func ListPresets() []Preset {
	return builtinPresets
}

// --- preset definitions ---

func soloEngineerPreset() Preset {
	return Preset{
		Name:        "solo-engineer",
		Description: "Personal source: instructions, one starter rule, one example skill. Good fit for individual developers composing their own AI workspace.",
		Files: map[string]string{
			"csaw.yml":    starterProfile,
			".csawignore": starterIgnore,
			"AGENTS.md":   presetSoloAgents,
			"rules/personal.md": `# Personal Preferences

- Prefer terse explanations over thorough ones; assume I know the basics.
- When asked for help, suggest the smallest change that solves the problem.
- Cite line numbers when referencing existing code.
`,
			"skills/debug-strategy/SKILL.md": `---
name: debug-strategy
description: Systematic approach for narrowing down bugs by hypothesis
---

# Debug Strategy

When debugging a non-trivial bug:

1. **Reproduce.** Write the smallest input that triggers the bug. Don't proceed without a repro.
2. **Form a hypothesis.** State what you think is happening in one sentence.
3. **Find the strongest disconfirming evidence you can run cheaply.** Don't confirm — refute.
4. **If the hypothesis survives, fix at the lowest layer where the bug originates** (not the topmost symptom).
5. **Add a regression test before declaring done.**

Resist the urge to "try a few things." Each change without a hypothesis adds noise.
`,
		},
	}
}

func teamGoPreset() Preset {
	return Preset{
		Name:        "team-go",
		Description: "Team source for Go-shop teams: protected AGENTS.md and security rule, Go conventions, code-review agent. Mark this source priority 50 in the canonical 4-tier stack.",
		Files: map[string]string{
			"csaw.yml":    presetTeamGoProfile,
			".csawignore": starterIgnore,
			"AGENTS.md":   presetTeamGoAgents,
			"rules/go-conventions.md": `# Go Conventions

- Prefer the standard library before adding a dependency.
- Use ` + "`context.Context`" + ` as the first parameter for any function that does I/O or could block.
- Wrap errors with ` + "`fmt.Errorf(\"...: %w\", err)`" + ` so they unwrap cleanly.
- Table-driven tests for anything with more than one input shape.
- Use ` + "`gofmt`" + ` defaults — no opinions to litigate.
- Avoid ` + "`init()`" + ` functions unless absolutely necessary.
`,
			"rules/security.md": `# Security (Protected)

- Never log credentials, tokens, or session IDs at any level.
- All external input is untrusted: validate at the system boundary, not deep in business logic.
- Database queries use parameterized statements only — never string concatenation.
- New dependencies require a one-line justification in the PR description.
- Cryptography uses the standard library or x/crypto only.
`,
			"rules/testing.md": `# Testing Standards

- Every exported function has at least one test.
- Integration tests hit real databases/services in CI, not mocks (mocks lie when contracts drift).
- Test names describe the scenario, not the function: ` + "`TestUserCreate_WhenEmailExists_ReturnsConflict`" + `, not ` + "`TestCreate`" + `.
- ` + "`t.Helper()`" + ` on every test helper. ` + "`t.Cleanup()`" + ` over ` + "`defer`" + ` in tests.
`,
			"agents/code-reviewer.md": `---
name: code-reviewer
description: Reviews Go code for correctness, idiom, and adherence to team conventions
---

# Code Reviewer (Go)

You are a senior Go reviewer. For each diff:

1. **Correctness first.** Look for off-by-one, nil dereferences, race conditions, error swallowing.
2. **Idiom second.** Flag non-idiomatic patterns (interface pollution, unnecessary pointers, ` + "`init()`" + ` abuse).
3. **Convention third.** Cross-check against ` + "`rules/go-conventions.md`" + ` and ` + "`rules/security.md`" + `.
4. **Be specific.** Reference line numbers. Suggest the minimal fix, not a rewrite.
5. **Skip nits the formatter handles.** Don't comment on what ` + "`gofmt`" + ` will fix.
`,
			"skills/commit-message/SKILL.md": starterSkillCommitMsg,
		},
	}
}

func teamFrontendPreset() Preset {
	return Preset{
		Name:        "team-frontend",
		Description: "Team source for TypeScript/React teams: protected AGENTS.md and accessibility rule, TS/React conventions, frontend-focused code-review agent.",
		Files: map[string]string{
			"csaw.yml":    presetTeamFrontendProfile,
			".csawignore": starterIgnore,
			"AGENTS.md":   presetTeamFrontendAgents,
			"rules/typescript-style.md": `# TypeScript Style

- ` + "`strict`" + ` mode on. No ` + "`any`" + `; use ` + "`unknown`" + ` and narrow.
- Prefer ` + "`type`" + ` over ` + "`interface`" + ` for object shapes (consistency, easier composition).
- No default exports for components — named exports only (better grep, better refactors).
- Use ` + "`satisfies`" + ` when you need a literal to conform to a type without widening.
- Async functions return ` + "`Promise<T>`" + ` explicitly when public; let inference handle internals.
`,
			"rules/react-patterns.md": `# React Patterns

- Components are functions, not classes.
- Co-locate state with the component that uses it; lift only when shared.
- ` + "`useEffect`" + ` is for synchronizing with external systems, not for derived data. Use ` + "`useMemo`" + ` or compute inline.
- Custom hooks for reusable stateful logic — single-responsibility, clear input/output.
- Forms: validate at the boundary (Zod or similar), not field-by-field with side effects.
`,
			"rules/accessibility.md": `# Accessibility (Protected)

- Every interactive element is keyboard-reachable and has a visible focus indicator.
- Images that convey meaning have ` + "`alt`" + ` text; decorative images use ` + "`alt=\"\"`" + `.
- Form inputs have associated ` + "`<label>`" + ` elements (not placeholder-as-label).
- Color is never the only way to convey state (errors, required fields, etc.).
- Test with keyboard-only navigation before declaring a component done.
`,
			"agents/code-reviewer.md": `---
name: code-reviewer
description: Reviews TypeScript/React code for correctness, performance, accessibility, and team conventions
---

# Code Reviewer (Frontend)

You are a senior frontend reviewer. For each diff:

1. **Accessibility check.** Run through ` + "`rules/accessibility.md`" + ` for any new interactive element.
2. **Correctness.** Watch for stale closures, missing keys, mutation of props/state, race conditions in effects.
3. **Performance.** Flag unnecessary re-renders, missing ` + "`useMemo`" + `/` + "`useCallback`" + ` only where measurable, bundle-size impact for new deps.
4. **Convention.** Cross-check ` + "`rules/typescript-style.md`" + ` and ` + "`rules/react-patterns.md`" + `.
5. **Specific feedback.** Line numbers, minimal fix, no rewrites.
`,
			"skills/commit-message/SKILL.md": starterSkillCommitMsg,
		},
	}
}

// --- shared preset content (different from default starters) ---

var presetSoloAgents = `# Personal Coding Workspace

This is a personal AI workspace source. Files here apply to whatever project I'm working on, layered on top of any team or company sources.

## Style
- Terse over thorough. Assume I know the basics.
- Smallest change that solves the problem.
- Cite line numbers when referencing code.

## Workflow
- Tests before claiming done.
- Don't ask permission to edit files I've opened.
- If unsure between two approaches, pick the simpler one and note the alternative.
`

var presetTeamGoProfile = `# csaw.yml — team profile with protected files
#
# The csaw: block below marks files as protected. Lower-priority sources
# (e.g., personal) cannot silently override these files. Audit catches drift.

csaw:
  protected:
    - AGENTS.md
    - rules/security.md

default:
  description: Team Go conventions + security + code-review
  include:
    - AGENTS.md
    - rules/**
    - skills/**
    - agents/**
`

var presetTeamGoAgents = `# Team Standards (Go)

This source publishes team-wide standards for Go development. ` + "`AGENTS.md`" + ` and ` + "`rules/security.md`" + ` are protected — lower-priority sources can't override them.

## Stack assumptions
- Go 1.22+
- Standard library first; external deps justified in PR descriptions.

## Workflow
- Run ` + "`gofmt`" + `, ` + "`go vet`" + `, and tests before opening a PR.
- Conventional commits (see skills/commit-message).
- Security review for any change touching auth, crypto, or external input handling.

## Code review
The ` + "`code-reviewer`" + ` agent enforces the rules in ` + "`rules/`" + `. Invoke it on any non-trivial diff.
`

var presetTeamFrontendProfile = `# csaw.yml — team profile with protected files

csaw:
  protected:
    - AGENTS.md
    - rules/accessibility.md

default:
  description: Team TypeScript/React conventions + a11y + code-review
  include:
    - AGENTS.md
    - rules/**
    - skills/**
    - agents/**
`

var presetTeamFrontendAgents = `# Team Standards (Frontend)

This source publishes team-wide standards for TypeScript and React development. ` + "`AGENTS.md`" + ` and ` + "`rules/accessibility.md`" + ` are protected — accessibility is non-negotiable.

## Stack assumptions
- TypeScript 5.x (strict), React 18+
- Bundler: whatever the project uses; agents read the project's config.

## Workflow
- Lint + typecheck + tests before opening a PR.
- Accessibility check on every component touching interactive UI.
- Conventional commits (see skills/commit-message).

## Code review
The ` + "`code-reviewer`" + ` agent enforces the rules in ` + "`rules/`" + ` — accessibility first.
`
