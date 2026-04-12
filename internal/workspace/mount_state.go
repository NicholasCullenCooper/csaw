package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

const (
	mountStateFileName   = "mount-state.json"
	restoreStateFileName = "restore-state.json"
)

type MountedStateEntry struct {
	RelativePath string `json:"relative_path"`
	SourceName   string `json:"source_name"`
	SourcePath   string `json:"source_path"`
}

type MountState struct {
	Entries []MountedStateEntry `json:"entries"`
}

func mountStatePath(projectRoot string) string {
	return filepath.Join(StashDir(projectRoot), mountStateFileName)
}

func restoreStatePath(paths runtime.Paths, projectRoot string) string {
	sum := sha256.Sum256([]byte(runtime.NormalizeFSPath(projectRoot)))
	return filepath.Join(paths.State, hex.EncodeToString(sum[:])+"-"+restoreStateFileName)
}

func ReadMountState(projectRoot string) (MountState, error) {
	return readStateFile(mountStatePath(projectRoot))
}

func WriteMountState(projectRoot string, state MountState) error {
	if len(state.Entries) == 0 {
		if err := os.Remove(mountStatePath(projectRoot)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(StashDir(projectRoot), 0o755); err != nil {
		return err
	}
	return writeStateFile(mountStatePath(projectRoot), state)
}

func ReadRestoreState(paths runtime.Paths, projectRoot string) (MountState, error) {
	return readStateFile(restoreStatePath(paths, projectRoot))
}

func WriteRestoreState(paths runtime.Paths, projectRoot string, state MountState) error {
	if len(state.Entries) == 0 {
		if err := os.Remove(restoreStatePath(paths, projectRoot)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(paths.State, 0o755); err != nil {
		return err
	}
	return writeStateFile(restoreStatePath(paths, projectRoot), state)
}

func readStateFile(path string) (MountState, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MountState{}, nil
		}
		return MountState{}, err
	}

	var state MountState
	if err := json.Unmarshal(content, &state); err != nil {
		return MountState{}, err
	}
	return state, nil
}

func writeStateFile(path string, state MountState) error {
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}

func UpsertMountedEntry(state MountState, entry MountedStateEntry) MountState {
	for index, existing := range state.Entries {
		if existing.RelativePath == entry.RelativePath {
			state.Entries[index] = entry
			return state
		}
	}
	state.Entries = append(state.Entries, entry)
	return state
}

func RemoveMountedEntries(state MountState, paths []string) MountState {
	if len(paths) == 0 {
		return state
	}

	lookup := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		lookup[runtime.NormalizeRegistryPath(path)] = struct{}{}
	}

	filtered := state.Entries[:0]
	for _, entry := range state.Entries {
		if _, ok := lookup[runtime.NormalizeRegistryPath(entry.RelativePath)]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	state.Entries = filtered
	return state
}
