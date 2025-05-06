package cmd

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ColonyTokens struct {
	LoadBalancerIP        string
	LoadBalancerInterface string
	DataCenterID          string
	AgentID               string
	ColonyAPIURL          string
}

func getInitCommand() *cobra.Command {
	var dataCenterID, apiKey, agentID, apiURL, loadBalancerIP, loadBalancerInterface string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize colony on your host to provision in your data center",
		Long:  `initialize colony on your host to provision in your data center`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := logger.New(logger.Debug)
			ctx := cmd.Context()

			// Check if an API key comes from the environment
			if apiKey == "" {
				apiKey = os.Getenv("COLONY_API_KEY")
			}

			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %w", err)
			}
			defer dockerCLI.Close()

			containerExists, err := dockerCLI.CheckColonyK3sContainerExists(ctx)
			if err != nil {
				return fmt.Errorf("failed to check for running container: %w", err)
			} else if containerExists {
				return fmt.Errorf("container already exists. please remove before continuing or run `colony destroy`")
			}

			colonyAPI := colony.New(apiURL, apiKey)
			if agentID == "" {
				agent, err := colonyAPI.RegisterAgent(ctx, dataCenterID)
				if err != nil {
					if err == colony.ErrDataCenterAlreadyRegistered {
						return fmt.Errorf("data center %s already has an agent registered", dataCenterID)
					}
					return fmt.Errorf("error registering agent: %w", err)
				}
				agentID = agent.ID
			}

			if err := colonyAPI.Heartbeat(ctx, agentID); err != nil {
				return fmt.Errorf("error sending heartbeat: %w", err)
			}

			log.Info("agent registered successfully")

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error getting user home directory: %w", err)
			}

			err = exec.CreateDirIfNotExist(filepath.Join(homeDir, constants.ColonyDir, "k3s-bootstrap"))
			if err != nil {
				return fmt.Errorf("error creating directory templates: %w", err)
			}

			colonyYamlTmpl, err := manifests.Colony.ReadFile(fmt.Sprintf("colony/%s.tmpl", constants.ColonyYamlPath))
			if err != nil {
				return fmt.Errorf("error reading templates file: %w", err)
			}

			tmpl, err := template.New("colony").Parse(string(colonyYamlTmpl))
			if err != nil {
				return fmt.Errorf("error parsing template: %w", err)
			}

			colonyK3sBootstrapPath := filepath.Join(homeDir, constants.ColonyDir, "k3s-bootstrap", constants.ColonyYamlPath)
			colonyKubeconfigPath := filepath.Join(homeDir, constants.ColonyDir, constants.KubeconfigHostPath)

			outputFile, err := os.Create(colonyK3sBootstrapPath)
			if err != nil {
				return fmt.Errorf("error creating output file: %w", err)
			}
			defer outputFile.Close()

			err = tmpl.Execute(outputFile, &ColonyTokens{
				LoadBalancerIP:        loadBalancerIP,
				LoadBalancerInterface: loadBalancerInterface,
				DataCenterID:          dataCenterID,
				AgentID:               agentID,
				ColonyAPIURL:          apiURL,
			})
			if err != nil {
				return fmt.Errorf("error executing template: %w", err)
			}

			err = dockerCLI.CreateColonyK3sContainer(ctx, colonyK3sBootstrapPath, colonyKubeconfigPath, homeDir)
			if err != nil {
				return fmt.Errorf("error creating container: %w", err)
			}

			k8sClient, err := k8s.New(log, colonyKubeconfigPath)
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
			}

			if err := k8sClient.WaitForKubernetesAPIHealthy(ctx, 5*time.Minute); err != nil {
				return fmt.Errorf("error waiting for kubernetes api to be healthy: %w", err)
			}

			if err := k8sClient.FetchAndWaitForDeployments(ctx, k8s.DeploymentDetails{
				Label:     "kubernetes.io/name",
				Value:     "CoreDNS",
				Namespace: "kube-system",
			}); err != nil {
				return fmt.Errorf("error waiting for coredns deployment: %w", err)
			}

			// Create the secret
			apiKeySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.ColonyAPISecretName,
					Namespace: constants.ColonyNamespace,
				},
				Data: map[string][]byte{
					"api-key": []byte(apiKey),
				},
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateSecret(ctx, apiKeySecret); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			k8sconfig, err := os.ReadFile(colonyKubeconfigPath)
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			mgmtKubeConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mgmt-kubeconfig",
					Namespace: constants.ColonyNamespace,
				},
				Data: map[string][]byte{
					"kubeconfig": k8sconfig,
				},
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateSecret(ctx, mgmtKubeConfigSecret); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			deploymentsToWaitFor := []k8s.DeploymentDetails{
				{
					Label:     "k8s-app",
					Value:     "metrics-server",
					Namespace: "kube-system",
				},
				{
					Label:     "app.kubernetes.io/name",
					Value:     "colony-agent",
					Namespace: constants.ColonyNamespace,
				},
				{
					Label:     "app",
					Value:     "hegel",
					Namespace: constants.ColonyNamespace,
				},
				{
					Label:     "app",
					Value:     "rufio",
					Namespace: constants.ColonyNamespace,
				},
				{
					Label:     "app",
					Value:     "smee",
					Namespace: constants.ColonyNamespace,
				},
				{
					Label:     "app",
					Value:     "tink-server",
					Namespace: constants.ColonyNamespace,
				},
				{
					Label:       "app",
					Value:       "tink-controller",
					Namespace:   constants.ColonyNamespace,
					ReadTimeout: 180,
					WaitTimeout: 120,
				},
			}

			if err := k8sClient.FetchAndWaitForDeployments(ctx, deploymentsToWaitFor...); err != nil {
				return fmt.Errorf("error waiting for deployment: %w", err)
			}

			if err := k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
			}

			log.Info("applying tinkerbell templates")
			colonyTemplates, err := manifests.Templates.ReadDir("templates")
			if err != nil {
				return fmt.Errorf("error reading templates: %w", err)
			}
			var manifestsFiles []string

			for _, file := range colonyTemplates {
				content, err := manifests.Templates.ReadFile(filepath.Join("templates", file.Name()))
				if err != nil {
					return fmt.Errorf("error reading templates file: %w", err)
				}
				manifestsFiles = append(manifestsFiles, string(content))
			}

			if err := k8sClient.ApplyManifests(ctx, manifestsFiles); err != nil {
				return fmt.Errorf("error applying templates: %w", err)
			}

			log.Info("downloading operating systems for hook")

			downloadTemplates, err := manifests.Downloads.ReadDir("downloads")
			if err != nil {
				return fmt.Errorf("error reading templates: %w", err)
			}

			var downloadFiles []string

			for _, file := range downloadTemplates {
				content, err := manifests.Downloads.ReadFile(filepath.Join("downloads", file.Name()))
				if err != nil {
					return fmt.Errorf("error reading templates file: %w", err)
				}
				downloadFiles = append(downloadFiles, string(content))
			}

			if err := k8sClient.ApplyManifests(ctx, downloadFiles); err != nil {
				return fmt.Errorf("error applying templates: %w", err)
			}

			clusterRoleName := "smee-role"

			patch := []map[string]interface{}{{
				"op":   "add",
				"path": "/rules/-",
				"value": rbacv1.PolicyRule{
					APIGroups: []string{"tinkerbell.org"},
					Resources: []string{"hardware", "hardware/status"},
					Verbs:     []string{"create", "update"},
				},
			}}

			// Convert patch to JSON
			patchBytes, err := json.Marshal(patch)
			if err != nil {
				return fmt.Errorf("error marshalling clusterrole patch: %w", err)
			}

			err = k8sClient.PatchClusterRole(ctx, clusterRoleName, patchBytes)
			if err != nil {
				return fmt.Errorf("error patching ClusterRole: %w", err)
			}
			log.Info("colony init completed successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "api key for interacting with colony cloud")
	cmd.Flags().StringVar(&dataCenterID, "data-center-id", "", "data center id for interacting with colony cloud")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "agent id for interacting with colony cloud")
	cmd.Flags().StringVar(&apiURL, "api-url", "https://colony-api.konstruct.io", "api url for interacting with colony cloud")
	cmd.Flags().StringVar(&loadBalancerInterface, "load-balancer-interface", "", "the local network interface for colony to use")
	cmd.Flags().StringVar(&loadBalancerIP, "load-balancer-ip", "", "the local network interface for colony to use")

	cmd.MarkFlagRequired("api-key")
	cmd.MarkFlagRequired("data-center-id")
	cmd.MarkFlagRequired("load-balancer-interface")
	cmd.MarkFlagRequired("load-balancer-ip")
	return cmd
}
