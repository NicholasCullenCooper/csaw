package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMultiParentProfile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "csaw.yml")
	content := `
base:
  include:
    - agents/base.md
security:
  include:
    - agents/security.md
backend:
  extends:
    - base
    - security
  include:
    - agents/go.md
  exclude:
    - skills/experimental/**
`

	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolver, err := NewFileResolver(file)
	if err != nil {
		t.Fatalf("NewFileResolver() error = %v", err)
	}

	profile, err := resolver.Resolve("backend")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := len(profile.Include), 3; got != want {
		t.Fatalf("len(Include) = %d, want %d (%v)", got, want, profile.Include)
	}
	if got, want := len(profile.Exclude), 1; got != want {
		t.Fatalf("len(Exclude) = %d, want %d", got, want)
	}
}

func TestResolveCycleFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "csaw.yml")
	content := `
a:
  extends: b
  include: [a]
b:
  extends: a
  include: [b]
`

	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolver, err := NewFileResolver(file)
	if err != nil {
		t.Fatalf("NewFileResolver() error = %v", err)
	}

	if _, err := resolver.Resolve("a"); err == nil {
		t.Fatal("Resolve() error = nil, want cycle error")
	}
}
