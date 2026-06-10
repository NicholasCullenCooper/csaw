package vendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndLoadDeclarations(t *testing.T) {
	tmp := t.TempDir()

	// Start with an existing csaw.yml that has profiles but no vendors.
	if err := os.WriteFile(filepath.Join(tmp, "csaw.yml"), []byte(`default:
  description: my profile
  include:
    - rules/**
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a vendor.
	if err := AddDeclaration(tmp, Declaration{
		Name:    "awesome-copilot",
		URL:     "https://github.com/github/awesome-copilot",
		Ref:     "main",
		Include: []string{"agents/**"},
	}); err != nil {
		t.Fatalf("AddDeclaration: %v", err)
	}

	// Reload and verify.
	decls, err := LoadDeclarations(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 1 {
		t.Fatalf("got %d declarations, want 1", len(decls))
	}
	d := decls[0]
	if d.Name != "awesome-copilot" || d.URL != "https://github.com/github/awesome-copilot" {
		t.Errorf("unexpected declaration: %+v", d)
	}
	if d.Ref != "main" {
		t.Errorf("Ref = %q, want main", d.Ref)
	}

	// The original profile must still be present in csaw.yml.
	data, _ := os.ReadFile(filepath.Join(tmp, "csaw.yml"))
	if !strings.Contains(string(data), "description: my profile") {
		t.Errorf("AddDeclaration clobbered existing profiles; csaw.yml:\n%s", data)
	}
}

func TestLoadDeclarationsExpandsShorthand(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "csaw.yml"), []byte(`vendors:
  acme:
    url: "gh:acme/registry#v1.0.0"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	decls, err := LoadDeclarations(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 1 {
		t.Fatalf("got %d declarations", len(decls))
	}
	if decls[0].URL != "https://github.com/acme/registry.git" {
		t.Errorf("URL = %q (shorthand not expanded)", decls[0].URL)
	}
	if decls[0].Ref != "v1.0.0" {
		t.Errorf("Ref = %q (#ref not extracted from shorthand)", decls[0].Ref)
	}
}

func TestAddDeclarationRefusesDuplicate(t *testing.T) {
	tmp := t.TempDir()
	d := Declaration{Name: "x", URL: "https://example.com/x.git"}
	if err := AddDeclaration(tmp, d); err != nil {
		t.Fatal(err)
	}
	err := AddDeclaration(tmp, d)
	if err == nil {
		t.Fatal("expected duplicate-add error, got nil")
	}
	if !strings.Contains(err.Error(), "already declared") {
		t.Errorf("error = %q; should mention 'already declared'", err.Error())
	}
}

func TestRemoveDeclaration(t *testing.T) {
	tmp := t.TempDir()
	if err := AddDeclaration(tmp, Declaration{Name: "a", URL: "https://a.example/x.git"}); err != nil {
		t.Fatal(err)
	}
	if err := AddDeclaration(tmp, Declaration{Name: "b", URL: "https://b.example/x.git"}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveDeclaration(tmp, "a"); err != nil {
		t.Fatal(err)
	}

	decls, _ := LoadDeclarations(tmp)
	if len(decls) != 1 || decls[0].Name != "b" {
		t.Errorf("after removing a, got: %+v", decls)
	}

	// Removing the second one should leave the vendors block absent.
	if err := RemoveDeclaration(tmp, "b"); err != nil {
		t.Fatal(err)
	}
	decls, _ = LoadDeclarations(tmp)
	if len(decls) != 0 {
		t.Errorf("after removing both, got: %+v", decls)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, "csaw.yml"))
	if strings.Contains(string(data), "vendors:") {
		t.Errorf("csaw.yml still has vendors block after removing all entries:\n%s", data)
	}
}

func TestRemoveDeclarationUnknownErrors(t *testing.T) {
	tmp := t.TempDir()
	if err := AddDeclaration(tmp, Declaration{Name: "x", URL: "https://x.git"}); err != nil {
		t.Fatal(err)
	}
	err := RemoveDeclaration(tmp, "nope")
	if err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Errorf("expected 'not declared' error, got: %v", err)
	}
}
