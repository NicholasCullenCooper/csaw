package vendor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NicholasCullenCooper/csaw/internal/git"
)

// AuditFindings is the structured result of a vendor audit, organized by
// drift type. Caller formats for CLI display.
type AuditFindings struct {
	// LocalDrift: files under vendor/<name>/ whose current SHA differs
	// from the lockfile entry (someone hand-edited inside the vendor area).
	LocalDrift []DriftFinding
	// UpstreamDrift: vendors whose resolved upstream HEAD has moved since
	// the last sync (refetching would update them).
	UpstreamDrift []UpstreamFinding
	// PromotionDrift: promoted files where either (a) the vendor origin
	// changed since promote, or (b) the promoted copy was edited.
	PromotionDrift []PromotionFinding
}

// HasAny reports whether any drift was detected.
func (a AuditFindings) HasAny() bool {
	return len(a.LocalDrift) > 0 || len(a.UpstreamDrift) > 0 || len(a.PromotionDrift) > 0
}

// DriftFinding is one vendored file out of sync with its lockfile entry.
type DriftFinding struct {
	Vendor     string
	VendorPath string
	Reason     string // e.g., "modified", "missing", "extra"
}

// UpstreamFinding is one vendor whose upstream has moved.
type UpstreamFinding struct {
	Vendor       string
	URL          string
	RefRequested string // empty means default branch was tracked
	LocalSHA     string // what the lockfile pins
	UpstreamSHA  string // current upstream HEAD for the ref
}

// PromotionFinding is a promoted file whose lineage no longer matches.
type PromotionFinding struct {
	Vendor                    string
	VendorPath                string
	PromotedTo                string
	VendorChangedSincePromote bool   // vendor file SHA differs from vendor_sha256_at_promote
	PromotedFileEdited        bool   // promoted file SHA differs from vendor_sha256_at_promote
	Detail                    string // human-readable summary
}

