package pinning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetAndGet(t *testing.T) {
	state := PinState{}
	state = Set(state, "team", "feature/new-rules")

	ref, ok := Get(state, "team")
	if !ok || ref != "feature/new-rules" {
		t.Fatalf("Get(team) = %q, %v; want feature/new-rules, true", ref, ok)
	}

	_, ok = Get(state, "other")
	if ok {
		t.Fatal("Get(other) should return false")
	}
}

func TestSetOverwrites(t *testing.T) {
	state := PinState{}
	state = Set(state, "team", "v1")
	state = Set(state, "team", "v2")

	ref, _ := Get(state, "team")
	if ref != "v2" {
		t.Fatalf("Get(team) = %q, want v2", ref)
	}
	if len(state.Pins) != 1 {
		t.Fatalf("len(Pins) = %d, want 1", len(state.Pins))
	}
}

func TestRemove(t *testing.T) {
	state := PinState{}
	state = Set(state, "team", "v1")
	state = Set(state, "personal", "main")
	state = Remove(state, "team")

	_, ok := Get(state, "team")
	if ok {
		t.Fatal("team should be removed")
	}

	ref, ok := Get(state, "personal")
	if !ok || ref != "main" {
		t.Fatalf("personal should still exist: %q, %v", ref, ok)
	}
}

func TestReadWriteRoundtrip(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".csaw-stash"), 0o755); err != nil {
		t.Fatal(err)
	}

	state := PinState{}
	state = Set(state, "team", "feature/branch")
	if err := Write(project, state); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	loaded, err := Read(project)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	ref, ok := Get(loaded, "team")
	if !ok || ref != "feature/branch" {
		t.Fatalf("roundtrip failed: %q, %v", ref, ok)
	}
}

func TestWriteEmptyRemovesFile(t *testing.T) {
	project := t.TempDir()
	stashDir := filepath.Join(project, ".csaw-stash")
	if err := os.MkdirAll(stashDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write then clear
	state := Set(PinState{}, "team", "v1")
	if err := Write(project, state); err != nil {
		t.Fatal(err)
	}

	state = Remove(state, "team")
	if err := Write(project, state); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(PinStatePath(project)); !os.IsNotExist(err) {
		t.Fatal("pins.json should be deleted when empty")
	}
}

func TestReadNonexistent(t *testing.T) {
	state, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(state.Pins) != 0 {
		t.Fatalf("Pins = %v, want empty", state.Pins)
	}
}
