package mount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestToolRegistryMatchesProjectionAudit verifies that the tools listed under
// `csaw_projection_audit.in_code_now` in docs/reference/tool-projection.json
// exactly match the keys of ToolRegistry in this package.
//
// This catches drift when someone adds/removes a tool in code without updating
// the projection JSON (or vice versa).
func TestToolRegistryMatchesProjectionAudit(t *testing.T) {
	// Find the JSON relative to this test file (works from any pwd).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	// internal/mount/ → repo root is two levels up.
	jsonPath := filepath.Join(wd, "..", "..", "docs", "reference", "tool-projection.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read %s: %v", jsonPath, err)
	}

	var doc struct {
		CSAWProjectionAudit struct {
			InCodeNow []struct {
				Tool string `json:"tool"`
			} `json:"in_code_now"`
		} `json:"csaw_projection_audit"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse tool-projection.json: %v", err)
	}

	auditTools := map[string]bool{}
	for _, t := range doc.CSAWProjectionAudit.InCodeNow {
		auditTools[t.Tool] = true
	}

	registryTools := map[string]bool{}
	for name := range ToolRegistry {
		registryTools[name] = true
	}

	var missingFromAudit []string
	for name := range registryTools {
		if !auditTools[name] {
			missingFromAudit = append(missingFromAudit, name)
		}
	}
	var missingFromRegistry []string
	for name := range auditTools {
		if !registryTools[name] {
			missingFromRegistry = append(missingFromRegistry, name)
		}
	}
	sort.Strings(missingFromAudit)
	sort.Strings(missingFromRegistry)

	if len(missingFromAudit) > 0 {
		t.Errorf("ToolRegistry has tools missing from tool-projection.json csaw_projection_audit.in_code_now: %v\n"+
			"Either add them to the JSON's in_code_now list, or remove them from ToolRegistry.", missingFromAudit)
	}
	if len(missingFromRegistry) > 0 {
		t.Errorf("tool-projection.json csaw_projection_audit.in_code_now lists tools missing from ToolRegistry: %v\n"+
			"Either add them to ToolRegistry, or remove them from the JSON's in_code_now list.", missingFromRegistry)
	}
}