// Audit runs all three drift checks for the registry's vendors.
//
// If g is nil, the upstream-drift check is skipped (no network access).
// Useful for CI environments that vendor for reproducibility but don't
// have credentials to reach upstream.
func Audit(ctx context.Context, g git.Git, registryRoot, cacheRoot string) (AuditFindings, error) {
	var findings AuditFindings

	lf, err := LoadLockfile(registryRoot)
	if err != nil {
		return findings, err
	}

	// 1. Vendor-local drift (per declared vendor)
	declarations, err := LoadDeclarations(registryRoot)
	if err != nil {
		return findings, err
	}
	declaredNames := make(map[string]Declaration, len(declarations))
	for _, d := range declarations {
		declaredNames[d.Name] = d
	}

	// Check each lockfile-tracked vendor for file-level drift.
	for name, state := range lf.Vendors {
		drifted, err := detectVendorLocalDrift(name, registryRoot)
		if err != nil {
			return findings, err
		}
		for _, path := range drifted {
			reason := "modified"
			if strings.HasSuffix(path, " (missing)") {
				path = strings.TrimSuffix(path, " (missing)")
				reason = "missing"
			}
			findings.LocalDrift = append(findings.LocalDrift, DriftFinding{
				Vendor: name, VendorPath: path, Reason: reason,
			})
		}
		// Also flag files on disk that aren't in the lockfile (extras).
		vendorDir := filepath.Join(registryRoot, "vendor", name)
		extras, err := findExtraFiles(vendorDir, state.Files)
		if err != nil {
			return findings, err
		}
		for _, p := range extras {
			findings.LocalDrift = append(findings.LocalDrift, DriftFinding{
				Vendor: name, VendorPath: p, Reason: "extra (not in lockfile)",
			})
		}
	}

	// 2. Upstream drift (per declared vendor, requires network)
	if g != nil {
		for _, d := range declarations {
			state, locked := lf.Vendors[d.Name]
			if !locked {
				// Declared but never synced — surface as needing initial sync.
				findings.UpstreamDrift = append(findings.UpstreamDrift, UpstreamFinding{
					Vendor: d.Name, URL: d.URL, RefRequested: d.Ref,
					LocalSHA: "", UpstreamSHA: "(never synced; run csaw vendor sync)",
				})
				continue
			}
			ref := d.Ref
			if ref == "" {
				ref = "HEAD"
			}
			upstream, err := remoteRefSHA(ctx, g, d.URL, ref)
			if err != nil {
				// Don't fail the whole audit; surface as a notice.
				findings.UpstreamDrift = append(findings.UpstreamDrift, UpstreamFinding{
					Vendor: d.Name, URL: d.URL, RefRequested: d.Ref,
					LocalSHA: state.RefResolved, UpstreamSHA: "(network check failed: " + err.Error() + ")",
				})
				continue
			}
			if upstream != state.RefResolved {
				findings.UpstreamDrift = append(findings.UpstreamDrift, UpstreamFinding{
					Vendor: d.Name, URL: d.URL, RefRequested: d.Ref,
					LocalSHA: state.RefResolved, UpstreamSHA: upstream,
				})
			}
		}
	}

	// 3. Promotion drift
	for _, p := range lf.Promotions {
		// (a) Did the vendored origin file change since promote?
		vendorAbs := filepath.Join(registryRoot, "vendor", p.Vendor, p.VendorPath)
		currentVendorSHA, err := hashFile(vendorAbs)
		vendorChanged := false
		if err == nil {
			vendorChanged = currentVendorSHA != p.VendorSHA256AtPromote
		}
		// (b) Did the promoted file get edited after promote?
		promotedAbs := filepath.Join(registryRoot, p.PromotedTo)
		currentPromotedSHA, err := hashFile(promotedAbs)
		promotedEdited := false
		if err == nil {
			promotedEdited = currentPromotedSHA != p.VendorSHA256AtPromote
		}

		if vendorChanged || promotedEdited {
			f := PromotionFinding{
				Vendor: p.Vendor, VendorPath: p.VendorPath, PromotedTo: p.PromotedTo,
				VendorChangedSincePromote: vendorChanged,
				PromotedFileEdited:        promotedEdited,
			}
			switch {
			case vendorChanged && promotedEdited:
				f.Detail = "both vendor and promoted copy changed — review divergence"
			case vendorChanged:
				f.Detail = "vendor changed since promote — consider re-promoting"
			case promotedEdited:
				f.Detail = "promoted copy edited locally — intentional divergence from vendor"
			}
			findings.PromotionDrift = append(findings.PromotionDrift, f)
		}
	}

	// Stable order for deterministic output.
	sort.Slice(findings.LocalDrift, func(i, j int) bool {
		if findings.LocalDrift[i].Vendor != findings.LocalDrift[j].Vendor {
			return findings.LocalDrift[i].Vendor < findings.LocalDrift[j].Vendor
		}
		return findings.LocalDrift[i].VendorPath < findings.LocalDrift[j].VendorPath
	})
	sort.Slice(findings.UpstreamDrift, func(i, j int) bool {
		return findings.UpstreamDrift[i].Vendor < findings.UpstreamDrift[j].Vendor
	})
	sort.Slice(findings.PromotionDrift, func(i, j int) bool {
		return findings.PromotionDrift[i].PromotedTo < findings.PromotionDrift[j].PromotedTo
	})

	// Audit lockfile-tracked vendors no longer declared — also worth surfacing.
	for name := range lf.Vendors {
		if _, ok := declaredNames[name]; !ok {
			findings.LocalDrift = append(findings.LocalDrift, DriftFinding{
				Vendor: name, VendorPath: "(entire vendor)",
				Reason: "in lockfile but not declared in csaw.yml — orphaned",
			})
		}
	}

	return findings, nil
}

// remoteRefSHA queries the upstream remote without needing a local checkout.
// Uses git ls-remote which is cheap and doesn't fetch any objects.
func remoteRefSHA(ctx context.Context, g git.Git, url, ref string) (string, error) {
	// For "HEAD" we want the symref target; ls-remote prints it as "HEAD\trefs/heads/main"
	// followed by the SHA. Simpler: use ls-remote with the explicit ref.
	out, err := g.Run(ctx, "", "ls-remote", url, ref)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		// Ref not found on remote
		return "", fmt.Errorf("ref %q not found at %s", ref, url)
	}
	// ls-remote output: SHA<tab>refname (possibly multiple lines for HEAD)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("could not parse ls-remote output for %s %s", url, ref)
}

// findExtraFiles returns paths inside vendorDir that don't appear in
// `known` (the lockfile-tracked file set). Excludes the vendor meta file.
func findExtraFiles(vendorDir string, known map[string]FileRecord) ([]string, error) {
	if _, err := os.Stat(vendorDir); os.IsNotExist(err) {
		return nil, nil
	}
	var extras []string
	err := filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(vendorDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == vendorMetaFileName {
			return nil // expected meta file, not "extra"
		}
		if _, ok := known[relSlash]; !ok {
			extras = append(extras, relSlash)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(extras)
	return extras, nil
}
