package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
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

			kubeConfigPath := filepath.Join(homeDir, constants.ColonyDir, constants.KubeconfigHostPath)
			if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
				return fmt.Errorf("kubeconfig file not found at %s, cannot clean datacenter", kubeConfigPath)
			}

			k8sClient, err := k8s.New(log, kubeConfigPath)
			if err != nil {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			agentConfig, err := k8sClient.GetAgentConfig(ctx)
			if err != nil {
				return fmt.Errorf("failed to get agent config from cluster: %w", err)
			}

			log.Info("calling clean datacenter endpoint")
			colonyAPI := colony.New(agentConfig.APIURL, agentConfig.APIKey)
			if err := colonyAPI.CleanDatacenter(ctx, agentConfig.AgentID); err != nil {
				return fmt.Errorf("failed to clean datacenter: %w", err)
			}

			log.Info("creating docker client")
			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %w", err)
			}
			defer dockerCLI.Close()

			if err := dockerCLI.RemoveColonyK3sContainer(ctx); err != nil {
				return fmt.Errorf("error: failed to remove colony container %w", err)
			}

			if err := exec.DeleteDirectory(filepath.Join(homeDir, constants.ColonyDir)); err != nil {
				return fmt.Errorf("error: failed to delete kubeconfig file %w", err)
			}

			log.Info("colony installation removed successfully")
			return nil
		},
	}
	return cmd
}
