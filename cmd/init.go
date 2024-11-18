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
	"github.com/konstructio/colony/internal/download"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
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

			// TODO hack, the kube api is not always ready need to figure out a better condition
			time.Sleep(time.Second * 7)

			k8sClient, err := k8s.New(log, filepath.Join(pwd, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
			}

			coreDNSDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"kubernetes.io/name",
				"CoreDNS",
				"kube-system",
				50,
			)
			if err != nil {
				return fmt.Errorf("error finding coredns deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, coreDNSDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for coredns deployment: %w", err)
			}

			// Create the secret
			apiKeySecret := &corev1.Secret{
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

			k8sconfig, err := os.ReadFile(constants.KubeconfigHostPath)
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			mgmtKubeConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mgmt-kubeconfig",
					Namespace: "tink-system",
				},
				Data: map[string][]byte{
					"kubeconfig": k8sconfig,
				},
			}

			// Create a secret in the cluster
			if err := k8sClient.CreateSecret(ctx, mgmtKubeConfigSecret); err != nil {
				return fmt.Errorf("error creating secret: %w", err)
			}

			metricsServerDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"k8s-app",
				"metrics-server",
				"kube-system",
				50,
			)
			if err != nil {
				return fmt.Errorf("error finding metrics-server deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, metricsServerDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for metrics server deployment: %w", err)
			}

			colonyAgentDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app.kubernetes.io/name",
				"colony-agent",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding colony-agent deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, colonyAgentDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for colony agent deployment: %w", err)
			}

			hegelDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app",
				"hegel",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding hegel deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, hegelDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for hegel deployment: %w", err)
			}

			rufioDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app",
				"rufio",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding rufio deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, rufioDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for rufio deployment: %w", err)
			}

			smeeDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app",
				"smee",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding smee deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, smeeDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for smee deployment: %w", err)
			}

			tinkServerDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app",
				"tink-server",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding tink-server deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, tinkServerDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for tink server deployment: %w", err)
			}

			tinkControllerDeployment, err := k8sClient.ReturnDeploymentObject(
				ctx,
				"app",
				"tink-controller",
				"tink-system",
				180,
			)
			if err != nil {
				return fmt.Errorf("error finding tink-controller deployment: %w", err)
			}

			_, err = k8sClient.WaitForDeploymentReady(ctx, tinkControllerDeployment, 120)
			if err != nil {
				return fmt.Errorf("error waiting for tink controller deployment: %w", err)
			}

			k8sClient, err = k8s.New(log, filepath.Join(pwd, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("error creating Kubernetes client: %w", err)
			}

			log.Info("Applying tink templates")
			err = exec.CreateDirIfNotExist(filepath.Join(pwd, "templates"))
			if err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}

			templates := []string{"ubuntu-focal-k3s-server.yaml", "ubuntu-focal.yaml", "discovery.yaml", "reboot.yaml", "ubuntu-focal-k3s-join.yaml"}
			for _, template := range templates {
				url := fmt.Sprintf("https://raw.githubusercontent.com/jarededwards/k3s-datacenter/refs/heads/main/templates/%s", template)
				filename := filepath.Join(pwd, "templates", template)

				log.Infof("downloading template from %q into %q", url, filename)
				err := download.FileFromURL(url, filename)
				if err != nil {
					return fmt.Errorf("error downloading file: %w", err)
				}

				log.Info("downloaded:", filename)
			}

			manifests, err := exec.ReadFilesInDir(filepath.Join(pwd, "templates"))
			if err != nil {
				return fmt.Errorf("error reading files directory: %w", err)
			}

			if err := k8sClient.ApplyManifests(ctx, manifests); err != nil {
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

			talosConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "download-talos",
					Namespace: "tink-system",
				},
				Data: map[string]string{
					"entrypoint.sh": `#!/usr/bin/env bash
					# This script is designed to download specific Talos files required for an IPXE script to work.
					set -euxo pipefail
					if ! which wget &>/dev/null; then
					apk add --update wget
					fi
					base_url=$1
					output_dir=$2
					files=("initramfs-amd64.xz" "vmlinuz-amd64")
					for file in "${files[@]}"; do
					wget "${base_url}/${file}" -O "${output_dir}/${file}"
					done`,
				},
			}

			err = k8sClient.CreateConfigMap(ctx, talosConfigMap)
			if err != nil {
				return fmt.Errorf("error creating configmap: %w", err)
			}
			talosJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "download-talos",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "download-talos",
									Image:   "bash:5.2.2",
									Command: []string{"/script/entrypoint.sh"},
									Args: []string{
										"https://github.com/siderolabs/talos/releases/download/v1.8.0",
										"/output",
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "hook-artifacts",
											MountPath: "/output",
										},
										{
											Name:      "configmap-volume",
											MountPath: "/script",
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Volumes: []corev1.Volume{
								{
									Name: "hook-artifacts",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/opt/hook",
											Type: new(corev1.HostPathType), // DirectoryOrCreate by default
										},
									},
								},
								{
									Name: "configmap-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											DefaultMode: new(int32), // 0700 in octal
										},
									},
								},
							},
						},
					},
				},
			}
			err = k8sClient.CreateJob(ctx, talosJob)
			if err != nil {
				return fmt.Errorf("error creating job: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "api key for interacting with colony cloud")
	cmd.Flags().StringVar(&apiURL, "api-url", "https://colony-api-virtual.konstruct.io", "api url for interacting with colony cloud")
	cmd.Flags().StringVar(&loadBalancerInterface, "load-balancer-interface", "", "the local network interface for colony to use")
	cmd.Flags().StringVar(&loadBalancerIP, "load-balancer-ip", "", "the local network interface for colony to use")

	cmd.MarkFlagRequired("api-key")
	cmd.MarkFlagRequired("api-url")
	cmd.MarkFlagRequired("load-balancer-interface")
	cmd.MarkFlagRequired("load-balancer-ip")
	return cmd
}
