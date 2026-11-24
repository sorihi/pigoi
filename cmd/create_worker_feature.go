package cmd

import (
	"fmt"

	"github.com/sorihi/pigoi/internal/generator"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "create-worker-feature [name]",
		Short: "Generate and register a RabbitMQ worker feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := generator.GenerateWorkerFeature(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Worker feature %s created successfully! Run it with go run ./cmd\n", args[0])
			return nil
		},
	})
}
