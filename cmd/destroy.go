package cmd

import (
	"fmt"
	"os"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
)

func getDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "remove colony deployment from your host",
		Long:  `remove colony deployment from your host`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := logger.New(logger.Debug)

			log.Info("creating docker client")
			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %v ", err.Error())
			}

			err = dockerCLI.RemoveColonyK3sContainer(ctx)
			if err != nil {
				return fmt.Errorf("error: %v", err.Error())
			}

			err = os.Remove(constants.KubeconfigHostPath)
			if err != nil {
				return fmt.Errorf("error removing file: %v ", err.Error())
			}

			log.Info("colony installation removed from host ")

			return nil
		},
	}
	return cmd
}
