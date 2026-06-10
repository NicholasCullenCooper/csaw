package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/git"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/vendor"
)

// newVendorCommand wires up `csaw vendor ...` subcommands. The vendor
// feature is csaw's safe-consumer primitive for external agent-context
// catalogs (skills.sh, APM packages, awesome-copilot, internal bundles).
// Vendored content lives under <registry>/vendor/<name>/ with full
// provenance and hashes; nothing under vendor/ ever projects to a
// mounted project until explicitly promoted via `csaw vendor promote`.
//
// See docs/planning/vendors-design.md for the design rationale.
func newVendorCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor external agent-context catalogs into a locked area, audit drift, and promote selected files into csaw sources",
		Long: `csaw vendor is the safe-consumer primitive for external agent-context
catalogs (skills.sh, APM packages, awesome-copilot, internal bundle
manifests, any git repo).

Workflow:
  1. Declare a vendor in your registry's csaw.yml (csaw vendor add ...)
  2. Fetch it into <registry>/vendor/<name>/ with hashes locked
     (csaw vendor sync)
  3. Inspect drift between vendored copy + upstream + promoted files
     (csaw vendor audit)
  4. Copy reviewed files into your real csaw kind directories
     (csaw vendor promote <vendor>/<path> --into <dest>)

Vendored content NEVER projects to a mounted project. Only files copied
into real kind directories (skills/, agents/, rules/, etc.) via promote
or by hand authoring will mount.

Run csaw vendor <subcommand> --help for details.`,
	}
	root.AddCommand(newVendorAddCommand())
	root.AddCommand(newVendorListCommand())
	root.AddCommand(newVendorSyncCommand())
	root.AddCommand(newVendorAuditCommand())
	root.AddCommand(newVendorPromoteCommand())
	root.AddCommand(newVendorRemoveCommand())
	return root
}

// vendorRegistryRoot resolves the working directory as a csaw source
// registry root. Returns the absolute path. Today this defers to the
// current working directory; in the future it could resolve via a source
// catalog lookup.
func vendorRegistryRoot() (string, error) {
	wd, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	return wd, nil
}

func vendorCacheRoot(paths runtime.Paths) string {
	return filepath.Join(paths.State, "vendor-cache")
}

// --- subcommands ---

func newVendorAddCommand() *cobra.Command {
	var (
		ref     string
		include []string
		exclude []string
	)
	cmd := &cobra.Command{
		Use:   "add <name> <url-or-shorthand>",
		Short: "Declare a vendor in the current registry's csaw.yml (does not sync)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			name, raw := args[0], args[1]

			// Normalize shorthand here too so the user sees the canonical
			// URL stored in csaw.yml, not the shorthand.
			url := raw
			extractedRef := ""
			if sources.IsShorthand(raw) {
				parsed, err := sources.ParseShorthand(raw)
				if err != nil {
					return err
				}
				url = parsed.URL
				extractedRef = parsed.Ref
			}
			if ref == "" {
				ref = extractedRef
			}

			decl := vendor.Declaration{
				Name: name, URL: url, Ref: ref,
				Include: include, Exclude: exclude,
			}
			if err := vendor.AddDeclaration(registryRoot, decl); err != nil {
				return err
			}
			output.Successf("declared vendor %q (%s)", name, url)
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", output.Faint("Next: csaw vendor sync "+name))
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref (branch/tag/commit) to track")
	cmd.Flags().StringArrayVar(&include, "include", nil, "glob pattern to include (repeatable; default: all)")
	cmd.Flags().StringArrayVar(&exclude, "exclude", nil, "glob pattern to exclude after include (repeatable)")
	return cmd
}

func newVendorListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List declared vendors and their sync state",
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			decls, err := vendor.LoadDeclarations(registryRoot)
			if err != nil {
				return err
			}
			lf, err := vendor.LoadLockfile(registryRoot)
			if err != nil {
				return err
			}

			if len(decls) == 0 {
				output.Muted("no vendors declared in csaw.yml")
				fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n",
					output.Faint("Declare one: csaw vendor add <name> <git-url-or-gh:org/repo>"))
				return nil
			}

			output.Header("vendors")
			fmt.Fprintln(cmd.OutOrStdout())
			for _, d := range decls {
				refLabel := d.Ref
				if refLabel == "" {
					refLabel = "(default branch)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", d.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "    %s %s @ %s\n", output.Faint("url:"), d.URL, refLabel)
				state, synced := lf.Vendors[d.Name]
				if !synced {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s %s\n", output.Faint("status:"), "not yet synced")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s %d file(s), last sync %s, locked at %s\n",
						output.Faint("status:"),
						len(state.Files),
						timeSince(state.SyncedAt),
						state.RefResolved[:min(12, len(state.RefResolved))])
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newVendorSyncCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync [<name>]",
		Short: "Fetch one or all declared vendors into <registry>/vendor/<name>/",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}
			cacheRoot := vendorCacheRoot(paths)

			decls, err := vendor.LoadDeclarations(registryRoot)
			if err != nil {
				return err
			}
			if len(decls) == 0 {
				return fmt.Errorf("no vendors declared in csaw.yml")
			}

			var targets []vendor.Declaration
			if len(args) == 1 {
				name := args[0]
				for _, d := range decls {
					if d.Name == name {
						targets = []vendor.Declaration{d}
						break
					}
				}
				if len(targets) == 0 {
					return fmt.Errorf("vendor %q not declared in csaw.yml", name)
				}
			} else {
				targets = decls
			}

			ctx := context.Background()
			for _, d := range targets {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s syncing %s...\n", output.Faint("→"), d.Name)
				res, err := vendor.Sync(ctx, git.ExecGit{}, d, registryRoot, cacheRoot, force)
				if err != nil {
					return err
				}
				output.Successf("%s · %d file(s) @ %s", res.Name, res.FilesAdded, res.RefResolved[:min(12, len(res.RefResolved))])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite vendor-local edits if present")
	return cmd
}

