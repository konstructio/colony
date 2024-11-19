package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getInitCommand() *cobra.Command {
	var apiKey, apiURL, loadBalancerIP, loadBalancerInterface string

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

			colonyAPI := colony.New(apiURL, apiKey)
			if err := colonyAPI.ValidateAPIKey(ctx); err != nil {
				return fmt.Errorf("error validating colony api key: %q %w \n visit https://colony.konstruct.io to get a valid api key", apiKey, err)
			}

			log.Info("colony api key provided is valid")

			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %w", err)
			}
			defer dockerCLI.Close()

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error getting current working directory: %w", err)
			}

			err = dockerCLI.CreateColonyK3sContainer(ctx, loadBalancerIP, loadBalancerInterface, pwd)
			if err != nil {
				return fmt.Errorf("error creating container: %w", err)
			}

			k8sClient, err := k8s.New(log, filepath.Join(pwd, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
			}

			if err := k8sClient.WaitForKubernetesAPIHealthy(ctx, 5*time.Minute); err != nil {
				return fmt.Errorf("error waiting for kubernetes api to be healthy: %w", err)
			}

			if err := k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
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

			k8sconfig, err := os.ReadFile(constants.KubeconfigHostPath)
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

			k8sClient, err = k8s.New(log, filepath.Join(pwd, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
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

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "api key for interacting with colony cloud")
	cmd.Flags().StringVar(&apiURL, "api-url", "https://colony-api-virtual.konstruct.io", "api url for interacting with colony cloud")
	cmd.Flags().StringVar(&loadBalancerInterface, "load-balancer-interface", "", "the local network interface for colony to use")
	cmd.Flags().StringVar(&loadBalancerIP, "load-balancer-ip", "", "the local network interface for colony to use")

	cmd.MarkFlagRequired("api-key")
	cmd.MarkFlagRequired("load-balancer-interface")
	cmd.MarkFlagRequired("load-balancer-ip")
	return cmd
}
