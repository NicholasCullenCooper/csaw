package workspace

import (
	"bytes"
	"os/exec"
	"strings"
)

// IsGitIgnored checks whether a path is already ignored by .gitignore
// (not .git/info/exclude — we want to know if the path is covered by
// the project's committed ignore rules). Returns false on any error
// (e.g., not a git repo) so callers default to adding an exclude entry.
func IsGitIgnored(projectRoot string, relativePath string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", relativePath)
	cmd.Dir = projectRoot

	// git check-ignore -q exits 0 if ignored, 1 if not ignored.
	// We need to distinguish "ignored by .gitignore" from "ignored by
	// .git/info/exclude" — but git check-ignore doesn't differentiate.
	// Use --no-index to skip .git/info/exclude... except that flag
	// doesn't exist. Instead, check with -v and parse the source.
	cmdVerbose := exec.Command("git", "check-ignore", "-v", relativePath)
	cmdVerbose.Dir = projectRoot

	var stdout bytes.Buffer
	cmdVerbose.Stdout = &stdout

	if err := cmdVerbose.Run(); err != nil {
		// Exit code 1 = not ignored. Any other error = treat as not ignored.
		return false
	}

	// Output format: "<source>:<linenum>:<pattern>\t<pathname>"
	// If source is ".git/info/exclude", it's OUR exclude — don't count it.
	// If source is ".gitignore" or any other file, it's project-level ignore.
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return false
	}

	// Parse source from the verbose output
	colonIdx := strings.Index(output, ":")
	if colonIdx < 0 {
		return false
	}
	source := output[:colonIdx]

	// If the ignore comes from .git/info/exclude, that's csaw's own exclude.
	// We want to know if something ELSE covers this path.
	if strings.HasSuffix(source, ".git/info/exclude") {
		return false
	}

	return true
}

// GitIgnoreSource returns which file and pattern is causing a path to be
// ignored, for use in user-facing messages. Returns empty strings if the
// path is not ignored or on error.
func GitIgnoreSource(projectRoot string, relativePath string) (file string, pattern string) {
	cmd := exec.Command("git", "check-ignore", "-v", relativePath)
	cmd.Dir = projectRoot

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", ""
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", ""
	}

	// Format: "<source>:<linenum>:<pattern>\t<pathname>"
	tabIdx := strings.Index(output, "\t")
	if tabIdx < 0 {
		return "", ""
	}
	prefix := output[:tabIdx]

	parts := strings.SplitN(prefix, ":", 3)
	if len(parts) < 3 {
		return "", ""
	}

	return parts[0], strings.TrimSpace(parts[2])
}
