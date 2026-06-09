package mcpmerge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Plan describes the result of a dry-run merge against a target file.
// Renders cleanly for the CLI dry-run output.
type Plan struct {
	Target            MergeTarget
	TargetPath        string   // absolute path
	TargetExists      bool     // does the target file currently exist on disk?
	HasManagedSection bool     // is there already a csaw-managed section to replace?
	UserOwnedServers  []string // mcp_servers.* names found OUTSIDE the managed section
	Fragment          Fragment
	Conflicts         []Conflict  // entries to skip because the user already has same name
	WillApply         []MCPServer // entries that will actually be merged
}

// Conflict describes a server entry csaw will NOT write because the user
// already has an entry of the same name outside the managed section.
type Conflict struct {
	ServerName string
	Reason     string
}

// Result is returned by Apply / Remove for caller display.
type Result struct {
	TargetPath  string
	BytesBefore int
	BytesAfter  int
	Added       []string
	Removed     []string
	Skipped     []Conflict
}

// PlanMerge dry-runs the merge: reads the target file (if it exists),
// detects any existing managed section + the user's pre-existing
// mcp_servers entries, and identifies which fragment servers would apply
// vs. which conflict with user entries.
//
// Does NOT write anything.
func PlanMerge(target MergeTarget, projectRoot string, fragment Fragment) (Plan, error) {
	targetPath := filepath.Join(projectRoot, target.ProjectPath)
	plan := Plan{
		Target:     target,
		TargetPath: targetPath,
		Fragment:   fragment,
	}

	content, err := os.ReadFile(targetPath)
	switch {
	case err == nil:
		plan.TargetExists = true
	case os.IsNotExist(err):
		// fine — fresh file
	default:
		return plan, fmt.Errorf("read %s: %w", targetPath, err)
	}

	var userContent []byte
	if plan.TargetExists {
		// Find existing managed section so we can exclude it from the
		// "user's existing servers" check.
		start, end, found, err := FindManagedSection(content, target)
		if err != nil {
			return plan, err
		}
		plan.HasManagedSection = found
		if found {
			userContent = append([]byte{}, content[:start]...)
			userContent = append(userContent, content[end:]...)
		} else {
			userContent = content
		}

		// Parse the user-content portion to discover existing mcp_servers
		// names that csaw would conflict with.
		if target.Format == FormatTOML {
			var parsed map[string]interface{}
			if err := toml.Unmarshal(userContent, &parsed); err != nil {
				return plan, fmt.Errorf("parse user portion of %s: %w (file may be syntactically invalid)", targetPath, err)
			}
			if mcpsRaw, ok := parsed["mcp_servers"]; ok {
				if mcps, ok := mcpsRaw.(map[string]interface{}); ok {
					for name := range mcps {
						plan.UserOwnedServers = append(plan.UserOwnedServers, name)
					}
				}
			}
		}
	}

	// Decide apply vs. skip per fragment server.
	userOwned := stringSet(plan.UserOwnedServers)
	for _, s := range fragment.Servers {
		if userOwned[s.Name] {
			plan.Conflicts = append(plan.Conflicts, Conflict{
				ServerName: s.Name,
				Reason:     "user already defines this server outside the csaw-managed section",
			})
			continue
		}
		plan.WillApply = append(plan.WillApply, s)
	}

	return plan, nil
}

// Apply writes the plan to disk: produces the new target-file content,
// writes it atomically (temp + rename), and persists a state manifest so
// Remove can detect drift later. Returns a Result describing what changed.
//
// The caller is responsible for any user confirmation — Apply assumes the
// user has already opted in by passing --apply at the CLI.
func Apply(plan Plan, stateDir string) (Result, error) {
	res := Result{TargetPath: plan.TargetPath}

	// Read existing content (may not exist).
	var existing []byte
	if plan.TargetExists {
		var err error
		existing, err = os.ReadFile(plan.TargetPath)
		if err != nil {
			return res, fmt.Errorf("read %s: %w", plan.TargetPath, err)
		}
		res.BytesBefore = len(existing)
	}

	// Build the new content: original content minus any existing managed
	// section, plus the new managed section appended.
	newContent, err := composeNewContent(existing, plan)
	if err != nil {
		return res, err
	}
	res.BytesAfter = len(newContent)

	// Write atomically.
	if err := os.MkdirAll(filepath.Dir(plan.TargetPath), 0o755); err != nil {
		return res, fmt.Errorf("ensure target dir: %w", err)
	}
	if err := atomicWrite(plan.TargetPath, newContent); err != nil {
		return res, err
	}

	// Persist state manifest for rollback drift detection.
	if err := writeManifest(stateDir, plan); err != nil {
		// Don't unwind the write — manifest write failure shouldn't
		// re-corrupt the target. Surface as a non-fatal warning.
		return res, fmt.Errorf("merge applied but state manifest write failed: %w (rollback drift detection will be unreliable)", err)
	}

	for _, s := range plan.WillApply {
		res.Added = append(res.Added, s.Name)
	}
	res.Skipped = plan.Conflicts
	return res, nil
}