func newVendorAuditCommand() *cobra.Command {
	var skipNetwork bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Detect vendor-local, upstream, and promotion drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}
			cacheRoot := vendorCacheRoot(paths)

			var g git.Git = git.ExecGit{}
			if skipNetwork {
				g = nil
			}
			findings, err := vendor.Audit(context.Background(), g, registryRoot, cacheRoot)
			if err != nil {
				return err
			}

			if !findings.HasAny() {
				output.Successf("no vendor drift detected")
				return nil
			}

			if len(findings.LocalDrift) > 0 {
				output.Header("vendor-local drift")
				fmt.Fprintln(cmd.OutOrStdout())
				for _, f := range findings.LocalDrift {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s/%s — %s\n",
						output.Faint("•"), f.Vendor, f.VendorPath, f.Reason)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(findings.UpstreamDrift) > 0 {
				output.Header("upstream drift")
				fmt.Fprintln(cmd.OutOrStdout())
				for _, f := range findings.UpstreamDrift {
					refLabel := f.RefRequested
					if refLabel == "" {
						refLabel = "(default branch)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s @ %s\n", output.Faint("•"), f.Vendor, refLabel)
					if len(f.LocalSHA) >= 12 {
						fmt.Fprintf(cmd.OutOrStdout(), "      local:    %s\n", f.LocalSHA[:12])
					}
					if len(f.UpstreamSHA) >= 12 && !strings.HasPrefix(f.UpstreamSHA, "(") {
						fmt.Fprintf(cmd.OutOrStdout(), "      upstream: %s\n", f.UpstreamSHA[:12])
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "      upstream: %s\n", f.UpstreamSHA)
					}
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(findings.PromotionDrift) > 0 {
				output.Header("promotion drift")
				fmt.Fprintln(cmd.OutOrStdout())
				for _, f := range findings.PromotionDrift {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s (from %s/%s)\n", output.Faint("•"), f.PromotedTo, f.Vendor, f.VendorPath)
					fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", f.Detail)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipNetwork, "no-network", false, "skip upstream-drift check (no git ls-remote calls)")
	return cmd
}

func newVendorPromoteCommand() *cobra.Command {
	var (
		dest  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "promote <vendor>/<vendor-path>",
		Short: "Copy a vendored file into a real csaw kind directory and record lineage",
		Long: `Promote copies <registry>/vendor/<vendor>/<vendor-path> to
<registry>/<dest>, recording the lineage in vendor.lock.yaml.

The vendored copy stays in place — promotion is a copy, not a move.
This preserves the vendor area as the immutable upstream record while
the promoted copy enters active csaw projection through the normal
mount/profile flow.

Examples:
  csaw vendor promote awesome-copilot/agents/code-reviewer.md --into agents/code-reviewer.md
  csaw vendor promote skills-sh-foo/SKILL.md --into skills/foo-skill/SKILL.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			if dest == "" {
				return fmt.Errorf("--into <dest-path> is required")
			}

			vendorName, vendorPath, ok := strings.Cut(args[0], "/")
			if !ok || vendorName == "" || vendorPath == "" {
				return fmt.Errorf("argument must be of the form <vendor>/<vendor-path>")
			}

			res, err := vendor.Promote(registryRoot, vendorName, vendorPath, dest, force)
			if err != nil {
				return err
			}
			verb := "promoted"
			if res.Replaced {
				verb = "promoted (overwrote existing)"
			}
			output.Successf("%s vendor/%s/%s → %s (%d bytes)", verb, res.Vendor, res.VendorPath, res.PromotedTo, res.BytesCopied)
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "into", "", "destination path within the registry (required)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file at the destination")
	return cmd
}

func newVendorRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a vendor from csaw.yml and delete its vendor/<name>/ directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryRoot, err := vendorRegistryRoot()
			if err != nil {
				return err
			}
			name := args[0]

			if err := vendor.RemoveDeclaration(registryRoot, name); err != nil {
				return err
			}

			lf, err := vendor.LoadLockfile(registryRoot)
			if err == nil {
				delete(lf.Vendors, name)
				_ = vendor.SaveLockfile(registryRoot, lf)
			}

			vendorDir := filepath.Join(registryRoot, "vendor", name)
			// Best-effort cleanup; do not fail if it's already gone.
			if err := os.RemoveAll(vendorDir); err != nil {
				output.Warnf("removed declaration but could not delete %s: %v", vendorDir, err)
			}

			output.Successf("removed vendor %q", name)
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", output.Faint("Note: any promoted files remain in place (now hand-authored content)"))
			return nil
		},
	}
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
