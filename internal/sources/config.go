package sources

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/csaw-ai/csaw/internal/git"
	"github.com/csaw-ai/csaw/internal/runtime"
)

const (
	KindRemote = "remote"
	KindLocal  = "local"
)

type Source struct {
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"`
	URL      string `yaml:"url,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
}

type Config struct {
	Sources           []Source `yaml:"sources,omitempty"`
	DefaultForkTarget string   `yaml:"default_fork_target,omitempty"`
	Tools             []string `yaml:"tools,omitempty"`
}

type Manager struct {
	Paths runtime.Paths
	Git   git.Git
}

func NewSource(name string, location string) (Source, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Source{}, errors.New("source name is required")
	}

	if strings.ContainsAny(name, `/\ `) {
		return Source{}, fmt.Errorf("invalid source name %q", name)
	}

	location = strings.TrimSpace(location)
	if location == "" {
		return Source{}, errors.New("source location is required")
	}

	if isLocalPath(location) {
		location, err := expandUserHome(location)
		if err != nil {
			return Source{}, err
		}
		abs, err := filepath.Abs(location)
		if err != nil {
			return Source{}, err
		}
		return Source{Name: name, Kind: KindLocal, Path: abs}, nil
	}

	return Source{Name: name, Kind: KindRemote, URL: location}, nil
}

func (s Source) CheckoutPath(paths runtime.Paths) string {
	if s.Kind == KindLocal && s.Path != "" {
		return s.Path
	}

	return filepath.Join(paths.Sources, s.Name)
}

func (m Manager) EnsureDirectories() error {
	for _, dir := range []string{m.Paths.Root, m.Paths.Sources, m.Paths.Contexts, m.Paths.State} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) Load() (Config, error) {
	if err := m.EnsureDirectories(); err != nil {
		return Config{}, err
	}

	content, err := os.ReadFile(m.Paths.Config)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(runtime.StripBOM(string(content))), &cfg); err != nil {
		return Config{}, err
	}

	sort.Slice(cfg.Sources, func(i, j int) bool { return cfg.Sources[i].Name < cfg.Sources[j].Name })
	return cfg, nil
}

func (m Manager) Save(cfg Config) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}

	sort.Slice(cfg.Sources, func(i, j int) bool { return cfg.Sources[i].Name < cfg.Sources[j].Name })

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(m.Paths.Config, content, 0o644)
}

func (m Manager) Add(source Source) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}

	for _, existing := range cfg.Sources {
		if existing.Name == source.Name {
			return fmt.Errorf("source %q already exists", source.Name)
		}
	}

	cfg.Sources = append(cfg.Sources, source)
	return m.Save(cfg)
}

func (m Manager) Remove(name string) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}

	filtered := cfg.Sources[:0]
	removed := false
	for _, source := range cfg.Sources {
		if source.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, source)
	}

	if !removed {
		return fmt.Errorf("source %q not found", name)
	}

	cfg.Sources = filtered
	return m.Save(cfg)
}

func (m Manager) Get(name string) (Source, error) {
	cfg, err := m.Load()
	if err != nil {
		return Source{}, err
	}

	for _, source := range cfg.Sources {
		if source.Name == name {
			return source, nil
		}
	}

	return Source{}, fmt.Errorf("source %q not found", name)
}

func (m Manager) Pull(ctx context.Context, name string) error {
	source, err := m.Get(name)
	if err != nil {
		return err
	}

	if source.Kind == KindLocal {
		return nil
	}

	checkout := source.CheckoutPath(m.Paths)
	if _, err := os.Stat(checkout); errors.Is(err, os.ErrNotExist) {
		if _, err := m.Git.Run(ctx, m.Paths.Sources, "clone", source.URL, checkout); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	_, err = m.Git.Run(ctx, checkout, "pull", "--ff-only")
	return err
}

func (m Manager) PullAll(ctx context.Context) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}

	for _, source := range cfg.Sources {
		if err := m.Pull(ctx, source.Name); err != nil {
			return err
		}
	}

	return nil
}

func isLocalPath(value string) bool {
	if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~") {
		return true
	}
	// Windows absolute paths like C:\ or D:/
	if len(value) >= 3 && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	_, err := os.Stat(value)
	return err == nil
}

func expandUserHome(value string) (string, error) {
	if value == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(value, "~/")), nil
	}
	return value, nil
}
