package lock

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func TestFromStateHashesSymlinkEntriesAndRelativizesPaths(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "team")
	os.MkdirAll(filepath.Join(sourceRoot, "rules"), 0o755)
	rule := filepath.Join(sourceRoot, "rules", "go.md")
	os.WriteFile(rule, []byte("v1\n"), 0o644)
	wantHash, _ := workspace.FileSHA256(rule)

	state := workspace.MountState{Entries: []workspace.MountedStateEntry{
		{RelativePath: ".claude/rules/go.md", SourceName: "team", SourcePath: rule},
		{RelativePath: "CLAUDE.md", SourceName: "team", SourcePath: filepath.Join(sourceRoot, "AGENTS.md"), Generated: true, ContentSHA256: "abc"},
		{RelativePath: "AGENTS.md", SourceName: "team", SourcePath: filepath.Join(sourceRoot, "AGENTS.md"), Mode: "copy", ContentSHA256: "def"},
	}}
	infos := map[string]SourceInfo{"team": {Root: sourceRoot, Kind: "remote", URL: "git@example.com:org/team.git", RefResolved: "0123456789abcdef"}}

	file, err := FromState(state, infos)
	if err != nil {
		t.Fatalf("FromState() error = %v", err)
	}
	if len(file.Sources) != 1 || file.Sources[0].RefResolved != "0123456789abcdef" {
		t.Fatalf("sources = %#v", file.Sources)
	}
	byPath := map[string]EntryLock{}
	for _, e := range file.Entries {
		byPath[e.Path] = e
	}
	if got := byPath[".claude/rules/go.md"]; got.Mode != "symlink" || got.ContentSHA256 != wantHash || got.SourcePath != "rules/go.md" {
		t.Fatalf("symlink entry = %#v", got)
	}
	if got := byPath["CLAUDE.md"]; got.Mode != "generated" || got.ContentSHA256 != "abc" {
		t.Fatalf("generated entry = %#v", got)
	}
	if got := byPath["AGENTS.md"]; got.Mode != "copy" || got.ContentSHA256 != "def" || got.SourcePath != "AGENTS.md" {
		t.Fatalf("copy entry = %#v", got)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	project := t.TempDir()
	want := File{
		Sources: []SourceLock{{Name: "team", Kind: "remote", URL: "u", RefRequested: "main", RefResolved: "sha"}},
		Entries: []EntryLock{{Path: "AGENTS.md", Source: "team", SourcePath: "AGENTS.md", Mode: "copy", ContentSHA256: "h"}},
	}
	if err := Write(project, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got, found, err := Read(project)
	if err != nil || !found {
		t.Fatalf("Read() = found %v, err %v", found, err)
	}
	if got.Version != Version || got.GeneratedAt == "" {
		t.Fatalf("Read() = %#v", got)
	}
	got.GeneratedAt = ""
	want.Version = Version
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", got, want)
	}

	if _, found, err := Read(t.TempDir()); err != nil || found {
		t.Fatalf("Read() on empty project = found %v, err %v", found, err)
	}
}

func TestDiffReportsEveryDifference(t *testing.T) {
	locked := File{
		Sources: []SourceLock{{Name: "team", URL: "u", RefRequested: "main", RefResolved: "aaaaaaaaaaaaaaaa"}, {Name: "gone"}},
		Entries: []EntryLock{
			{Path: "AGENTS.md", Source: "team", Mode: "copy", ContentSHA256: "h1"},
			{Path: "removed.md", Source: "team", Mode: "copy", ContentSHA256: "h3"},
		},
	}
	current := File{
		Sources: []SourceLock{{Name: "team", URL: "u", RefRequested: "main", RefResolved: "bbbbbbbbbbbbbbbb"}, {Name: "extra"}},
		Entries: []EntryLock{
			{Path: "AGENTS.md", Source: "team", Mode: "symlink", ContentSHA256: "h2"},
			{Path: "added.md", Source: "team", Mode: "copy", ContentSHA256: "h4"},
		},
	}
	diffs := Diff(locked, current)
	joined := strings.Join(diffs, "\n")
	for _, want := range []string{
		"source extra: mounted but not locked",
		"source gone: locked but not mounted",
		"source team: commit bbbbbbbbbbbb, locked aaaaaaaaaaaa",
		"AGENTS.md: mode symlink, locked copy",
		"AGENTS.md: content h2, locked h1",
		"added.md: mounted but not locked",
		"removed.md: locked but not mounted",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Diff() missing %q in:\n%s", want, joined)
		}
	}
	if got := Diff(locked, locked); len(got) != 0 {
		t.Fatalf("identical files should not differ: %v", got)
	}
}
