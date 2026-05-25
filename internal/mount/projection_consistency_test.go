package mount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestToolRegistryMatchesProjectionJSON verifies that the tools marked
// `csaw_in_code: true` in docs/reference/tool-projection.json (across both
// the `tools` and `watchlist` sections) exactly match the keys of
// ToolRegistry in this package.
//
// Catches drift when someone adds/removes a tool in code without updating
// the projection JSON (or vice versa).
func TestToolRegistryMatchesProjectionJSON(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	jsonPath := filepath.Join(wd, "..", "..", "docs", "reference", "tool-projection.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read %s: %v", jsonPath, err)
	}

	var doc struct {
		Tools map[string]struct {
			CSAWInCode bool `json:"csaw_in_code"`
		} `json:"tools"`
		Watchlist struct {
			Tools map[string]struct {
				CSAWInCode bool `json:"csaw_in_code"`
			} `json:"tools"`
		} `json:"watchlist"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse tool-projection.json: %v", err)
	}

	jsonInCode := map[string]bool{}
	for name, t := range doc.Tools {
		if t.CSAWInCode {
			jsonInCode[name] = true
		}
	}
	for name, t := range doc.Watchlist.Tools {
		if t.CSAWInCode {
			jsonInCode[name] = true
		}
	}

	registryTools := map[string]bool{}
	for name := range ToolRegistry {
		registryTools[name] = true
	}

	var missingFromJSON []string
	for name := range registryTools {
		if !jsonInCode[name] {
			missingFromJSON = append(missingFromJSON, name)
		}
	}
	var missingFromRegistry []string
	for name := range jsonInCode {
		if !registryTools[name] {
			missingFromRegistry = append(missingFromRegistry, name)
		}
	}
	sort.Strings(missingFromJSON)
	sort.Strings(missingFromRegistry)

	if len(missingFromJSON) > 0 {
		t.Errorf("ToolRegistry has tools missing csaw_in_code:true in tool-projection.json: %v\n"+
			"Set csaw_in_code:true on each in the JSON's tools or watchlist section.", missingFromJSON)
	}
	if len(missingFromRegistry) > 0 {
		t.Errorf("tool-projection.json marks tools csaw_in_code:true that are missing from ToolRegistry: %v\n"+
			"Either add them to ToolRegistry, or set csaw_in_code:false in the JSON.", missingFromRegistry)
	}
}
