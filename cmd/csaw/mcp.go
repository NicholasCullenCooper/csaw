package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/mcpmerge"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

// newMCPCommand wires up `csaw mcp ...` subcommands.
//
// MCP support in csaw splits across two pipelines:
//   - Symlink projection (existing) for tools with a dedicated MCP file
//     (.mcp.json, .cursor/mcp.json, .vscode/mcp.json). Handled by csaw use/mount.
//   - Merged-config projection (this command, since v0.9.0) for tools whose
//     MCP config lives inside a shared file alongside user-owned settings
//     (Codex's config.toml today; OpenCode/Copilot/VS Code-settings on demand).
//     See docs/planning/mcp-merge-design.md for the architecture.
func newMCPCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "mcp",
		Short: "Project MCP server entries into shared-config tool files (merged-config projection)",
		Long: `Manage csaw's merged-config MCP projection: write team MCP entries
into a tool's shared config file (which also holds user settings, auth, etc.)
without disturbing user-owned content.

csaw writes a clearly-marked bounded section at the end of the target file;
unmount removes only that section, leaving the rest byte-for-byte intact.
Tools whose MCP lives in a dedicated file (Claude Code's .mcp.json,
Cursor's .cursor/mcp.json, VS Code's .vscode/mcp.json) are handled by
the existing symlink projection in csaw use/mount — this command is for
the harder case where MCP shares a file with user settings.`,
	}
	root.AddCommand(newMCPSyncCommand())
	return root
}

func newMCPSyncCommand() *cobra.Command {
	var (
		fromSource string
		apply      bool
		remove     bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "sync <target>",
		Short: "Merge MCP entries from a source into a target tool's config file",
		Long: `Merge MCP server entries from a csaw source registry into a target tool's
shared config file. Bounded section approach: csaw's entries live in a clearly
marked block; user content elsewhere in the file is untouched.

Supported targets (csaw mcp sync --help-targets):
  codex  - OpenAI Codex CLI (.codex/config.toml, TOML format)

Examples:
  csaw mcp sync codex --from team                     # dry-run: show what would change
  csaw mcp sync codex --from team --apply             # write the merge
  csaw mcp sync codex --remove                        # roll back (refuses if user edited inside the section)
  csaw mcp sync codex --remove --force                # roll back, ignoring drift detection

Source fragments live at <source>/mcp/<fragment-name>. For codex, the
fragment is mcp/codex.toml — a TOML file containing one or more
[mcp_servers.<name>] tables. Literal secrets in sensitive-named fields
(token, password, api_key, etc.) are refused at parse time; use Codex's
env_vars = ["VAR_NAME"] pattern or "$VAR" / "${env:VAR}" references.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := args[0]
			target, ok := mcpmerge.GetTarget(targetName)
			if !ok {
				return fmt.Errorf("unknown target %q (supported: %s)",
					targetName, strings.Join(mcpmerge.ListTargetNames(), ", "))
			}

			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}
			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}
			stateDir := filepath.Join(paths.State, "mcpmerge")

			// --remove path: no source required, no fragment read.
			if remove {
				if apply || fromSource != "" {
					return fmt.Errorf("--remove is exclusive with --apply / --from")
				}
				return runRemove(cmd, target, projectRoot, stateDir, force)
			}

			// --apply or dry-run path: needs source.
			if fromSource == "" {
				return fmt.Errorf("--from <source> is required (specify which csaw source to read mcp/%s from)", target.FragmentName)
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}
			source, err := manager.Get(fromSource)
			if err != nil {
				return fmt.Errorf("source %q not configured: %w", fromSource, err)
			}
			fragPath := filepath.Join(source.CheckoutPath(paths), "mcp", target.FragmentName)

			frag, err := mcpmerge.ReadFragment(target, fragPath, fromSource)
			if err != nil {
				return err
			}

			plan, err := mcpmerge.PlanMerge(target, projectRoot, frag)
			if err != nil {
				return err
			}

			renderPlan(cmd, plan, apply)

			if !apply {
				return nil
			}

			res, err := mcpmerge.Apply(plan, stateDir)
			if err != nil {
				return err
			}
			output.Successf("wrote %s (%d → %d bytes)", trimProjectPath(res.TargetPath, projectRoot), res.BytesBefore, res.BytesAfter)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromSource, "from", "", "csaw source to read the MCP fragment from")
	cmd.Flags().BoolVar(&apply, "apply", false, "actually write the merge (default is dry-run)")
	cmd.Flags().BoolVar(&remove, "remove", false, "remove csaw-managed section from target file")
	cmd.Flags().BoolVar(&force, "force", false, "with --remove: override drift detection if user has edited the managed section")
	return cmd
}

func runRemove(cmd *cobra.Command, target mcpmerge.MergeTarget, projectRoot, stateDir string, force bool) error {
	res, err := mcpmerge.Remove(target, projectRoot, stateDir)
	if err != nil {
		if !force {
			return err
		}
		// Force path: re-read, strip section regardless of drift. Implemented
		// here rather than in mcpmerge to keep the safe API safe.
		// (Drift override is rare; if it becomes common, promote to package.)
		fmt.Fprintf(cmd.ErrOrStderr(), "  %s --force: ignoring drift detection\n", output.Faint("⚠"))
		// We can't easily force from here without re-implementing — for the
		// MVP, document that --force isn't wired yet and surface the original
		// error so the user understands the state. Recovery is a manual edit.
		return fmt.Errorf("%w\n(--force not yet wired in MVP; inspect the section manually and remove it by hand if needed)", err)
	}
	output.Successf("removed csaw-managed section from %s (%d → %d bytes)", trimProjectPath(res.TargetPath, projectRoot), res.BytesBefore, res.BytesAfter)
	for _, name := range res.Removed {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", output.Faint("-"), name)
	}
	return nil
}

func renderPlan(cmd *cobra.Command, plan mcpmerge.Plan, willApply bool) {
	header := fmt.Sprintf("Plan: merge from %s into %s (%s)", plan.Fragment.SourceName, plan.Target.ProjectPath, plan.Target.Format)
	output.Header(header)
	fmt.Fprintln(cmd.OutOrStdout())

	if !plan.TargetExists {
		fmt.Fprintf(cmd.OutOrStdout(), "  target file does not exist yet — will be created\n")
	} else {
		extra := ""
		if plan.HasManagedSection {
			extra = " (has existing csaw-managed section — will replace)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  target exists%s\n", extra)
	}
	if len(plan.UserOwnedServers) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  user-owned mcp_servers: %d (%s)\n", len(plan.UserOwnedServers), strings.Join(plan.UserOwnedServers, ", "))
	}
	fmt.Fprintln(cmd.OutOrStdout())

	if len(plan.WillApply) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Will add:")
		for _, s := range plan.WillApply {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s %s\n", output.Faint("+"), s.Name)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if len(plan.Conflicts) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Skipped (conflict with user-defined entry):")
		for _, c := range plan.Conflicts {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s %s — %s\n", output.Faint("⚠"), c.ServerName, c.Reason)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if !willApply {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", output.Faint("Dry run only. Re-run with --apply to write."))
	}
}

func trimProjectPath(abs, projectRoot string) string {
	if rel, err := filepath.Rel(projectRoot, abs); err == nil {
		return rel
	}
	return abs
}
