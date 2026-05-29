package sources

import (
	"errors"
	"fmt"
	"strings"
)

// HostShorthand maps a recognized prefix to a base URL template. The template
// must contain "{path}" which is replaced with the org/repo portion.
//
// Only the three biggest hosts ship by default — additional hosts can be
// added here as user demand emerges. Per docs/planning/package-manager-lessons.md,
// resist the temptation to overload the shorthand grammar (no #semver:^X
// style cleverness); the matrix of prefix × ref-kind is already wide enough.
var hostShorthands = map[string]string{
	"gh": "https://github.com/{path}.git",
	"gl": "https://gitlab.com/{path}.git",
	"bb": "https://bitbucket.org/{path}.git",
}

// ParsedShorthand is the canonical result of parsing a source-add shorthand
// like "gh:co/team-source#v1.2.0". URL is always a valid git URL; Ref is the
// extracted git ref (branch, tag, or commit) or empty if none was specified.
type ParsedShorthand struct {
	URL string
	Ref string
}

// IsShorthand reports whether input looks like a host-shorthand (e.g.,
// "gh:org/repo"). It does NOT validate the org/repo portion — call
// ParseShorthand for that.
func IsShorthand(input string) bool {
	input = strings.TrimSpace(input)
	for prefix := range hostShorthands {
		if strings.HasPrefix(input, prefix+":") {
			// Disambiguate against full URLs like "https:..." or "git@host:...".
			// A shorthand prefix is always a known short host alias followed by
			// ":" and a non-slash character (path), never "://" or empty.
			rest := strings.TrimPrefix(input, prefix+":")
			if rest == "" {
				return false
			}
			if strings.HasPrefix(rest, "//") {
				return false
			}
			return true
		}
	}
	return false
}

// ParseShorthand expands a host-shorthand string into a canonical URL and
// optional ref. Recognized forms:
//
//	gh:org/repo            -> https://github.com/org/repo.git
//	gh:org/repo#main       -> URL + Ref="main"
//	gh:org/repo#v1.2.0     -> URL + Ref="v1.2.0"
//	gh:org/repo#abc1234    -> URL + Ref="abc1234" (treated as branch/tag/commit by git)
//
// gl: and bb: behave the same with their respective host base URLs.
//
// Returns an error if the prefix is unknown, the org/repo portion is missing,
// or the input contains characters that aren't valid in a git path.
func ParseShorthand(input string) (ParsedShorthand, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return ParsedShorthand{}, errors.New("shorthand is empty")
	}

	prefix, rest, ok := strings.Cut(input, ":")
	if !ok {
		return ParsedShorthand{}, fmt.Errorf("shorthand %q missing ':' separator", input)
	}

	template, known := hostShorthands[prefix]
	if !known {
		return ParsedShorthand{}, fmt.Errorf("unknown shorthand prefix %q (known: gh, gl, bb)", prefix)
	}

	if rest == "" {
		return ParsedShorthand{}, fmt.Errorf("shorthand %q has no path after prefix", input)
	}

	// Split path from optional #ref
	path, ref, _ := strings.Cut(rest, "#")
	path = strings.TrimSpace(path)
	ref = strings.TrimSpace(ref)

	if path == "" {
		return ParsedShorthand{}, fmt.Errorf("shorthand %q has empty path", input)
	}
	// Must contain at least one "/" — org/repo shape
	if !strings.Contains(path, "/") {
		return ParsedShorthand{}, fmt.Errorf("shorthand %q path must be of the form org/repo", input)
	}
	// Reject obviously invalid characters in path
	if strings.ContainsAny(path, " \t\n\\") {
		return ParsedShorthand{}, fmt.Errorf("shorthand %q path contains invalid characters", input)
	}

	url := strings.Replace(template, "{path}", path, 1)
	return ParsedShorthand{URL: url, Ref: ref}, nil
}
