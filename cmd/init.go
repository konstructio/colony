package cmd

import (
	"fmt"
	"os"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	helmChartName     = "colony"
	helmChartRepoName = "konstruct"
	helmChartRepoURL  = "https://charts.konstruct.io"
	helmChartVersion  = "0.0.6-rc3"
	namespace         = "tink-system"
)

func getInitCommand() *cobra.Command {
	var apiKey, apiURL, loadBalancer string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize colony in your management cluster",
		Long:  `initialize colony in your management cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(logger.Debug)
			ctx := cmd.Context()

			// Check if an API key comes from the environment
			if apiKey == "" {
				apiKey = os.Getenv("COLONY_API_KEY")
			}

			colonyApi := colony.New(apiURL, apiKey)
			if err := colonyApi.ValidateAPIKey(ctx); err != nil {
				return fmt.Errorf("error validating api key: %w", err)
			}

			log.Info("initializing colony cloud with api key:", apiKey)

			k8sClient, err := k8s.New(log, "/home/vagrant/.kube/config")
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateAPIKeySecret(ctx, apiKey); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			log.Info("Applying tink templates")
			templates, err := colonyApi.GetSystemTemplates(ctx)
			if err != nil {
				return fmt.Errorf("error getting system templates: %w", err)
			}

			var manifests []string
			for _, template := range templates {
				manifests = append(manifests, template.Template)
			}

			if err := k8sClient.ApplyManifests(ctx, manifests); err != nil {
				return fmt.Errorf("error applying templates: %w", err)
			}

			log.Info("Installing Colony helm chart")
			res, err := exec.ExecuteCommand(
				log,
				"helm",
				"-n",
				"tink-system",
				"repo",
				"list",
				"-o",
				"yaml",
			)
			if err != nil {
				log.Warn("no repos found on helm:" + err.Error())
			}

			var existingHelmRepositories []struct {
				Name string `yaml:"name"`
				URL  string `yaml:"url"`
			}
			if res != "" {
				if err := yaml.Unmarshal([]byte(res), &existingHelmRepositories); err != nil {
					return fmt.Errorf("could not get existing helm repositories: %w", err)
				}
			}

			repoExists := func() bool {
				for _, repo := range existingHelmRepositories {
					if repo.Name == helmChartRepoName && repo.URL == helmChartRepoURL {
						return true
					}
				}
				return false
			}()

			if !repoExists {
				// Add helm chart repository
				_, err = exec.ExecuteCommand(
					log,
					"helm",
					"repo",
					"add",
					helmChartRepoName,
					helmChartRepoURL,
				)
				if err != nil {
					return fmt.Errorf("error adding helm chart repository: %w", err)
				}
				log.Infof("Added Colony helm chart repository")
			} else {
				log.Infof("Colony helm chart repository already added")
			}

			// Update helm chart repository locally
			_, err = exec.ExecuteCommand(
				log,
				"helm",
				"repo",
				"update",
			)
			if err != nil {
				return fmt.Errorf("error updating helm chart repository: %w", err)
			}

			log.Infof("Colony helm chart repository updated")

			// Determine if helm release has already been installed
			res, err = exec.ExecuteCommand(
				log,
				"helm",
				"-n",
				"tink-system",
				"list",
				"-o",
				"yaml",
				"-A",
			)
			if err != nil {
				log.Infof("error listing current helm repositories installed: %s", err)
			}

			type helmRelease struct {
				AppVersion string `yaml:"app_version"`
				Chart      string `yaml:"chart"`
				Name       string `yaml:"name"`
				Namespace  string `yaml:"namespace"`
				Revision   string `yaml:"revision"`
				Status     string `yaml:"status"`
				Updated    string `yaml:"updated"`
			}

			var existingHelmReleases []helmRelease
			if err := yaml.Unmarshal([]byte(res), &existingHelmReleases); err != nil {
				return fmt.Errorf("could not get existing helm releases: %w", err)
			}

			chartInstalled := func() bool {
				for _, release := range existingHelmReleases {
					if release.Name == helmChartName {
						return true
					}
				}
				return false
			}()

			if !chartInstalled {
				installFlags := []string{
					"install",
					"--namespace",
					namespace,
					helmChartName,
					"--version",
					helmChartVersion,
					"konstruct/colony",
					"--set",
					"colony-agent.extraEnvSecrets.API_TOKEN.name=colony-api",
					"--set",
					"colony-agent.extraEnvSecrets.API_TOKEN.key=api-key",
					"--set",
					fmt.Sprintf("colony-agent.extraEnv.LOAD_BALANCER=%s", loadBalancer),
					"--set",
					fmt.Sprintf("colony-agent.extraEnv.COLONY_API_URL=%s", apiURL),
					"--set",
					fmt.Sprintf("colony-agent.extraEnv.TALOS_URL_FILES_SOURCE=http://%s:8080", loadBalancer),
				}

				_, err = exec.ExecuteCommand(log, "helm", installFlags...)
				if err != nil {
					return fmt.Errorf("error installing helm chart: %w", err)
				}
				log.Info("Colony helm chart installed")
			} else {
				log.Info("Colony helm chart already installed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "apiKey", "", "api key for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&apiURL, "apiURL", "", "api url for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&loadBalancer, "loadBalancer", "10.0.10.2", "load balancer ip address (required)")

	return cmd
}

func init() {
}
