package mcpmerge

import (
	"bytes"
	"fmt"
	"strings"
)

// FindManagedSection locates an existing csaw-managed section in target file
// content. The section bytes start at the StartMarker line and end after
// the EndMarker line's trailing newline — separator whitespace before or
// after the section is NOT included (callers handle that separately).
//
// Returns (0, 0, false, nil) when no markers are present. Returns an
// error if a start marker is found but no end marker — the file has been
// corrupted (user edited the markers); caller should refuse to overwrite.
//
// The strict "section is markers-only" definition is load-bearing: the
// manifest's SHA is computed on the same byte range, so Render → Write →
// Find → SHA must round-trip exactly.
func FindManagedSection(content []byte, target MergeTarget) (start, end int, found bool, err error) {
	startBytes := []byte(target.StartMarker)
	endBytes := []byte(target.EndMarker)

	startIdx := bytes.Index(content, startBytes)
	if startIdx < 0 {
		return 0, 0, false, nil
	}

	// Find the end marker after the start marker.
	endIdx := bytes.Index(content[startIdx:], endBytes)
	if endIdx < 0 {
		return 0, 0, false, fmt.Errorf("found start marker but no matching end marker — target file may have been corrupted; refusing to write")
	}
	endIdx += startIdx

	// Extend `end` to include the end-marker line's trailing newline if present.
	endLineEnd := endIdx + len(endBytes)
	if endLineEnd < len(content) && content[endLineEnd] == '\n' {
		endLineEnd++
	}

	return startIdx, endLineEnd, true, nil
}

// RenderManagedSection produces the bounded-section text to write into a
// target file. Each MCP server's RawTOML is included verbatim (preserving
// source-author formatting and comments), separated by blank lines. The
// section is prefixed by a leading blank line for visual separation from
// preceding user content.
//
// Output shape:
//
//	<blank line>
//	# === csaw managed start ... ===
//	# Managed by csaw — generated from <source>/mcp/<fragment> on <timestamp omitted for reproducibility>
//	# Source servers: <count>
//
//	[mcp_servers.<name1>]
//	... server 1 body ...
//
//	[mcp_servers.<name2>]
//	... server 2 body ...
//
//	# === csaw managed end ===
func RenderManagedSection(servers []MCPServer, target MergeTarget, sourceName string) []byte {
	var b strings.Builder
	b.WriteString(target.StartMarker)
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("# Source: %s · %d server(s) · regenerate: csaw mcp sync %s --apply · remove: csaw mcp sync %s --remove\n",
		sourceName, len(servers), target.Name, target.Name))
	b.WriteByte('\n')

	for i, s := range servers {
		b.WriteString(s.RawTOML)
		// RawTOML already ends with \n; add a blank separator line between
		// servers (but not after the last).
		if i < len(servers)-1 {
			b.WriteByte('\n')
		}
	}

	b.WriteString(target.EndMarker)
	b.WriteByte('\n')
	return []byte(b.String())
}
