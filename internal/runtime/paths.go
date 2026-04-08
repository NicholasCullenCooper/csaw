package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DirName        = ".csaw"
	SourcesDirName = "sources"
	ContextsDirName = "contexts"
	StateDirName    = "state"
	ConfigFileName  = "config.yml"
	ProfilesFile    = "csaw.yml"
	IgnoreFile      = ".csawignore"
	StashDirName    = ".csaw-stash"
	ManifestName    = "manifest.json"
	ManagedComment  = "# csaw-managed"
)

var noiseFiles = map[string]struct{}{
	".DS_Store":   {},
	"Thumbs.db":   {},
	"desktop.ini": {},
}

type Paths struct {
	Root     string
	Sources  string
	Contexts string
	State    string
	Config   string
}

type PathNormalizer interface {
	Normalize(string) string
	Equal(string, string) bool
	StartsWith(string, string) bool
}

type DefaultNormalizer struct{}

func ResolvePaths() (Paths, error) {
	if root := os.Getenv("CSAW_HOME"); root != "" {
		return BuildPaths(root), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	return BuildPaths(filepath.Join(home, DirName)), nil
}

func BuildPaths(root string) Paths {
	root = filepath.Clean(root)
	return Paths{
		Root:     root,
		Sources:  filepath.Join(root, SourcesDirName),
		Contexts: filepath.Join(root, ContextsDirName),
		State:    filepath.Join(root, StateDirName),
		Config:   filepath.Join(root, ConfigFileName),
	}
}

func (DefaultNormalizer) Normalize(value string) string {
	return NormalizeFSPath(value)
}

func (DefaultNormalizer) Equal(left string, right string) bool {
	return PathsEqual(left, right)
}

func (DefaultNormalizer) StartsWith(child string, parent string) bool {
	return PathStartsWith(child, parent)
}

func NormalizeFSPath(value string) string {
	cleaned := strings.TrimPrefix(value, `\\?\`)
	cleaned = filepath.Clean(cleaned)
	if abs, err := filepath.Abs(cleaned); err == nil {
		cleaned = abs
	}
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}

func PathsEqual(left string, right string) bool {
	return NormalizeFSPath(left) == NormalizeFSPath(right)
}

func PathStartsWith(child string, parent string) bool {
	normalizedChild := NormalizeFSPath(child)
	normalizedParent := NormalizeFSPath(parent)
	if normalizedChild == normalizedParent {
		return true
	}

	return strings.HasPrefix(normalizedChild, normalizedParent+string(filepath.Separator))
}

func NormalizeRegistryPath(value string) string {
	value = strings.ReplaceAll(value, `\`, `/`)
	value = strings.TrimPrefix(value, "./")
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	value = strings.TrimRight(value, "/")
	return value
}

func StripBOM(value string) string {
	if value == "" {
		return value
	}
	if []rune(value)[0] == '\uFEFF' {
		return string([]rune(value)[1:])
	}
	return value
}

func IsNoiseFile(name string) bool {
	_, ok := noiseFiles[name]
	return ok
}

func FindRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside a git repository")
		}
		dir = parent
	}
}
