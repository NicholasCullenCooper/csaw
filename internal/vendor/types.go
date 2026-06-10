// Package vendor implements csaw's vendor feature: safely consuming
// external agent-context catalogs (skills.sh, APM packages,
// awesome-copilot, internal bundle manifests, any git repo) without
// letting upstream layouts become active mounted context.
//
// The architectural primitive is a third state for files: in addition to
// "in a configured source" (mounts to project) and "not in any source"
// (doesn't exist), vendor adds "fetched from upstream, hash-locked, never
// projects, requires explicit promote to enter a real source." See
// docs/planning/vendors-design.md for the full design rationale.
package vendor

import "time"

// Declaration is a single entry in a source registry's `vendors:` block of
// csaw.yml. Authored by hand or written by `csaw vendor add`.
type Declaration struct {
	// Name is the local identifier — used as the directory name under
	// vendor/<Name>/ and as the key in vendor.lock.yaml.
	Name string `yaml:"-"` // map key in csaw.yml, not a field
	// URL is the upstream git URL. Shorthand (gh:/gl:/bb:) is resolved
	// to a canonical URL at parse time.
	URL string `yaml:"url"`
	// Ref is the requested git ref (branch, tag, or commit). Empty means
	// the upstream default branch.
	Ref string `yaml:"ref,omitempty"`
	// Include is an optional list of glob patterns (gitignore-style) used
	// to filter files copied from upstream into vendor/<Name>/. Empty
	// means copy everything.
	Include []string `yaml:"include,omitempty"`
	// Exclude is an optional list of glob patterns to filter out after
	// Include matching. Applied after Include.
	Exclude []string `yaml:"exclude,omitempty"`
}

// Lockfile is the on-disk representation of vendor.lock.yaml at the
// registry root. Records per-vendor sync state and an append-only log of
// promotions.
type Lockfile struct {
	// Version is the lockfile schema version, currently 1.
	Version int `yaml:"version"`
	// Vendors maps vendor name → per-vendor sync state.
	Vendors map[string]VendorState `yaml:"vendors"`
	// Promotions is the append-only log of `csaw vendor promote` actions.
	// Used for lineage tracking and promotion-drift detection in audit.
	Promotions []Promotion `yaml:"promotions,omitempty"`
}

// VendorState records the most-recent sync of one vendor. Drift detection
// compares the recorded files map against the current contents of
// vendor/<Name>/ on disk.
type VendorState struct {
	URL          string                `yaml:"url"`
	RefRequested string                `yaml:"ref_requested,omitempty"`
	RefResolved  string                `yaml:"ref_resolved"` // exact SHA at sync time
	SyncedAt     time.Time             `yaml:"synced_at"`
	Files        map[string]FileRecord `yaml:"files"` // path within vendor/<name>/ → hash + size
}

// FileRecord pins the integrity of a vendored file.
type FileRecord struct {
	SHA256 string `yaml:"sha256"`
	Size   int64  `yaml:"size"`
}

// Promotion records a single `csaw vendor promote` action. Append-only;
// supports promotion-drift detection (did the vendored origin move since
// promote? did the promoted copy drift in the consumer's working tree?).
type Promotion struct {
	Vendor                string    `yaml:"vendor"`
	VendorPath            string    `yaml:"vendor_path"` // path within vendor/<Vendor>/
	PromotedTo            string    `yaml:"promoted_to"` // path within the registry root
	PromotedAt            time.Time `yaml:"promoted_at"`
	VendorSHA256AtPromote string    `yaml:"vendor_sha256_at_promote"` // anchor for promotion-drift
}

// VendorMeta is the small per-vendor metadata file written inside
// vendor/<Name>/ on every sync. Lives alongside the vendored content for
// quick provenance discovery without parsing the lockfile.
type VendorMeta struct {
	URL         string    `yaml:"url"`
	RefResolved string    `yaml:"ref_resolved"`
	SyncedAt    time.Time `yaml:"synced_at"`
}