// Remove undoes a previous Apply: locates the bounded section, verifies its
// hash matches the manifest (drift check), and rewrites the target file
// without the section. Returns error if the managed-section content has
// drifted from what csaw wrote (user edited inside the bounds) — caller
// must decide whether to override.
func Remove(target MergeTarget, projectRoot string, stateDir string) (Result, error) {
	targetPath := filepath.Join(projectRoot, target.ProjectPath)
	res := Result{TargetPath: targetPath}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return res, fmt.Errorf("target %s does not exist — nothing to remove", targetPath)
		}
		return res, err
	}
	res.BytesBefore = len(content)

	start, end, found, err := FindManagedSection(content, target)
	if err != nil {
		return res, err
	}
	if !found {
		return res, fmt.Errorf("no csaw-managed section found in %s — nothing to remove", targetPath)
	}

	section := content[start:end]

	// Drift check against manifest.
	manifest, err := readManifest(stateDir, target)
	if err == nil {
		actualHash := sha256.Sum256(section)
		if hex.EncodeToString(actualHash[:]) != manifest.SectionSHA256 {
			return res, fmt.Errorf("csaw-managed section in %s has been modified since csaw wrote it (SHA mismatch) — refusing to remove. Inspect the section, then re-run with --force to remove anyway", targetPath)
		}
		for _, name := range manifest.Servers {
			res.Removed = append(res.Removed, name)
		}
	}
	// Manifest absent is non-fatal: removal still works, drift just can't
	// be detected. Common after fresh clone or csaw home wipe.

	// Strip preceding separator whitespace that Apply added. If start-2..start
	// is "\n\n", that's the blank-line separator we inserted between user
	// content and our section — remove it. After Apply, base was
	// TrimRight'd then `\n\n` was appended, so the round-trip restore is:
	// drop both inserted newlines and re-emit a single trailing newline.
	before := content[:start]
	if len(before) >= 2 && before[len(before)-1] == '\n' && before[len(before)-2] == '\n' {
		before = before[:len(before)-2]
		// Restore a single trailing newline to keep the file tidy
		// (and to match the typical original-file convention).
		if len(before) > 0 {
			before = append(before, '\n')
		}
	}

	newContent := append([]byte{}, before...)
	newContent = append(newContent, content[end:]...)
	res.BytesAfter = len(newContent)

	if err := atomicWrite(targetPath, newContent); err != nil {
		return res, err
	}

	// Best-effort manifest cleanup.
	_ = os.Remove(manifestPath(stateDir, target))

	return res, nil
}

// composeNewContent builds the result file: original content with any
// existing csaw-managed section excised, followed by the freshly-rendered
// managed section.
func composeNewContent(existing []byte, plan Plan) ([]byte, error) {
	// Excise existing managed section if any.
	var base []byte
	if plan.HasManagedSection {
		start, end, found, err := FindManagedSection(existing, plan.Target)
		if err != nil {
			return nil, err
		}
		if !found {
			// HasManagedSection was set from a prior call; defensively re-derive.
			base = existing
		} else {
			base = append([]byte{}, existing[:start]...)
			base = append(base, existing[end:]...)
		}
	} else {
		base = existing
	}

	// Normalize trailing whitespace, then place the managed section after
	// a blank-line separator if there's existing content (visual spacing).
	// The separator bytes live OUTSIDE the managed section so Remove can
	// strip the section without worrying about preceding whitespace.
	base = bytes.TrimRight(base, " \t\n")
	rendered := RenderManagedSection(plan.WillApply, plan.Target, plan.Fragment.SourceName)
	if len(base) > 0 {
		base = append(base, '\n', '\n')
	}
	return append(base, rendered...), nil
}

func atomicWrite(path string, content []byte) error {
	tmp := path + ".csaw.tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write temp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s → %s: %w", tmp, path, err)
	}
	return nil
}

// --- state manifest ---

// Manifest is the on-disk record of what csaw wrote into a target file.
// Used by Remove to detect post-write drift.
type Manifest struct {
	TargetName    string    `json:"target_name"`
	TargetPath    string    `json:"target_path"`
	SourceName    string    `json:"source_name"`
	Servers       []string  `json:"servers"`
	SectionSHA256 string    `json:"section_sha256"`
	WrittenAt     time.Time `json:"written_at"`
}

func manifestPath(stateDir string, target MergeTarget) string {
	return filepath.Join(stateDir, "mcpmerge-"+target.Name+".json")
}

func writeManifest(stateDir string, plan Plan) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	// Recompute the rendered section SHA on the freshly-rendered bytes.
	rendered := RenderManagedSection(plan.WillApply, plan.Target, plan.Fragment.SourceName)
	sum := sha256.Sum256(rendered)

	names := make([]string, len(plan.WillApply))
	for i, s := range plan.WillApply {
		names[i] = s.Name
	}

	m := Manifest{
		TargetName:    plan.Target.Name,
		TargetPath:    plan.TargetPath,
		SourceName:    plan.Fragment.SourceName,
		Servers:       names,
		SectionSHA256: hex.EncodeToString(sum[:]),
		WrittenAt:     time.Now().UTC(),
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(stateDir, plan.Target), data, 0o644)
}

func readManifest(stateDir string, target MergeTarget) (Manifest, error) {
	data, err := os.ReadFile(manifestPath(stateDir, target))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it] = true
	}
	return set
}
