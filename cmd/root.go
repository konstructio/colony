package cmd

import (
	"github.com/spf13/cobra"
)

// GetRootCommand returns the root cobra command
func GetRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "colony",
		Short:         "colony is a tool to manage your data center",
		Long:          ``,
		SilenceUsage:  true,
		SilenceErrors: true, // we print the errors ourselves on main
	}

	cmd.AddCommand(getDestroyCommand(), getInitCommand(), getVersionCommand())
	return cmd
}
