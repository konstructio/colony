package cmd

import (
	// "fmt"
	// "os"

	// "github.com/konstructio/colony/internal/exec"
	// "github.com/konstructio/colony/internal/k8s"

	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/docker"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/util/yaml"
)

func getInitCommand() *cobra.Command {
	var apiKey, apiURL, loadBalancer string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize colony on your host to provision in your data center",
		Long:  `initialize colony on your host to provision in your data center`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(logger.Debug)
			ctx := cmd.Context()

			// leaving this for testing
			os.Setenv("COLONY_API_KEY", "20a711e6-eee0-4ef8-82a0-c5e849575419")

			// Check if an API key comes from the environment
			if apiKey == "" {
				apiKey = os.Getenv("COLONY_API_KEY")
			}

			colonyApi := colony.New(apiURL, apiKey)
			if err := colonyApi.ValidateAPIKey(ctx); err != nil {
				return fmt.Errorf("error validating colony api key: %s %v \n visit https://colony.konstruct.io to get a valid api key", apiKey, err.Error())
			}

			log.Info("colony api key provided is valid")

			dockerCLI, err := docker.New(log)
			if err != nil {
				return fmt.Errorf("error creating docker client: %v ", err.Error())
			}

			err = dockerCLI.CreateColonyK3sContainer(ctx)
			if err != nil {
				return fmt.Errorf("error creating container: %v ", err.Error())
			}

			k8sClient, err := k8s.New(log, "./"+constants.KubeconfigHostPath)
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %v ", err.Error())
			}

			coreDNSDeployment, err := k8sClient.ReturnDeploymentObject(
				"kubernetes.io/name",
				"CoreDNS",
				"kube-system",
				50,
			)
			if err != nil {
				return fmt.Errorf("error finding coredns deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(coreDNSDeployment, 300)

			metricsServerDeployment, err := k8sClient.ReturnDeploymentObject(
				"k8s-app",
				"metrics-server",
				"kube-system",
				50,
			)
			if err != nil {
				return fmt.Errorf("error finding metrics-server deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(metricsServerDeployment, 300)

			time.Sleep(time.Second * 9)

			hegelDeployment, err := k8sClient.ReturnDeploymentObject(
				"app",
				"hegel",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding hegel deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(hegelDeployment, 300)

			rufioDeployment, err := k8sClient.ReturnDeploymentObject(
				"app",
				"rufio",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding rufio deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(rufioDeployment, 300)

			smeeDeployment, err := k8sClient.ReturnDeploymentObject(
				"app",
				"smee",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding smee deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(smeeDeployment, 300)

			tinkServerDeployment, err := k8sClient.ReturnDeploymentObject(
				"app",
				"tink-server",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding tink-server deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(tinkServerDeployment, 300)

			tinkControllerDeployment, err := k8sClient.ReturnDeploymentObject(
				"app",
				"tink-controller",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding tink-controller deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(tinkControllerDeployment, 300)

			// Create the secret
			apiKeySecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "colony-api",
					Namespace: "tink-system",
				},
				Data: map[string][]byte{
					"api-key": []byte(apiKey),
				},
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateSecret(ctx, apiKeySecret); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			k8sconfig, err := ioutil.ReadFile(constants.KubeconfigDockerPath)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err.Error())
			}

			mgmtKubeConfigSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mgmt-kubeconfig",
					Namespace: "tink-system",
				},
				Data: map[string][]byte{
					"kubeconfig": []byte(k8sconfig),
				},
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateSecret(ctx, mgmtKubeConfigSecret); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			colonyAgentDeployment, err := k8sClient.ReturnDeploymentObject(
				"app.kubernetes.io/name",
				"colony-agent",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding colony-agent deployment: %v ", err.Error())
			}

			k8sClient.WaitForDeploymentReady(colonyAgentDeployment, 300)

			time.Sleep(time.Second * 33)
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

			return nil
		},
	}

	// cmd.Flags().StringVar(&apiKey, "apiKey", "", "api key for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&apiURL, "apiURL", "https://colony-api-virtual.konstruct.io", "api url for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&loadBalancer, "loadBalancer", "10.90.14.2", "load balancer ip address (required)")

	return cmd
}

func init() {
}
