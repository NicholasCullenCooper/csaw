package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

func TestCatalogResolverCrossSourceExtends(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	acmeRoot := filepath.Join(root, "acme")
	if err := os.MkdirAll(acmeRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(acmeRoot, runtime.ProfilesFile), []byte(`
backend:
  include:
    - agents/go.md
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.Root, runtime.ProfilesFile), []byte(`
default:
  extends: acme/backend
  include:
    - personal/skills/debug/**
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolver, err := NewCatalogResolver(paths, []sources.CatalogSource{{Name: "acme", Root: acmeRoot}})
	if err != nil {
		t.Fatalf("NewCatalogResolver() error = %v", err)
	}

	profile, err := resolver.Resolve("default")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := len(profile.Include), 2; got != want {
		t.Fatalf("len(Include) = %d, want %d (%v)", got, want, profile.Include)
	}
	if profile.Include[0] != "acme/agents/go.md" {
		t.Fatalf("first include = %q, want acme-qualified include", profile.Include[0])
	}
}
