package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrNothingToPush = errors.New("nothing to push")

type CatalogSource struct {
	Name     string
	Kind     string
	Root     string
	Priority int
}

func (m Manager) Catalog() ([]CatalogSource, error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, err
	}

	catalog := make([]CatalogSource, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		catalog = append(catalog, CatalogSource{
			Name:     source.Name,
			Kind:     source.Kind,
			Root:     source.CheckoutPath(m.Paths),
			Priority: source.Priority,
		})
	}

	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Name < catalog[j].Name })
	return catalog, nil
}

func (m Manager) Push(ctx context.Context, name string, message string) error {
	source, err := m.Get(name)
	if err != nil {
		return err
	}

	root := source.CheckoutPath(m.Paths)
	if message == "" {
		message = "csaw: update " + name
	}

	gitDir := filepath.Join(root, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("source %q is not a git repository: %s", name, root)
		}
		return err
	}

	status, err := m.Git.Run(ctx, root, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return ErrNothingToPush
	}

	if _, err := m.Git.Run(ctx, root, "add", "-A"); err != nil {
		return err
	}
	if _, err := m.Git.Run(ctx, root, "commit", "-m", message); err != nil {
		return err
	}
	_, err = m.Git.Run(ctx, root, "push")
	return err
}

// WorktreeCheckout creates or updates a git worktree for a pinned source ref.
// It returns the worktree path which should be used as the source root.
func (m Manager) WorktreeCheckout(ctx context.Context, source Source, ref string, projectRoot string) (string, error) {
	mainCheckout := source.CheckoutPath(m.Paths)
	worktreePath := m.WorktreePath(source, projectRoot)

	checkoutRef := ref
	if _, err := m.Git.Run(ctx, mainCheckout, "fetch", "origin", ref); err == nil {
		if resolved, err := m.Git.Run(ctx, mainCheckout, "rev-parse", "FETCH_HEAD"); err == nil {
			if resolved = strings.TrimSpace(resolved); resolved != "" {
				checkoutRef = resolved
			}
		}
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
			return "", err
		}
		if _, err := m.Git.Run(ctx, mainCheckout, "worktree", "add", "--detach", worktreePath, checkoutRef); err != nil {
			return "", fmt.Errorf("failed to create worktree for %s@%s: %w", source.Name, ref, err)
		}
	} else {
		if _, err := m.Git.Run(ctx, worktreePath, "checkout", "--detach", checkoutRef); err != nil {
			return "", fmt.Errorf("failed to checkout %s in worktree: %w", ref, err)
		}
	}

	return worktreePath, nil
}

func (m Manager) WorktreePath(source Source, projectRoot string) string {
	mainCheckout := source.CheckoutPath(m.Paths)
	sum := sha256.Sum256([]byte(projectRoot))
	projectID := hex.EncodeToString(sum[:])[:12]
	return filepath.Join(mainCheckout, ".worktrees", projectID)
}

// WorktreeRemove cleans up a worktree for a previously pinned source.
func (m Manager) WorktreeRemove(ctx context.Context, source Source, projectRoot string) error {
	mainCheckout := source.CheckoutPath(m.Paths)
	worktreePath := m.WorktreePath(source, projectRoot)

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil
	}

	_, err := m.Git.Run(ctx, mainCheckout, "worktree", "remove", worktreePath)
	return err
}

func (m Manager) ExistingCatalog() ([]CatalogSource, error) {
	catalog, err := m.Catalog()
	if err != nil {
		return nil, err
	}

	filtered := catalog[:0]
	for _, source := range catalog {
		info, err := os.Stat(source.Root)
		if err != nil || !info.IsDir() {
			continue
		}
		filtered = append(filtered, source)
	}

	return filtered, nil
}
