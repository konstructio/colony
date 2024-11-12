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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := logger.New(logger.Debug)

			log.Info("creating docker client")
			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %w", err)
			}
			defer dockerCLI.Close()

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error getting current working directory: %w", err)
			}

			err = dockerCLI.RemoveColonyK3sContainer(ctx)
			if err != nil {
				return fmt.Errorf("error: failed to remove colony container %w", err)
			}

			err = exec.DeleteFile(filepath.Join(pwd, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("error: failed to delete kubeconfig file %w", err)
			}

			//! templates directory is not removed

			log.Info("colony installation removed from host")
			return nil
		},
	}
	return cmd
}
