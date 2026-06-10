package vendor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/git"
)

// vendorMetaFileName is written inside each vendor/<name>/ directory
// alongside the vendored content so provenance is discoverable without
// opening the lockfile. Prefixed with a dot so it sorts cleanly and
// doesn't get mistaken for content.
const vendorMetaFileName = ".csaw-vendor-meta.yaml"

// SyncResult describes what a sync did, for caller display.
type SyncResult struct {
	Name        string
	URL         string
	RefResolved string
	FilesAdded  int
	FilesUpdate int
	FilesGone   int  // present in last lockfile, no longer in upstream
	Skipped     bool // upstream had no changes since last sync
}

// Sync fetches an upstream vendor and copies filtered content into
// <registryRoot>/vendor/<name>/. Updates the lockfile with per-file SHAs
// and writes a small VendorMeta file inside the vendor dir.
//
// cacheRoot is csaw's vendor cache directory (typically <paths.State>/vendor-cache).
// Multiple registries vendoring the same URL share the cache.
//
// Refuses to overwrite if vendor-local drift is detected (a vendored file's
// current SHA differs from the lockfile entry) unless force is true.
func Sync(ctx context.Context, g git.Git, decl Declaration, registryRoot, cacheRoot string, force bool) (SyncResult, error) {
	res := SyncResult{Name: decl.Name, URL: decl.URL}

	// 1. Pre-flight: check for vendor-local drift before destructive ops.
	if !force {
		drifted, err := detectVendorLocalDrift(decl.Name, registryRoot)
		if err != nil {
			return res, fmt.Errorf("preflight drift check for %s: %w", decl.Name, err)
		}
		if len(drifted) > 0 {
			return res, fmt.Errorf("vendor %q has local edits in: %v — run `csaw vendor audit` to inspect, or re-run with --force to overwrite", decl.Name, drifted)
		}
	}

	// 2. Ensure cache for this vendor URL exists.
	cachePath, err := ensureCache(ctx, g, decl.URL, cacheRoot)
	if err != nil {
		return res, err
	}

	// 3. Resolve the requested ref to a SHA.
	ref := decl.Ref
	if ref == "" {
		// Use the cached repo's current default-branch HEAD.
		head, err := g.Run(ctx, cachePath, "rev-parse", "HEAD")
		if err != nil {
			return res, fmt.Errorf("resolve default HEAD in cache: %w", err)
		}
		res.RefResolved = strings.TrimSpace(head)
	} else {
		// Try as branch/tag/commit. Fetch first so newly-pushed refs resolve.
		if _, err := g.Run(ctx, cachePath, "fetch", "origin", ref); err != nil {
			// Non-fatal: ref might be a local SHA already in the cache.
		}
		resolved, err := g.Run(ctx, cachePath, "rev-parse", ref)
		if err != nil {
			return res, fmt.Errorf("resolve ref %q in %s: %w", ref, decl.URL, err)
		}
		res.RefResolved = strings.TrimSpace(resolved)
	}

	// 4. Checkout the resolved SHA in the cache (detached HEAD is fine; the
	// cache is a vendoring workspace, not a working tree the user touches).
	if _, err := g.Run(ctx, cachePath, "checkout", "--detach", res.RefResolved); err != nil {
		return res, fmt.Errorf("checkout %s in cache: %w", res.RefResolved, err)
	}

	// 5. Walk files matching include/exclude, copy + hash.
	vendorDir := filepath.Join(registryRoot, "vendor", decl.Name)
	files, err := collectAndCopy(cachePath, vendorDir, decl.Include, decl.Exclude)
	if err != nil {
		return res, fmt.Errorf("collect+copy vendor files: %w", err)
	}
	res.FilesAdded = len(files)

	// 6. Write the per-vendor meta file (small provenance pointer).
	meta := VendorMeta{
		URL:         decl.URL,
		RefResolved: res.RefResolved,
		SyncedAt:    time.Now().UTC(),
	}
	metaBytes, _ := yaml.Marshal(meta)
	metaPath := filepath.Join(vendorDir, vendorMetaFileName)
	if err := os.WriteFile(metaPath, metaBytes, 0o644); err != nil {
		return res, fmt.Errorf("write vendor meta: %w", err)
	}

	// 7. Update the lockfile.
	lf, err := LoadLockfile(registryRoot)
	if err != nil {
		return res, err
	}
	lf.Vendors[decl.Name] = VendorState{
		URL:          decl.URL,
		RefRequested: decl.Ref,
		RefResolved:  res.RefResolved,
		SyncedAt:     meta.SyncedAt,
		Files:        files,
	}
	if err := SaveLockfile(registryRoot, lf); err != nil {
		return res, err
	}

	return res, nil
}

