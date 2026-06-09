package mcpmerge

import (
	"strings"
	"testing"
)

func testTarget() MergeTarget {
	return MergeTarget{
		Name:        "test",
		ProjectPath: "config.toml",
		Format:      FormatTOML,
		StartMarker: "# === csaw managed start ===",
		EndMarker:   "# === csaw managed end ===",
	}
}

func TestFindManagedSectionNoMarkers(t *testing.T) {
	content := []byte("model = \"gpt-4o\"\n[other]\nx = 1\n")
	start, end, found, err := FindManagedSection(content, testTarget())
	if err != nil {
		t.Fatal(err)
	}
	if found || start != 0 || end != 0 {
		t.Errorf("expected (0,0,false), got (%d,%d,%v)", start, end, found)
	}
}

func TestFindManagedSectionLocatesBlock(t *testing.T) {
	content := []byte("preamble\n\n# === csaw managed start ===\n[mcp_servers.foo]\ncommand = \"x\"\n# === csaw managed end ===\nafter\n")
	start, end, found, err := FindManagedSection(content, testTarget())
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected to find section")
	}
	// Sanity: section text contains both markers and the inner table.
	section := string(content[start:end])
	for _, must := range []string{"# === csaw managed start ===", "# === csaw managed end ===", "[mcp_servers.foo]"} {
		if !strings.Contains(section, must) {
			t.Errorf("section %q missing %q", section, must)
		}
	}
	// Preamble must NOT be included.
	if strings.Contains(section, "preamble") {
		t.Errorf("section %q wrongly includes preamble", section)
	}
	// "after" must not be included.
	if strings.Contains(section, "after") {
		t.Errorf("section %q wrongly includes after-marker content", section)
	}
}

func TestFindManagedSectionMissingEndMarkerErrors(t *testing.T) {
	content := []byte("preamble\n# === csaw managed start ===\nbroken without end marker\n")
	_, _, _, err := FindManagedSection(content, testTarget())
	if err == nil {
		t.Fatal("expected error for orphan start marker; got nil")
	}
	if !strings.Contains(err.Error(), "no matching end marker") {
		t.Errorf("error %q should mention missing end marker", err.Error())
	}
}

func TestRenderManagedSectionStableShape(t *testing.T) {
	servers := []MCPServer{
		{Name: "github", RawTOML: "[mcp_servers.github]\ncommand = \"npx\"\nargs = [\"-y\", \"@modelcontextprotocol/server-github\"]\n"},
		{Name: "linear", RawTOML: "[mcp_servers.linear]\ncommand = \"npx\"\nargs = [\"-y\", \"@tacticiq/linear-mcp\"]\n"},
	}
	out := string(RenderManagedSection(servers, testTarget(), "team"))
	for _, must := range []string{
		"# === csaw managed start ===",
		"# === csaw managed end ===",
		"[mcp_servers.github]",
		"[mcp_servers.linear]",
		"Source: team",
	} {
		if !strings.Contains(out, must) {
			t.Errorf("rendered output missing %q.\nOutput:\n%s", must, out)
		}
	}
	if !strings.HasPrefix(out, "# === csaw managed start ===") {
		t.Errorf("output should start with the start marker (no leading whitespace); composeNewContent owns separator spacing.\nGot prefix: %q", out[:min(60, len(out))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
