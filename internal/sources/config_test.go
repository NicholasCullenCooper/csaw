package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

func TestNewSource(t *testing.T) {
	local, err := NewSource("personal", ".")
	if err != nil {
		t.Fatalf("NewSource local returned error: %v", err)
	}
	if local.Kind != KindLocal {
		t.Fatalf("local kind = %q, want %q", local.Kind, KindLocal)
	}

	remote, err := NewSource("acme", "git@github.com:acme/registry.git")
	if err != nil {
		t.Fatalf("NewSource remote returned error: %v", err)
	}
	if remote.Kind != KindRemote {
		t.Fatalf("remote kind = %q, want %q", remote.Kind, KindRemote)
	}
}

func TestCheckoutPath(t *testing.T) {
	paths := runtime.BuildPaths(filepath.Join(string(filepath.Separator), "tmp", "csaw"))
	source := Source{Name: "team", Kind: KindRemote}
	if got, want := source.CheckoutPath(paths), filepath.Join(paths.Sources, "team"); got != want {
		t.Fatalf("CheckoutPath() = %q, want %q", got, want)
	}
}

func TestExpandUserHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	got, err := expandUserHome("~/registry")
	if err != nil {
		t.Fatalf("expandUserHome() error = %v", err)
	}

	want := filepath.Join(home, "registry")
	if got != want {
		t.Fatalf("expandUserHome() = %q, want %q", got, want)
	}
}
