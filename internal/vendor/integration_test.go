package vendor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/git"
)

// runGit is a small test helper that runs git commands in a directory.
// Used to seed the upstream repo for integration tests.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestSyncPromoteAuditEndToEnd exercises the full vendor lifecycle against
// a local "upstream" git repo (no network). Validates that:
//   - sync clones, copies, hashes, writes lockfile + meta file
//   - vendored content lands at the right paths
//   - promote copies + records lineage with the correct SHA anchor
//   - audit detects no drift in a fresh state
//   - audit detects vendor-local drift when a vendored file is edited
//   - audit detects promotion drift when a promoted file is edited
//   - audit detects upstream drift when upstream gets a new commit
func TestSyncPromoteAuditEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping integration test")
	}

	root := t.TempDir()
	upstream := filepath.Join(root, "upstream")
	registry := filepath.Join(root, "registry")
	cache := filepath.Join(root, "cache")
	for _, d := range []string{upstream, registry} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// --- Seed upstream repo with two files ---
	runGit(t, upstream, "init", "-q")
	runGit(t, upstream, "config", "user.email", "test@test")
	runGit(t, upstream, "config", "user.name", "test")
	if err := os.MkdirAll(filepath.Join(upstream, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstream, "agents", "reviewer.md"), []byte("# Reviewer agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstream, "README.md"), []byte("# upstream readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", "-A")
	runGit(t, upstream, "commit", "-q", "-m", "seed")

	// --- Declare the vendor in the registry's csaw.yml ---
	decl := Declaration{
		Name:    "awesome",
		URL:     upstream,
		Include: []string{"agents/**"}, // exclude README
	}
	if err := AddDeclaration(registry, decl); err != nil {
		t.Fatal(err)
	}

	// Reload through the manifest path to make sure parsing works end-to-end.
	decls, err := LoadDeclarations(registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 1 {
		t.Fatalf("expected 1 declaration after add, got %d", len(decls))
	}

	// --- Sync ---
	res, err := Sync(context.Background(), git.ExecGit{}, decls[0], registry, cache, false)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.FilesAdded != 1 {
		t.Errorf("FilesAdded = %d, want 1 (README excluded by include filter)", res.FilesAdded)
	}

	// Vendored file should exist; excluded README should not.
	if _, err := os.Stat(filepath.Join(registry, "vendor", "awesome", "agents", "reviewer.md")); err != nil {
		t.Errorf("vendored file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(registry, "vendor", "awesome", "README.md")); err == nil {
		t.Error("README.md should have been excluded by include filter but was vendored")
	}
	// Meta file should exist.
	if _, err := os.Stat(filepath.Join(registry, "vendor", "awesome", vendorMetaFileName)); err != nil {
		t.Errorf("vendor meta file missing: %v", err)
	}

	// Audit (skip network) — should be clean immediately after sync.
	findings, err := Audit(context.Background(), nil, registry, cache)
	if err != nil {
		t.Fatal(err)
	}
	if findings.HasAny() {
		t.Errorf("clean audit expected; got: %+v", findings)
	}

	// --- Promote one file ---
	promoteRes, err := Promote(registry, "awesome", "agents/reviewer.md", "agents/reviewer.md", false)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if promoteRes.BytesCopied != int64(len("# Reviewer agent\n")) {
		t.Errorf("BytesCopied = %d", promoteRes.BytesCopied)
	}
	if _, err := os.Stat(filepath.Join(registry, "agents", "reviewer.md")); err != nil {
		t.Errorf("promoted file missing: %v", err)
	}

	// Audit again — still clean (vendored copy untouched, promoted file matches).
	findings, _ = Audit(context.Background(), nil, registry, cache)
	if findings.HasAny() {
		t.Errorf("audit should be clean after promote; got: %+v", findings)
	}

	// --- Trigger vendor-local drift: edit inside vendor/ ---
	if err := os.WriteFile(filepath.Join(registry, "vendor", "awesome", "agents", "reviewer.md"),
		[]byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, _ = Audit(context.Background(), nil, registry, cache)
	if len(findings.LocalDrift) == 0 {
		t.Error("expected vendor-local drift after editing vendored file")
	}

	// Re-sync should refuse without --force because of the local drift.
	_, err = Sync(context.Background(), git.ExecGit{}, decls[0], registry, cache, false)
	if err == nil {
		t.Error("expected sync to refuse on vendor-local drift without --force")
	}

	// With --force, sync overwrites and clears the drift.
	if _, err := Sync(context.Background(), git.ExecGit{}, decls[0], registry, cache, true); err != nil {
		t.Fatalf("Sync --force: %v", err)
	}
	findings, _ = Audit(context.Background(), nil, registry, cache)
	if len(findings.LocalDrift) > 0 {
		t.Errorf("after --force sync, local drift should be gone; got: %+v", findings.LocalDrift)
	}

	// --- Trigger promotion drift: edit the PROMOTED copy ---
	if err := os.WriteFile(filepath.Join(registry, "agents", "reviewer.md"),
		[]byte("# Reviewer (locally customized)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, _ = Audit(context.Background(), nil, registry, cache)
	if len(findings.PromotionDrift) == 0 {
		t.Error("expected promotion drift after editing promoted copy")
	}

	// --- Trigger upstream drift: new commit in upstream ---
	if err := os.WriteFile(filepath.Join(upstream, "agents", "another.md"), []byte("# another\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", "-A")
	runGit(t, upstream, "commit", "-q", "-m", "add another")

	findings, err = Audit(context.Background(), git.ExecGit{}, registry, cache)
	if err != nil {
		t.Fatal(err)
	}
	foundUpstream := false
	for _, u := range findings.UpstreamDrift {
		if u.Vendor == "awesome" && u.LocalSHA != u.UpstreamSHA {
			foundUpstream = true
			break
		}
	}
	if !foundUpstream {
		t.Errorf("expected upstream drift after upstream commit; got: %+v", findings.UpstreamDrift)
	}
}

// TestPromoteRefusesWithoutLockfileEntry checks that promote refuses when
// the vendor has never been synced (no lockfile entry → no SHA anchor).
func TestPromoteRefusesWithoutLockfileEntry(t *testing.T) {
	root := t.TempDir()
	// Create the vendor file by hand (without sync producing a lockfile).
	if err := os.MkdirAll(filepath.Join(root, "vendor", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "x", "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Promote(root, "x", "a.md", "agents/a.md", false)
	if err == nil {
		t.Fatal("expected error promoting an unsynced vendor, got nil")
	}
}

// TestPromoteRefusesPathTraversal checks that promote rejects sneaky
// destinations that would escape the registry root.
func TestPromoteRefusesPathTraversal(t *testing.T) {
	root := t.TempDir()
	// Set up a minimal valid vendor so promote gets past the source check.
	if err := os.MkdirAll(filepath.Join(root, "vendor", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "x", "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	lf := &Lockfile{
		Version: 1,
		Vendors: map[string]VendorState{
			"x": {Files: map[string]FileRecord{"a.md": {SHA256: "doesntmatter", Size: 1}}},
		},
	}
	_ = SaveLockfile(root, lf)

	cases := []string{
		"../escaped.md",
		"agents/../../escaped.md",
		"vendor/x/a.md", // refusing to promote back into vendor/
	}
	for _, dest := range cases {
		_, err := Promote(root, "x", "a.md", dest, true)
		if err == nil {
			t.Errorf("promote(dest=%q) should have refused", dest)
		}
	}
}
