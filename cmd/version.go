package cmd

import (
	"github.com/konstructio/colony/configs"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
)

func getVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print the version for colony cli",
		Long:  `print the version for colony cli`,
		RunE: func(_ *cobra.Command, _ []string) error {
			log := logger.New(logger.Debug)
			log.Info("colony cli version: ", configs.Version)
			return nil
		},
	}
	return cmd
}
