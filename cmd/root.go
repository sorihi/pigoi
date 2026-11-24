package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "pigoi",
	Short:         "Pigoi - your humane Go development toolkit",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
