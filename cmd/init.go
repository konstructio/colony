package cmd

import (
	"context"
	"fmt"

	"github.com/konstructio/colony/internal/colonyapi"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

const (
	helmChartName     = "colony"
	helmChartRepoName = "konstruct"
	helmChartRepoURL  = "https://charts.konstruct.io"
	helmChartVersion  = "0.0.4"
	namespace         = "tink-system"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize colony in your management cluster",
	Long:  `initialize colony in your management cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.DebugLevel)

		// Set log format (optional)
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})

		ctx := context.Background()
		// Get flags values
		apiKey, _ := cmd.Flags().GetString("apiKey")
		colonyAPIURL, _ := cmd.Flags().GetString("apiURL")
		loadBalancer, _ := cmd.Flags().GetString("loadBalancer")

		colonyApi := colonyapi.New(colonyAPIURL, apiKey)
		if err := colonyApi.ValidateApiKey(ctx); err != nil {
			log.Fatalf("error validating api key: %s", err)
		}

		log.Info("initializing colony cloud with api key: ", apiKey)

		k8sClient, err := k8s.New("/home/vagrant/.kube/config")
		if err != nil {
			log.Fatalf("error creating k8s client: %s", err)
		}

		// Create a secret in the cluster
		if err := k8sClient.CreateAPIKeySecret(ctx, apiKey); err != nil {
			log.Fatalf("error creating secret: %s", err)
		}

		log.Info("Applying tink templates")
		templates, err := colonyApi.GetSystemTemplates(ctx)
		if err != nil {
			log.Fatalf("error getting system templates: %s", err)
		}

		var manifests []string
		for _, template := range templates {
			manifests = append(manifests, template.Template)
		}

		if err := k8sClient.ApplyManifests(ctx, manifests); err != nil {
			log.Fatalf("error applying templates: %s", err)
		}

		log.Info("Installing Colony helm chart")
		res, err := exec.ExecuteCommand(
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
				log.Fatalf("could not get existing helm repositories: %s", err.Error())
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
				"helm",
				"repo",
				"add",
				helmChartRepoName,
				helmChartRepoURL,
			)
			if err != nil {
				log.Fatalf("error adding helm chart repository: %s", err)
			}
			log.Infof("Added Colony helm chart repository")
		} else {
			log.Infof("Colony helm chart repository already added")
		}

		// Update helm chart repository locally
		_, err = exec.ExecuteCommand(
			"helm",
			"repo",
			"update",
		)
		if err != nil {
			log.Errorf("error updating helm chart repository: %s", err)
		}

		log.Infof("Colony helm chart repository updated")

		// Determine if helm release has already been installed
		res, err = exec.ExecuteCommand(
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
			log.Errorf("could not get existing helm releases: %s", err)
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
				fmt.Sprintf("colony-agent.extraEnv.COLONY_API_URL=%s", colonyAPIURL),
			}

			_, err = exec.ExecuteCommand("helm", installFlags...)
			if err != nil {
				log.Fatalf("error installing helm chart: %s", err)
			}
			log.Print("Colony helm chart installed")
		} else {
			log.Printf("Colony helm chart already installed")
		}
	},
}

func init() {
	initCmd.Flags().String("apiKey", "", "api key for interacting with colony cloud (required)")
	initCmd.MarkFlagRequired("apiKey")
	initCmd.Flags().String("apiURL", "", "api url for interacting with colony cloud (required)")
	initCmd.MarkFlagRequired("apiURL")
	initCmd.Flags().String("loadBalancer", "10.0.10.2", "load balancer ip address (required)")
}
