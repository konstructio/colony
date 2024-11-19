package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
)

func getDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "remove colony deployment from your host",
		Long:  `remove colony deployment from your host`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := logger.New(logger.Debug)

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error getting user home directory: %w", err)
			}

			log.Info("creating docker client")
			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %w", err)
			}
			defer dockerCLI.Close()

			err = dockerCLI.RemoveColonyK3sContainer(ctx)
			if err != nil {
				return fmt.Errorf("error: failed to remove colony container %w", err)
			}

			err = exec.DeleteDirectory(filepath.Join(homeDir, constants.ColonyDir))
			if err != nil {
				return fmt.Errorf("error: failed to delete kubeconfig file %w", err)
			}

			log.Info("colony installation removed successfully")

			return nil
		},
	}
	return cmd
}
