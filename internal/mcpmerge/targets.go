// Package mcpmerge projects MCP server entries from csaw source registries
// into shared-config files that csaw does not fully own — e.g., Codex CLI's
// .codex/config.toml, where the user owns model preferences, providers, and
// other configuration but wants team MCP entries managed via csaw.
//
// Unlike the symlink-based MCP projection in internal/mount, this package
// writes a clearly-marked bounded section at the end of the target file.
// User content outside that section is preserved byte-for-byte. Rollback
// (csaw mcp sync <tool> --remove) deletes only the bounded section.
//
// Per docs/planning/mcp-merge-design.md, this approach was chosen after a
// spike confirmed full parse-modify-serialize round-trip via go-toml/v2
// drops comments and reorders keys. Text-level bounded sections sidestep
// every round-trip hazard.
package mcpmerge

// Format identifies the syntax of a merge target file. The MVP supports
// TOML (Codex); JSON/JSONC support is pre-staged in the design doc but
// not yet implemented — add when a real user reports friction.
type Format int

const (
	FormatTOML  Format = iota
	FormatJSON         // not yet wired
	FormatJSONC        // not yet wired
)

func (f Format) String() string {
	switch f {
	case FormatTOML:
		return "toml"
	case FormatJSON:
		return "json"
	case FormatJSONC:
		return "jsonc"
	}
	return "unknown"
}

// MergeTarget describes a tool whose MCP config lives inside a shared file
// csaw cannot fully own. csaw projects MCP entries into a bounded section
// at the end of the file.
type MergeTarget struct {
	// Name is the CLI identifier: `csaw mcp sync <Name>`.
	Name string
	// DisplayName is the human-readable label.
	DisplayName string
	// ProjectPath is the path csaw writes, relative to the project root.
	ProjectPath string
	// FragmentName is the filename csaw expects in a source registry's
	// mcp/ directory (e.g., "codex.toml" → looked for at <source>/mcp/codex.toml).
	FragmentName string
	// Format determines parser selection for conflict detection.
	Format Format
	// StartMarker is the literal line beginning the managed section.
	// Must be unique in well-formed target files.
	StartMarker string
	// EndMarker is the literal line ending the managed section.
	EndMarker string
}

// KnownMergeTargets is the registry of supported merge targets.
//
// The MVP supports only Codex. Per docs/planning/next-up.md, additional
// targets (OpenCode, Copilot CLI, VS Code settings) are added when a real
// user reports friction — not pre-emptively for parity.
var KnownMergeTargets = []MergeTarget{
	{
		Name:         "codex",
		DisplayName:  "OpenAI Codex CLI",
		ProjectPath:  ".codex/config.toml",
		FragmentName: "codex.toml",
		Format:       FormatTOML,
		StartMarker:  "# === csaw managed start (do not edit; use: csaw mcp sync codex --remove) ===",
		EndMarker:    "# === csaw managed end ===",
	},
}

// GetTarget looks up a merge target by name. Returns false if unknown.
func GetTarget(name string) (MergeTarget, bool) {
	for _, t := range KnownMergeTargets {
		if t.Name == name {
			return t, true
		}
	}
	return MergeTarget{}, false
}

// ListTargetNames returns supported target names in registration order.
// Used by the CLI to render help text and validate user input.
func ListTargetNames() []string {
	names := make([]string, len(KnownMergeTargets))
	for i, t := range KnownMergeTargets {
		names[i] = t.Name
	}
	return names
}
