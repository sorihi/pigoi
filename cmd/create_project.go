package cmd

import (
	"fmt"

	"github.com/sorihi/pigoi/internal/generator"
	"github.com/spf13/cobra"
)

func init() {
	var module string
	var templateName string

	command := &cobra.Command{
		Use:   "create-project [name]",
		Short: "Generate a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if module == "" {
				module = name
			}

			err := generator.Generate(generator.Options{
				Name:     name,
				Module:   module,
				Template: templateName,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"Project %s created successfully!\n", name)

			return nil
		},
	}

	command.Flags().StringVar(
		&module, "module", "",
		"Go module path",
	)

	command.Flags().StringVar(
		&templateName, "template", "go-api",
		"Starter template",
	)

	rootCmd.AddCommand(command)
}
