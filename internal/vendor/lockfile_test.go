package vendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestLoadLockfileMissingReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	lf, err := LoadLockfile(tmp)
	if err != nil {
		t.Fatalf("LoadLockfile on empty dir: %v", err)
	}
	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if lf.Vendors == nil {
		t.Error("Vendors map should be initialized, not nil")
	}
}

func TestSaveAndReloadLockfile(t *testing.T) {
	tmp := t.TempDir()
	lf := &Lockfile{
		Version: 1,
		Vendors: map[string]VendorState{
			"acme": {
				URL: "https://example.com/acme.git", RefRequested: "main",
				RefResolved: "abc123def456",
				SyncedAt:    time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC),
				Files: map[string]FileRecord{
					"agents/x.md": {SHA256: "deadbeef", Size: 123},
				},
			},
		},
		Promotions: []Promotion{
			{Vendor: "acme", VendorPath: "agents/x.md", PromotedTo: "agents/x.md",
				PromotedAt:            time.Date(2026, 6, 9, 12, 1, 0, 0, time.UTC),
				VendorSHA256AtPromote: "deadbeef"},
		},
	}
	if err := SaveLockfile(tmp, lf); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadLockfile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := reloaded.Vendors["acme"].RefResolved, "abc123def456"; got != want {
		t.Errorf("RefResolved roundtrip: got %q, want %q", got, want)
	}
	if got := reloaded.Vendors["acme"].Files["agents/x.md"].SHA256; got != "deadbeef" {
		t.Errorf("file SHA roundtrip: got %q", got)
	}
	if len(reloaded.Promotions) != 1 {
		t.Fatalf("promotions roundtrip: got %d, want 1", len(reloaded.Promotions))
	}
}

func TestSaveLockfileWritesHumanHeader(t *testing.T) {
	tmp := t.TempDir()
	lf := &Lockfile{Version: 1, Vendors: map[string]VendorState{}}
	if err := SaveLockfile(tmp, lf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	str := string(data)
	if !strings.HasPrefix(str, "# csaw vendor lockfile") {
		t.Errorf("missing header; got start: %q", str[:minInt(40, len(str))])
	}
}
