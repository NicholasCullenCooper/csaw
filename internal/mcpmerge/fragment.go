package mcpmerge

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Fragment is a parsed source MCP fragment file (e.g., <source>/mcp/codex.toml).
// Holds the per-server name + raw text representation so csaw can write
// each server's TOML block byte-for-byte into the target's managed section,
// preserving any comments or formatting the source author chose.
type Fragment struct {
	// SourceName is the csaw source the fragment came from, for provenance
	// in conflict reports and state manifests.
	SourceName string
	// SourcePath is the absolute path to the fragment file on disk.
	SourcePath string
	// Target is the merge target this fragment is intended for.
	Target MergeTarget
	// Servers are the MCP server entries in the order they appeared.
	Servers []MCPServer
}

// MCPServer is one [mcp_servers.<name>] entry from a fragment.
type MCPServer struct {
	// Name is the table key — appears in [mcp_servers.<Name>].
	Name string
	// RawTOML is the verbatim text of the entry as it appeared in the
	// fragment, including the [mcp_servers.NAME] header line. Used to
	// render the managed section in the target file without re-emitting
	// (which would lose comments and formatting).
	RawTOML string
}

// ReadFragment loads a source MCP fragment file for the given target. It:
//  1. Validates the file parses as the expected format (TOML for Codex).
//  2. Extracts the [mcp_servers.<name>] entries.
//  3. Validates entries contain no literal secrets in sensitive-named fields.
//  4. Captures each entry's raw text for byte-faithful re-emission.
//
// Returns a populated Fragment or a descriptive error.
func ReadFragment(target MergeTarget, sourcePath, sourceName string) (Fragment, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return Fragment{}, fmt.Errorf("read fragment %s: %w", sourcePath, err)
	}

	if target.Format != FormatTOML {
		return Fragment{}, fmt.Errorf("ReadFragment for format %s not implemented yet (MVP is TOML/Codex only)", target.Format)
	}

	// Step 1: validate it parses as TOML and has the expected shape.
	var parsed map[string]interface{}
	if err := toml.Unmarshal(content, &parsed); err != nil {
		return Fragment{}, fmt.Errorf("parse %s as TOML: %w", sourcePath, err)
	}

	mcpServersRaw, ok := parsed["mcp_servers"]
	if !ok {
		return Fragment{}, fmt.Errorf("%s: no [mcp_servers.*] tables found (fragment must contain at least one MCP server)", sourcePath)
	}
	mcpServers, ok := mcpServersRaw.(map[string]interface{})
	if !ok {
		return Fragment{}, fmt.Errorf("%s: mcp_servers is not a table", sourcePath)
	}

	// Step 2 + 3: validate each server's body for literal secrets in
	// sensitive-named fields (schema enforcement — see ValidateNoLiteralSecrets).
	for name, raw := range mcpServers {
		body, ok := raw.(map[string]interface{})
		if !ok {
			return Fragment{}, fmt.Errorf("%s: [mcp_servers.%s] is not a table", sourcePath, name)
		}
		if err := ValidateNoLiteralSecrets(name, body); err != nil {
			return Fragment{}, fmt.Errorf("%s: %w", sourcePath, err)
		}
	}

	// Step 4: scan the file text to extract each entry's raw TOML chunk.
	// We need the original formatting (comments, ordering, quoting) so we
	// can write the managed section byte-for-byte rather than re-emitting.
	chunks, err := extractMCPServerChunks(content)
	if err != nil {
		return Fragment{}, fmt.Errorf("extract %s: %w", sourcePath, err)
	}

	// Cross-check: every parsed name must have a chunk and vice versa.
	for name := range mcpServers {
		if _, ok := chunks[name]; !ok {
			return Fragment{}, fmt.Errorf("%s: parsed [mcp_servers.%s] but couldn't extract raw text — fragment may use a syntax form csaw can't handle yet", sourcePath, name)
		}
	}

	servers := make([]MCPServer, 0, len(chunks))
	for name, raw := range chunks {
		servers = append(servers, MCPServer{Name: name, RawTOML: raw})
	}
	// Stable order — by server name — so dry-run and apply output is consistent.
	sortServersByName(servers)

	return Fragment{
		SourceName: sourceName,
		SourcePath: sourcePath,
		Target:     target,
		Servers:    servers,
	}, nil
}

// extractMCPServerChunks scans TOML text and returns name → raw-text for
// every [mcp_servers.<name>] table. The raw text includes the header line
// and continues until the next top-level table header or EOF. Inline
// `[mcp_servers]` parent declarations and `[other.section]` headers
// terminate a chunk.
func extractMCPServerChunks(content []byte) (map[string]string, error) {
	chunks := make(map[string]string)
	headerRe := regexp.MustCompile(`^\[mcp_servers\.([^\]]+)\]\s*$`)
	anyHeaderRe := regexp.MustCompile(`^\[[^\]]+\]\s*$`)

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	// Allow up to 1MB per line (very generous; config files don't exceed this)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentName string
	var currentChunk strings.Builder

	flush := func() {
		if currentName != "" && currentChunk.Len() > 0 {
			// Trim trailing blank lines for clean rendering later.
			text := strings.TrimRight(currentChunk.String(), " \t\n") + "\n"
			chunks[currentName] = text
		}
		currentName = ""
		currentChunk.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if match := headerRe.FindStringSubmatch(line); match != nil {
			// Starting a new mcp_servers.<name> chunk.
			flush()
			currentName = match[1]
			currentChunk.WriteString(line + "\n")
			continue
		}
		if currentName != "" {
			if anyHeaderRe.MatchString(line) {
				// Another top-level header ends the current chunk.
				flush()
				continue
			}
			currentChunk.WriteString(line + "\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()

	return chunks, nil
}

func sortServersByName(servers []MCPServer) {
	// Simple insertion sort — fragments have few entries; no need for sort.Slice.
	for i := 1; i < len(servers); i++ {
		for j := i; j > 0 && servers[j-1].Name > servers[j].Name; j-- {
			servers[j-1], servers[j] = servers[j], servers[j-1]
		}
	}
}
