package mount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// projectionDoc mirrors the parts of docs/reference/tool-projection.json the
// consistency tests need. The JSON documents what each tool reads (research
// with sources and confidence); the Go ToolRegistry decides what csaw writes.
// These tests keep the two from drifting apart.
type projectionDoc struct {
	Tools map[string]struct {
		CSAWInCode bool   `json:"csaw_in_code"`
		CheckedAt  string `json:"checked_at"`
		Kinds      map[string]struct {
			ProjectPaths []string `json:"project_paths"`
		} `json:"kinds"`
	} `json:"tools"`
	Watchlist struct {
		Tools map[string]struct {
			CSAWInCode bool `json:"csaw_in_code"`
		} `json:"tools"`
	} `json:"watchlist"`
}

func loadProjectionDoc(t *testing.T) projectionDoc {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	jsonPath := filepath.Join(wd, "..", "..", "docs", "reference", "tool-projection.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read %s: %v", jsonPath, err)
	}
	var doc projectionDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse tool-projection.json: %v", err)
	}
	return doc
}

// TestToolRegistryMatchesProjectionJSON verifies that the tools marked
// `csaw_in_code: true` in docs/reference/tool-projection.json exactly match
// the keys of ToolRegistry.
func TestToolRegistryMatchesProjectionJSON(t *testing.T) {
	doc := loadProjectionDoc(t)

	jsonInCode := map[string]bool{}
	for name, tool := range doc.Tools {
		if tool.CSAWInCode {
			jsonInCode[name] = true
		}
	}
	for name, tool := range doc.Watchlist.Tools {
		if tool.CSAWInCode {
			jsonInCode[name] = true
		}
	}

	var missingFromJSON, missingFromRegistry []string
	for name := range ToolRegistry {
		if !jsonInCode[name] {
			missingFromJSON = append(missingFromJSON, name)
		}
	}
	for name := range jsonInCode {
		if _, ok := ToolRegistry[name]; !ok {
			missingFromRegistry = append(missingFromRegistry, name)
		}
	}
	sort.Strings(missingFromJSON)
	sort.Strings(missingFromRegistry)

	if len(missingFromJSON) > 0 {
		t.Errorf("ToolRegistry has tools missing csaw_in_code:true in tool-projection.json: %v", missingFromJSON)
	}
	if len(missingFromRegistry) > 0 {
		t.Errorf("tool-projection.json marks tools csaw_in_code:true that are not in ToolRegistry: %v", missingFromRegistry)
	}
}

var jsonKindNames = map[Kind]string{
	KindSkill: "skills",
	KindAgent: "agents",
	KindRule:  "rules",
	KindHook:  "hooks",
}

// TestProjectionPathsAreDocumented verifies that every directory the Go
// registry reads or writes for a tool is documented in that tool's JSON
// entry, so a path added in code without research behind it fails CI.
func TestProjectionPathsAreDocumented(t *testing.T) {
	doc := loadProjectionDoc(t)

	for _, name := range AllToolNames() {
		spec := ToolRegistry[name]
		entry, ok := doc.Tools[name]
		if !ok {
			t.Errorf("%s: no entry in tool-projection.json tools section", name)
			continue
		}
		for kind, dirs := range spec.Reads {
			jsonKind := jsonKindNames[kind]
			documented := entry.Kinds[jsonKind].ProjectPaths
			for _, dir := range dirs {
				if !pathDocumented(dir, documented) {
					t.Errorf("%s/%s: Go reads %q but tool-projection.json project_paths are %v", name, jsonKind, dir, documented)
				}
			}
			if dir, ok := spec.Writes[kind]; ok && !pathDocumented(dir, documented) {
				t.Errorf("%s/%s: Go writes %q but tool-projection.json project_paths are %v", name, jsonKind, dir, documented)
			}
		}
	}
}

func pathDocumented(dir string, documented []string) bool {
	dir = strings.TrimSuffix(dir, "/")
	for _, candidate := range documented {
		candidate = strings.TrimSuffix(candidate, "/")
		if candidate == dir || strings.HasPrefix(candidate, dir+"/") {
			return true
		}
	}
	return false
}

// TestProjectionResearchFreshness logs in-code tools whose research is older
// than 120 days. It never fails: vendor docs move, and the log line is the
// reminder to re-verify before a release.
func TestProjectionResearchFreshness(t *testing.T) {
	doc := loadProjectionDoc(t)
	const maxAge = 120 * 24 * time.Hour

	for _, name := range AllToolNames() {
		entry := doc.Tools[name]
		if entry.CheckedAt == "" {
			t.Logf("%s: no checked_at in tool-projection.json; verify against vendor docs", name)
			continue
		}
		checked, err := time.Parse("2006-01-02", entry.CheckedAt)
		if err != nil {
			t.Errorf("%s: checked_at %q is not YYYY-MM-DD", name, entry.CheckedAt)
			continue
		}
		if age := time.Since(checked); age > maxAge {
			t.Logf("%s: research is %d days old; re-verify against vendor docs", name, int(age.Hours()/24))
		}
	}
}