// detectVendorLocalDrift returns the list of vendor-paths whose current
// on-disk SHA differs from the lockfile's recorded SHA. Returns empty if
// no drift (including when the lockfile or vendor dir doesn't exist).
func detectVendorLocalDrift(name, registryRoot string) ([]string, error) {
	lf, err := LoadLockfile(registryRoot)
	if err != nil {
		return nil, err
	}
	state, ok := lf.Vendors[name]
	if !ok {
		return nil, nil // never synced; nothing to drift from
	}

	var drifted []string
	vendorDir := filepath.Join(registryRoot, "vendor", name)
	for relPath, rec := range state.Files {
		full := filepath.Join(vendorDir, relPath)
		current, err := hashFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				drifted = append(drifted, relPath+" (missing)")
				continue
			}
			return nil, err
		}
		if current != rec.SHA256 {
			drifted = append(drifted, relPath)
		}
	}
	sort.Strings(drifted)
	return drifted, nil
}

// ensureCache clones (if missing) or fetches (if present) the upstream
// repo into cacheRoot/<key>/. Returns the cache path.
func ensureCache(ctx context.Context, g git.Git, url, cacheRoot string) (string, error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", cacheRoot, err)
	}
	sum := sha256.Sum256([]byte(url))
	key := hex.EncodeToString(sum[:])[:16]
	cachePath := filepath.Join(cacheRoot, key)

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if _, err := g.Run(ctx, cacheRoot, "clone", url, key); err != nil {
			return "", fmt.Errorf("clone %s: %w", url, err)
		}
		return cachePath, nil
	} else if err != nil {
		return "", err
	}

	// Cache exists; fetch latest refs.
	if _, err := g.Run(ctx, cachePath, "fetch", "--all", "--tags", "--prune"); err != nil {
		return "", fmt.Errorf("fetch in cache %s: %w", cachePath, err)
	}
	return cachePath, nil
}

// collectAndCopy walks the cache, applies include/exclude filters, copies
// matching files into vendorDir, and returns a map of vendor-relative
// path → FileRecord (SHA + size).
//
// The vendor dir is wiped before copying (after pre-flight drift check)
// so removed-upstream files don't linger. The .csaw-vendor-meta.yaml file
// is preserved across this wipe by being re-written immediately after.
func collectAndCopy(cachePath, vendorDir string, include, exclude []string) (map[string]FileRecord, error) {
	// Remove old vendor content. If the dir doesn't exist, that's fine.
	if err := os.RemoveAll(vendorDir); err != nil {
		return nil, fmt.Errorf("clean vendor dir %s: %w", vendorDir, err)
	}
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir vendor dir %s: %w", vendorDir, err)
	}

	files := make(map[string]FileRecord)

	err := filepath.WalkDir(cachePath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Skip the .git directory entirely.
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		// Compute path relative to cache root.
		rel, err := filepath.Rel(cachePath, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		// Apply include filter (if any); then exclude filter.
		if !matchesAny(relSlash, include, true) {
			return nil
		}
		if matchesAny(relSlash, exclude, false) {
			return nil
		}

		// Copy file + hash.
		destPath := filepath.Join(vendorDir, rel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		sum, size, err := copyAndHash(path, destPath)
		if err != nil {
			return err
		}
		files[relSlash] = FileRecord{SHA256: sum, Size: size}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// matchesAny returns true if path matches any of the glob patterns.
// If patterns is empty, returns defaultWhenEmpty (semantics: empty
// include = include everything; empty exclude = exclude nothing).
func matchesAny(path string, patterns []string, defaultWhenEmpty bool) bool {
	if len(patterns) == 0 {
		return defaultWhenEmpty
	}
	for _, p := range patterns {
		ok, err := doublestar.Match(p, path)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// copyAndHash copies src → dst and returns the SHA-256 hex digest and
// size of the source content.
func copyAndHash(src, dst string) (string, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return "", 0, err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", 0, err
	}
	defer out.Close()

	h := sha256.New()
	mw := io.MultiWriter(out, h)
	n, err := io.Copy(mw, in)
	if err != nil {
		return "", 0, err
	}
	if n != info.Size() {
		// Defensive — should never happen, but worth catching.
		return "", n, fmt.Errorf("short copy %s: wrote %d of %d bytes", src, n, info.Size())
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// hashFile returns the SHA-256 hex digest of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
