package mount

import "testing"

func TestFilterEntries(t *testing.T) {
	planner := NewPlanner()
	entries := []string{
		"agents/base.md",
		"agents/go.md",
		"skills/debugging/SKILL.md",
		"skills/experimental/SKILL.md",
	}

	selection := Selection{
		IncludePatterns: []string{"agents", "skills/**"},
		ExcludePatterns: []string{"skills/experimental/**"},
	}

	filtered, err := planner.Filter(entries, selection)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if got, want := len(filtered), 3; got != want {
		t.Fatalf("len(filtered) = %d, want %d (%v)", got, want, filtered)
	}
}

func TestFilterDefaultsToAllWhenNoIncludes(t *testing.T) {
	planner := NewPlanner()
	entries := []string{"AGENTS.md", "skills/debugging/SKILL.md"}

	filtered, err := planner.Filter(entries, Selection{})
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if got, want := len(filtered), len(entries); got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
}

func TestFilterByKindOnly(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "AGENTS.md"},
		{RelativePath: "agents/code-reviewer.md"},
		{RelativePath: "agents/planner.md"},
		{RelativePath: "skills/code-review/SKILL.md"},
		{RelativePath: "skills/testing/SKILL.md"},
		{RelativePath: "rules/go.md"},
		{RelativePath: "mcp/claude-code.json"},
	}

	got := FilterByKind(entries, []Kind{KindAgent})
	if len(got) != 2 {
		t.Fatalf("FilterByKind(KindAgent) returned %d entries, want 2: %+v", len(got), got)
	}
	for _, entry := range got {
		if KindOf(entry) != KindAgent {
			t.Errorf("expected only agents; got %s (kind=%s)", entry.RelativePath, KindOf(entry))
		}
	}
}

func TestFilterByKindMultiple(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "agents/foo.md"},
		{RelativePath: "skills/bar/SKILL.md"},
		{RelativePath: "rules/baz.md"},
		{RelativePath: "mcp/claude-code.json"},
	}

	got := FilterByKind(entries, []Kind{KindAgent, KindRule})
	if len(got) != 2 {
		t.Fatalf("FilterByKind(agent+rule) returned %d entries, want 2", len(got))
	}
}

func TestFilterByKindEmptyReturnsAll(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "agents/foo.md"},
		{RelativePath: "skills/bar/SKILL.md"},
	}

	got := FilterByKind(entries, nil)
	if len(got) != len(entries) {
		t.Fatalf("FilterByKind(nil) should return all; got %d, want %d", len(got), len(entries))
	}
}

func TestFilterSourceEntriesAppliesKindFilter(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "AGENTS.md", QualifiedPath: "src/AGENTS.md"},
		{RelativePath: "agents/foo.md", QualifiedPath: "src/agents/foo.md"},
		{RelativePath: "skills/bar/SKILL.md", QualifiedPath: "src/skills/bar/SKILL.md"},
	}

	filtered, err := FilterSourceEntries(entries, Selection{Kinds: []Kind{KindAgent}})
	if err != nil {
		t.Fatalf("FilterSourceEntries error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry after kind filter, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].RelativePath != "agents/foo.md" {
		t.Errorf("unexpected entry: %s", filtered[0].RelativePath)
	}
}

func TestIsExperimentalPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Exact segment match at any depth, any kind
		{"skills/experimental/foo/SKILL.md", true},
		{"rules/experimental/draft.md", true},
		{"agents/experimental/wip.md", true},
		{"hooks/experimental/pre-commit.sh", true},
		{"ignore/experimental/cursor", true},
		{"experimental/anything.md", true},
		{"deep/nested/experimental/file.md", true},

		// NOT a match — substring is not enough
		{"rules/experimental-features.md", false},
		{"skills/my-experimental-skill/SKILL.md", false},
		{"agents/experimentalist.md", false},

		// Normal paths
		{"AGENTS.md", false},
		{"rules/security.md", false},
		{"skills/code-review/SKILL.md", false},

		// Windows-style separators normalize
		{"skills\\experimental\\foo.md", true},
	}
	for _, tc := range cases {
		got := IsExperimentalPath(tc.path)
		if got != tc.want {
			t.Errorf("IsExperimentalPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterExperimentalRemovesAndCounts(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "AGENTS.md"},
		{RelativePath: "rules/security.md"},
		{RelativePath: "skills/experimental/wip/SKILL.md"},
		{RelativePath: "rules/experimental/draft.md"},
		{RelativePath: "agents/reviewer.md"},
	}
	kept, hidden := FilterExperimental(entries)
	if hidden != 2 {
		t.Errorf("hiddenCount = %d, want 2", hidden)
	}
	if len(kept) != 3 {
		t.Fatalf("len(kept) = %d, want 3", len(kept))
	}
	for _, e := range kept {
		if IsExperimentalPath(e.RelativePath) {
			t.Errorf("kept entry %q is experimental — should have been filtered", e.RelativePath)
		}
	}
}
