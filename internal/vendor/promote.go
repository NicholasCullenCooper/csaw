package vendor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PromoteResult is the outcome of a successful promotion.
type PromoteResult struct {
	Vendor      string
	VendorPath  string
	PromotedTo  string
	BytesCopied int64
	Replaced    bool // destination already existed and was overwritten
}

// Promote copies a vendored file from <registryRoot>/vendor/<vendor>/<vendorPath>
// to <registryRoot>/<promotedTo>, records the lineage in the lockfile, and
// returns a Result for caller display.
//
// Refuses to overwrite an existing file at promotedTo unless force is true.
// Refuses if the vendor path doesn't exist in vendor/<vendor>/.
// Refuses if the vendor isn't tracked in the lockfile (must `csaw vendor sync`
// before promoting — guarantees we have a SHA to anchor lineage to).
//
// The vendored file STAYS in vendor/<vendor>/ — promotion is a copy, not a
// move. This preserves the vendor area as the immutable upstream record;
// the promoted copy lives in the regular kind dirs (skills/, agents/, etc.)
// and projects normally.
func Promote(registryRoot, vendor, vendorPath, promotedTo string, force bool) (PromoteResult, error) {
	res := PromoteResult{Vendor: vendor, VendorPath: vendorPath, PromotedTo: promotedTo}

	if vendor == "" || vendorPath == "" || promotedTo == "" {
		return res, fmt.Errorf("vendor, vendorPath, and promotedTo are all required")
	}
	// Defense against escaping out of the registry via "..".
	if filepath.IsAbs(promotedTo) || filepath.IsAbs(vendorPath) {
		return res, fmt.Errorf("paths must be relative")
	}
	cleanPromoted := filepath.Clean(promotedTo)
	if cleanPromoted == "." || cleanPromoted == "" {
		return res, fmt.Errorf("promoted-to path %q is invalid", promotedTo)
	}
	if hasParentTraversal(promotedTo) || hasParentTraversal(vendorPath) {
		return res, fmt.Errorf("paths must not contain '..' segments")
	}
	// Disallow promoting INTO the vendor area — that would defeat the purpose.
	if firstSegment(cleanPromoted) == "vendor" {
		return res, fmt.Errorf("refusing to promote into vendor/ — promote into a real kind directory (skills/, agents/, rules/, etc.)")
	}

	srcAbs := filepath.Join(registryRoot, "vendor", vendor, vendorPath)
	dstAbs := filepath.Join(registryRoot, cleanPromoted)

	// Verify source exists and is a regular file.
	srcInfo, err := os.Stat(srcAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return res, fmt.Errorf("vendor file not found: vendor/%s/%s — has the vendor been synced?", vendor, vendorPath)
		}
		return res, fmt.Errorf("stat vendor file: %w", err)
	}
	if srcInfo.IsDir() {
		return res, fmt.Errorf("vendor path is a directory; specify a file path")
	}

	// Require the vendor to be tracked in the lockfile so we have a SHA
	// anchor to record on the promotion.
	lf, err := LoadLockfile(registryRoot)
	if err != nil {
		return res, err
	}
	state, locked := lf.Vendors[vendor]
	if !locked {
		return res, fmt.Errorf("vendor %q is not in the lockfile — run `csaw vendor sync %s` first", vendor, vendor)
	}
	fileRec, hasRec := state.Files[vendorPath]
	if !hasRec {
		return res, fmt.Errorf("vendor file vendor/%s/%s is not tracked in the lockfile — re-sync the vendor", vendor, vendorPath)
	}

	// Refuse to overwrite without --force.
	if _, err := os.Stat(dstAbs); err == nil {
		if !force {
			return res, fmt.Errorf("destination %s already exists; re-run with --force to overwrite", cleanPromoted)
		}
		res.Replaced = true
	} else if !os.IsNotExist(err) {
		return res, fmt.Errorf("stat destination: %w", err)
	}

	// Copy + hash for verification.
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return res, fmt.Errorf("create destination dir: %w", err)
	}
	sum, n, err := copyAndHash(srcAbs, dstAbs)
	if err != nil {
		return res, fmt.Errorf("copy %s -> %s: %w", srcAbs, dstAbs, err)
	}
	res.BytesCopied = n

	// Sanity-check: the SHA of what we just copied must match the
	// lockfile-tracked SHA of the source. If not, the vendor file has
	// drifted since the last sync (detectable by Audit) and we shouldn't
	// be promoting in that state. Belt and suspenders.
	if sum != fileRec.SHA256 {
		// Roll back the partial copy to keep state clean.
		if !res.Replaced {
			_ = os.Remove(dstAbs)
		}
		return res, fmt.Errorf("vendor file vendor/%s/%s has local drift (SHA %s vs lockfile %s) — run `csaw vendor audit` and re-sync before promoting", vendor, vendorPath, sum[:12], fileRec.SHA256[:12])
	}

	// Record the promotion.
	lf.AppendPromotion(Promotion{
		Vendor:                vendor,
		VendorPath:            vendorPath,
		PromotedTo:            cleanPromoted,
		PromotedAt:            time.Now().UTC(),
		VendorSHA256AtPromote: fileRec.SHA256,
	})
	if err := SaveLockfile(registryRoot, lf); err != nil {
		return res, fmt.Errorf("update lockfile: %w", err)
	}

	return res, nil
}

func hasParentTraversal(p string) bool {
	parts := filepath.SplitList(filepath.Clean(p))
	if len(parts) > 1 {
		// SplitList uses OS list separator (':' or ';'), not path separator.
		// Use filepath.Clean + check each segment manually.
		_ = parts
	}
	clean := filepath.Clean(p)
	for _, seg := range filepath.SplitList(":" + clean) {
		_ = seg
	}
	// Simpler: just check whether the cleaned path contains a ".." segment.
	for _, segment := range splitPath(clean) {
		if segment == ".." {
			return true
		}
	}
	return false
}

func splitPath(p string) []string {
	var out []string
	for _, seg := range filepathSplit(p) {
		if seg == "" {
			continue
		}
		out = append(out, seg)
	}
	return out
}

func filepathSplit(p string) []string {
	// Use filepath.Separator-aware split that works on both unix and windows.
	var segs []string
	for {
		dir, name := filepath.Split(p)
		if name == "" {
			if dir != "" {
				segs = append([]string{dir}, segs...)
			}
			break
		}
		segs = append([]string{name}, segs...)
		p = filepath.Clean(dir)
		if p == "." || p == string(filepath.Separator) {
			break
		}
	}
	return segs
}

func firstSegment(p string) string {
	segs := splitPath(p)
	if len(segs) == 0 {
		return ""
	}
	return segs[0]
}
