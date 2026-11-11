// Package scan runs cheap, deterministic content checks over a directory
// of vendored agent context. It is not a prompt-injection scanner: it
// flags the mechanical things a reviewer should look at before promoting
// a file (invisible characters, tool grants, embedded hooks, command
// substitution, executables, symlinks) and leaves judgement, or a real
// scanner run through RunExternal, to the user.
package scan

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Finding is one thing worth a look in one file.
type Finding struct {
	Path   string `yaml:"path"`
	Check  string `yaml:"check"`
	Detail string `yaml:"detail"`
}

const (
	CheckHiddenUnicode  = "hidden-unicode"
	CheckToolGrant      = "skill-tool-grant"
	CheckEmbeddedHooks  = "skill-hooks"
	CheckCommandSubst   = "command-substitution"
	CheckExecutable     = "executable-outside-scripts"
	CheckSymlink        = "symlink"
	CheckBinaryInSkill  = "binary-in-skill"
	maxScannedFileBytes = 4 << 20
)

// hiddenRunes are invisible or bidirectional-control characters that can
// hide text from a human reviewer while an agent still reads it.
var hiddenRunes = map[rune]string{
	'\u00AD': "soft hyphen",
	'\u200B': "zero width space", '\u200C': "zero width non-joiner", '\u200D': "zero width joiner",
	'\u200E': "left-to-right mark", '\u200F': "right-to-left mark",
	'\u2028': "line separator", '\u2029': "paragraph separator",
	'\u202A': "LRE", '\u202B': "RLE", '\u202C': "PDF", '\u202D': "LRO", '\u202E': "RLO",
	'\u2060': "word joiner", '\u2061': "function application", '\u2062': "invisible times",
	'\u2063': "invisible separator", '\u2064': "invisible plus",
	'\u2066': "LRI", '\u2067': "RLI", '\u2068': "FSI", '\u2069': "PDI",
	'\ufeff':     "byte order mark (not at file start)",
	'\U000E0001': "language tag", // tag characters U+E0000..E007F are all covered below
}

var commandSubst = regexp.MustCompile("!`[^`]+`")

// Dir scans every file under root and returns findings sorted by path.
func Dir(root string) ([]Finding, error) {
	var findings []Finding
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(filepath.Base(rel), ".csaw-") {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			findings = append(findings, Finding{Path: rel, Check: CheckSymlink, Detail: "symlink to " + target + "; vendored content should be plain files"})
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Mode().Perm()&0o111 != 0 && !underScripts(rel) {
			findings = append(findings, Finding{Path: rel, Check: CheckExecutable, Detail: "executable bit set outside a scripts/ directory"})
		}
		if info.Size() > maxScannedFileBytes {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings = append(findings, File(rel, content)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Check < findings[j].Check
	})
	return findings, nil
}

// File runs the content checks on one file's bytes.
func File(rel string, content []byte) []Finding {
	var findings []Finding
	isText := utf8.Valid(content) && !bytes.Contains(content, []byte{0})
	base := filepath.Base(rel)
	isMarkdown := strings.HasSuffix(base, ".md")

	if !isText {
		if strings.Contains(rel, "/skills/") || strings.HasPrefix(rel, "skills/") {
			findings = append(findings, Finding{Path: rel, Check: CheckBinaryInSkill, Detail: "binary file inside a skill directory"})
		}
		return findings
	}

	if detail := hiddenUnicode(content); detail != "" {
		findings = append(findings, Finding{Path: rel, Check: CheckHiddenUnicode, Detail: detail})
	}

	if isMarkdown {
		if matches := commandSubst.FindAll(content, 3); len(matches) > 0 {
			findings = append(findings, Finding{Path: rel, Check: CheckCommandSubst, Detail: fmt.Sprintf("dynamic command substitution %s runs a shell command when the file is loaded", string(matches[0]))})
		}
	}

	if base == "SKILL.md" {
		front := frontmatter(content)
		if hasKey(front, "allowed-tools") {
			findings = append(findings, Finding{Path: rel, Check: CheckToolGrant, Detail: "frontmatter grants tools with allowed-tools; review before promoting"})
		}
		if hasKey(front, "hooks") {
			findings = append(findings, Finding{Path: rel, Check: CheckEmbeddedHooks, Detail: "frontmatter embeds hooks that run for the rest of the session once invoked"})
		}
	}
	return findings
}

func underScripts(rel string) bool {
	for _, segment := range strings.Split(rel, "/") {
		if segment == "scripts" || segment == "bin" {
			return true
		}
	}
	return false
}

func hiddenUnicode(content []byte) string {
	counts := map[string]int{}
	offset := 0
	for offset < len(content) {
		r, size := utf8.DecodeRune(content[offset:])
		if r == '\ufeff' && offset == 0 {
			offset += size
			continue
		}
		if name, ok := hiddenRunes[r]; ok {
			counts[name]++
		} else if r >= 0xE0000 && r <= 0xE007F {
			counts["tag character"]++
		}
		offset += size
	}
	if len(counts) == 0 {
		return ""
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s ×%d", name, counts[name]))
	}
	return "invisible or bidirectional characters: " + strings.Join(parts, ", ")
}

func frontmatter(content []byte) []string {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimPrefix(lines[0], "\ufeff")) != "---" {
		return nil
	}
	var out []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		out = append(out, line)
	}
	return out
}

func hasKey(front []string, key string) bool {
	for _, line := range front {
		if strings.HasPrefix(strings.TrimSpace(line), key+":") && !strings.HasPrefix(line, " ") {
			return true
		}
	}
	return false
}

// RunExternal runs a user-configured scanner command over dir. The command
// line is split on whitespace; a "{dir}" token is replaced by dir, and dir
// is appended when the token is absent. Combined output goes to logPath.
// status is "passed" or "failed (exit N)"; err reports only csaw's own
// failure to run the command.
func RunExternal(commandLine string, dir string, logPath string) (status string, err error) {
	fields := strings.Fields(commandLine)
	if len(fields) == 0 {
		return "", fmt.Errorf("scanner command is empty")
	}
	args := fields[1:]
	replaced := false
	for i, arg := range args {
		if arg == "{dir}" {
			args[i] = dir
			replaced = true
		}
	}
	if !replaced {
		args = append(args, dir)
	}
	cmd := exec.Command(fields[0], args...)
	cmd.Dir = dir
	output, runErr := cmd.CombinedOutput()
	if logPath != "" {
		if mkErr := os.MkdirAll(filepath.Dir(logPath), 0o755); mkErr == nil {
			_ = os.WriteFile(logPath, output, 0o644)
		}
	}
	if runErr == nil {
		return "passed", nil
	}
	var exitErr *exec.ExitError
	if ok := asExitError(runErr, &exitErr); ok {
		return fmt.Sprintf("failed (exit %d)", exitErr.ExitCode()), nil
	}
	return "", fmt.Errorf("run scanner %q: %w", fields[0], runErr)
}

func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		*target = exitErr
	}
	return ok
}
