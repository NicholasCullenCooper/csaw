package vendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NicholasCullenCooper/csaw/internal/scan"
)

func TestPromoteRefusesFlaggedFilesWithoutForce(t *testing.T) {
	registry := t.TempDir()
	vendorDir := filepath.Join(registry, "vendor", "up")
	os.MkdirAll(filepath.Join(vendorDir, "skills", "x"), 0o755)
	content := []byte("---\nname: x\nallowed-tools: Bash\n---\n")
	os.WriteFile(filepath.Join(vendorDir, "skills", "x", "SKILL.md"), content, 0o644)
	sum, _, err := copyAndHash(filepath.Join(vendorDir, "skills", "x", "SKILL.md"), filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatal(err)
	}
	lf := &Lockfile{Version: 1, Vendors: map[string]VendorState{"up": {
		URL: "u", RefResolved: "abc", SyncedAt: time.Now(),
		Files:    map[string]FileRecord{"skills/x/SKILL.md": {SHA256: sum, Size: int64(len(content))}},
		Findings: []scan.Finding{{Path: "skills/x/SKILL.md", Check: scan.CheckToolGrant, Detail: "grants tools"}},
	}}}
	if err := SaveLockfile(registry, lf); err != nil {
		t.Fatal(err)
	}

	_, err = Promote(registry, "up", "skills/x/SKILL.md", "skills/x/SKILL.md", false)
	if err == nil || !strings.Contains(err.Error(), scan.CheckToolGrant) {
		t.Fatalf("expected a refusal naming the finding, got %v", err)
	}
	if _, err := Promote(registry, "up", "skills/x/SKILL.md", "skills/x/SKILL.md", true); err != nil {
		t.Fatalf("--force should promote: %v", err)
	}

	// A failed scanner verdict blocks every file of the vendor.
	state := lf.Vendors["up"]
	state.Findings = nil
	state.ScannerStatus = "failed (exit 2)"
	lf.Vendors["up"] = state
	SaveLockfile(registry, lf)
	if _, err := Promote(registry, "up", "skills/x/SKILL.md", "skills/y/SKILL.md", false); err == nil || !strings.Contains(err.Error(), "scanner") {
		t.Fatalf("expected a scanner refusal, got %v", err)
	}
}
