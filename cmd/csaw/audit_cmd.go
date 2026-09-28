package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/audit"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

func newAuditCommand() *cobra.Command {
	var strict bool
	var jsonOut bool
	var initPolicy bool
	var forceInit bool

	cmd := &cobra.Command{
		Use:   "audit [path]",
		Short: "Audit active AI workspace context against project policy",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}

			projectRoot, err := runtime.FindRepoRoot(target)
			if err != nil {
				return err
			}

			if initPolicy {
				if jsonOut || strict {
					return errors.New("--init cannot be combined with --json or --strict")
				}
				policyPath, created, err := audit.InitPolicy(projectRoot, audit.InitOptions{Force: forceInit})
				if err != nil {
					return err
				}
				action := "created"
				if !created {
					action = "updated"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", action, policyPath)
				return nil
			}

			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			report, err := audit.Run(projectRoot, paths)
			if err != nil {
				return err
			}

			if jsonOut {
				content, err := audit.RenderJSON(report)
				if err != nil {
					return err
				}
				if _, err := cmd.OutOrStdout().Write(content); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), audit.RenderText(report))
			}

			if report.Failed(strict) {
				return errors.New(report.FailureSummary(strict))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings as well as errors")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a JSON audit report")
	cmd.Flags().BoolVar(&initPolicy, "init", false, "write a starter .csaw/policy.yml")
	cmd.Flags().BoolVar(&forceInit, "force", false, "overwrite an existing audit policy when used with --init")
	return cmd
}
