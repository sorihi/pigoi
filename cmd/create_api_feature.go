package cmd

import (
	"fmt"

	"github.com/sorihi/pigoi/internal/generator"
	"github.com/spf13/cobra"
)

func init() {
	command := &cobra.Command{
		Use:   "create-api-feature [name]",
		Short: "Generate and register a feature in the current project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := generator.GenerateFeature(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Feature %s created successfully!\n", args[0])
			return nil
		},
	}
	rootCmd.AddCommand(command)
}
