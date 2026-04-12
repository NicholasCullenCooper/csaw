package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

type ManifestEntry struct {
	SourcePath string `json:"source_path,omitempty"`
	StashedAt  string `json:"stashed_at"`
	Size       int64  `json:"size"`
}

type Manifest map[string]ManifestEntry

type StateStore interface {
	ReadManifest(projectRoot string) (Manifest, error)
	WriteManifest(projectRoot string, manifest Manifest) error
}

type FileStateStore struct{}

type MountedLink struct {
	RelativePath   string
	FullPath       string
	ActualTarget   string
	ResolvedTarget string
}

func StashDir(projectRoot string) string {
	return filepath.Join(projectRoot, runtime.StashDirName)
}

func ManifestPath(projectRoot string) string {
	return filepath.Join(StashDir(projectRoot), runtime.ManifestName)
}

func (FileStateStore) ReadManifest(projectRoot string) (Manifest, error) {
	content, err := os.ReadFile(ManifestPath(projectRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, nil
		}
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

func (store FileStateStore) WriteManifest(projectRoot string, manifest Manifest) error {
	if err := os.MkdirAll(StashDir(projectRoot), 0o755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	content = append(content, '\n')
	return os.WriteFile(ManifestPath(projectRoot), content, 0o644)
}

func StashFile(store StateStore, projectRoot string, relativePath string, sourcePath string) error {
	absolutePath := filepath.Join(projectRoot, relativePath)
	info, err := os.Stat(absolutePath)
	if err != nil {
		return err
	}

	stashPath := filepath.Join(StashDir(projectRoot), relativePath)
	if err := os.MkdirAll(filepath.Dir(stashPath), 0o755); err != nil {
		return err
	}

	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(stashPath, content, 0o644); err != nil {
		return err
	}

	manifest, err := store.ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	manifest[relativePath] = ManifestEntry{
		SourcePath: sourcePath,
		StashedAt:  time.Now().UTC().Format(time.RFC3339),
		Size:       info.Size(),
	}
	return store.WriteManifest(projectRoot, manifest)
}

func RestoreFile(store StateStore, projectRoot string, relativePath string) (bool, error) {
	stashPath := filepath.Join(StashDir(projectRoot), relativePath)
	content, err := os.ReadFile(stashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	absolutePath := filepath.Join(projectRoot, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(absolutePath, content, 0o644); err != nil {
		return false, err
	}

	manifest, err := store.ReadManifest(projectRoot)
	if err != nil {
		return false, err
	}
	delete(manifest, relativePath)
	if err := store.WriteManifest(projectRoot, manifest); err != nil {
		return false, err
	}

	return true, nil
}

func CleanupStash(store StateStore, projectRoot string) error {
	manifest, err := store.ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	if len(manifest) != 0 {
		return nil
	}

	if state, err := ReadMountState(projectRoot); err == nil && len(state.Entries) != 0 {
		return nil
	} else if err != nil {
		return err
	}

	return os.RemoveAll(StashDir(projectRoot))
}

func ReadExclude(projectRoot string) ([]string, error) {
	path := filepath.Join(projectRoot, ".git", "info", "exclude")
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(runtime.StripBOM(string(content)), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func WriteExclude(projectRoot string, lines []string) error {
	path := filepath.Join(projectRoot, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func AddExclusion(projectRoot string, filename string) (bool, error) {
	lines, err := ReadExclude(projectRoot)
	if err != nil {
		return false, err
	}

	trimmed := runtime.NormalizeRegistryPath(filename)
	for _, line := range lines {
		if strings.TrimSpace(line) == trimmed {
			return false, nil
		}
	}

	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, runtime.ManagedComment, trimmed)
	return true, WriteExclude(projectRoot, lines)
}

func RemoveExclusion(projectRoot string, filename string) (bool, error) {
	lines, err := ReadExclude(projectRoot)
	if err != nil {
		return false, err
	}

	trimmed := runtime.NormalizeRegistryPath(filename)
	filtered := make([]string, 0, len(lines))
	removed := false

	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == runtime.ManagedComment && index+1 < len(lines) && strings.TrimSpace(lines[index+1]) == trimmed {
			index++
			removed = true
			continue
		}
		filtered = append(filtered, lines[index])
	}

	if !removed {
		return false, nil
	}

	return true, WriteExclude(projectRoot, filtered)
}

func FindMountedLinks(root string, csawRoot string) ([]MountedLink, error) {
	var links []MountedLink

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if d.IsDir() && (name == ".git" || name == runtime.StashDirName || name == "node_modules") {
			return filepath.SkipDir
		}
		if runtime.IsNoiseFile(name) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}

		actualTarget, err := os.Readlink(path)
		if err != nil {
			return err
		}

		resolved := actualTarget
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(path), resolved)
		}
		resolved = runtime.NormalizeFSPath(resolved)
		if !runtime.PathStartsWith(resolved, csawRoot) {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		links = append(links, MountedLink{
			RelativePath:   runtime.NormalizeRegistryPath(relativePath),
			FullPath:       path,
			ActualTarget:   actualTarget,
			ResolvedTarget: resolved,
		})
		return nil
	})

	return links, err
}
