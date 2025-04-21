package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
)

func getAssetsCommand() *cobra.Command {
	assetsCmd := &cobra.Command{
		Use:   "assets",
		Short: "list the colony assets in the data center",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := logger.New(logger.Debug)
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error getting user home directory: %w", err)
			}

			k8sClient, err := k8s.New(log, filepath.Join(homeDir, constants.ColonyDir, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			if err = k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
			}
			err = k8sClient.ListAssets(ctx)
			if err != nil {
				return fmt.Errorf("error listing assets: %w", err)
			}

			return nil
		},
	}
	return assetsCmd
}
