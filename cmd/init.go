package cmd

import (
	// "fmt"
	// "os"

	// "github.com/konstructio/colony/internal/exec"
	// "github.com/konstructio/colony/internal/k8s"

	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/konstructio/colony/internal/colony"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	// v1 "k8s.io/api/core/v1"
	// rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/util/yaml"
)

func getInitCommand() *cobra.Command {
	var apiKey, apiURL, loadBalancerIP, loadBalancerInterface string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize colony on your host to provision in your data center",
		Long:  `initialize colony on your host to provision in your data center`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(logger.Debug)
			ctx := cmd.Context()

			// Check if an API key comes from the environment
			if apiKey == "" {
				apiKey = os.Getenv("COLONY_API_KEY")
			}

			colonyApi := colony.New(apiURL, apiKey)
			if err := colonyApi.ValidateAPIKey(ctx); err != nil {
				return fmt.Errorf("error validating colony api key: %q %w \n visit https://colony.konstruct.io to get a valid api key", apiKey, err)
			}

			log.Info("colony api key provided is valid")

			// dockerCLI, err := docker.New(log)
			// if err != nil {
			// 	return fmt.Errorf("error creating docker client: %w", err)
			// }

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error getting current working directory: %w", err)
			}

			// err = dockerCLI.CreateColonyK3sContainer(ctx, loadBalancerIP, loadBalancerInterface, pwd)
			// if err != nil {
			// 	return fmt.Errorf("error creating container: %w", err)
			// }

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

			k8sClient.WaitForDeploymentReady(ctx, coreDNSDeployment, 120)

			// Create the secret
			// apiKeySecret := &v1.Secret{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "colony-api",
			// 		Namespace: "tink-system",
			// 	},
			// 	Data: map[string][]byte{
			// 		"api-key": []byte(apiKey),
			// 	},
			// }

			// // Create a secret in the cluster
			// if err := k8sClient.CreateSecret(ctx, apiKeySecret); err != nil {
			// 	return fmt.Errorf("error creating secret: %w", err)
			// }

			// k8sconfig, err := ioutil.ReadFile(constants.KubeconfigHostPath)
			// if err != nil {
			// 	return fmt.Errorf("error reading file: %w", err)
			// }

			// mgmtKubeConfigSecret := &v1.Secret{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "mgmt-kubeconfig",
			// 		Namespace: "tink-system",
			// 	},
			// 	Data: map[string][]byte{
			// 		"kubeconfig": []byte(k8sconfig),
			// 	},
			// }

			// // Create a secret in the cluster
			// if err := k8sClient.CreateSecret(ctx, mgmtKubeConfigSecret); err != nil {
			// 	return fmt.Errorf("error creating secret: %w", err)
			// }

			// metricsServerDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"k8s-app",
			// 	"metrics-server",
			// 	"kube-system",
			// 	50,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding metrics-server deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, metricsServerDeployment, 120)

			// colonyAgentDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app.kubernetes.io/name",
			// 	"colony-agent",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding colony-agent deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, colonyAgentDeployment, 120)

			// hegelDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app",
			// 	"hegel",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding hegel deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, hegelDeployment, 120)

			// rufioDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app",
			// 	"rufio",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding rufio deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, rufioDeployment, 120)

			// smeeDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app",
			// 	"smee",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding smee deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, smeeDeployment, 120)

			// tinkServerDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app",
			// 	"tink-server",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding tink-server deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, tinkServerDeployment, 120)

			// tinkControllerDeployment, err := k8sClient.ReturnDeploymentObject(
			// 	ctx,
			// 	"app",
			// 	"tink-controller",
			// 	"tink-system",
			// 	180,
			// )
			// if err != nil {
			// 	return fmt.Errorf("error finding tink-controller deployment: %w", err)
			// }

			// k8sClient.WaitForDeploymentReady(ctx, tinkControllerDeployment, 120)

			// k8sClient, err = k8s.New(log, filepath.Join(pwd, constants.KubeconfigHostPath))
			// if err != nil {
			// 	return fmt.Errorf("error creating Kubernetes client: %w", err)
			// }

			// log.Info("Applying tink templates")
			// err = createDirIfNotExist(filepath.Join(pwd, "templates"))
			// if err != nil {
			// 	return fmt.Errorf("error creating directory: %w", err)
			// }

			// templates := []string{"ubuntu-focal-k3s-server.yaml", "ubuntu-focal.yaml", "discovery.yaml", "reboot.yaml", "ubuntu-focal-k3s-join.yaml"}
			// for _, template := range templates {
			// 	url := fmt.Sprintf("https://raw.githubusercontent.com/jarededwards/k3s-datacenter/refs/heads/main/templates/%s", template)
			// 	filename := filepath.Join(pwd, "templates/"+template)

			// 	fmt.Println(filename)
			// 	err := download.FileFromURL(url, filename)
			// 	if err != nil {
			// 		return fmt.Errorf("error downloading file: %w", err)
			// 	} else {
			// 		log.Info("downloaded:", filename)
			// 	}

			// }

			// manifests, err := readFilesInDir(filepath.Join(pwd, "templates"))
			// if err != nil {
			// 	return fmt.Errorf("error reading files directory: %w", err)
			// }

			// if err := k8sClient.ApplyManifests(ctx, manifests); err != nil {
			// 	return fmt.Errorf("error applying templates: %w", err)
			// }

			// clusterRoleName := "smee-role"

			// patch := []map[string]interface{}{
			// 	{
			// 		"op":   "add",
			// 		"path": "/rules/-",
			// 		"value": rbacv1.PolicyRule{
			// 			APIGroups: []string{"tinkerbell.org"},
			// 			Resources: []string{"hardware", "hardware/status"},
			// 			Verbs:     []string{"create", "update"},
			// 		},
			// 	},
			// }

			// // Convert patch to JSON
			// patchBytes, err := json.Marshal(patch)
			// if err != nil {
			// 	return fmt.Errorf("error marshalling clusterrole patch: %w", err)
			// }

			// err = k8sClient.PatchClusterRole(ctx, clusterRoleName, patchBytes)
			// if err != nil {
			// 	return fmt.Errorf("error patching ClusterRole: %w", err)
			// }

			//!

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

			ubuntuJammyConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "download-ubuntu-jammy",
					Namespace: "tink-system",
				},
				Data: map[string]string{
					"entrypoint.sh": `#!/usr/bin/env bash
# This script is designed to download a cloud image file (.img) and then convert it to a .raw.gz file.
# This is purpose built so non-raw cloud image files can be used with the "image2disk" action.
# See https://artifacthub.io/packages/tbaction/tinkerbell-community/image2disk.
set -euxo pipefail
if ! which pigz qemu-img &>/dev/null; then
	apk add --update pigz qemu-img
fi
image_url=$1
file=$2/${image_url##*/}
file=${file%.*}.raw.gz
if [[ ! -f "$file" ]]; then
	wget "$image_url" -O image.img
	qemu-img convert -O raw image.img image.raw
	pigz <image.raw >"$file"
	rm -f image.img image.raw
fi`,
				},
			}

			err = k8sClient.CreateConfigMap(ctx, ubuntuJammyConfigMap)
			if err != nil {
				return fmt.Errorf("error creating configmap: %w", err)
			}

			ubuntuJammyJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "download-ubuntu-jammy",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "download-ubuntu-jammy",
									Image:   "bash:5.2.2",
									Command: []string{"/script/entrypoint.sh"},
									Args: []string{
										"https://cloud-images.ubuntu.com/daily/server/jammy/current/jammy-server-cloudimg-amd64.img",
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
											Type: new(corev1.HostPathType), // This defaults to "DirectoryOrCreate"
										},
									},
								},
								{
									Name: "configmap-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											// Name:        "download-image",
											DefaultMode: new(int32), // 0700 in octal
										},
									},
								},
							},
						},
					},
				},
			}
			err = k8sClient.CreateJob(ctx, ubuntuJammyJob)
			if err != nil {
				return fmt.Errorf("error creating job: %w", err)
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

	cmd.Flags().StringVar(&apiKey, "apiKey", "", "api key for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&apiURL, "apiURL", "https://colony-api-virtual.konstruct.io", "api url for interacting with colony cloud (required)")
	cmd.Flags().StringVar(&loadBalancerInterface, "loadBalancerInterface", "eth0", "the local network interface for colony to use (required)")
	cmd.Flags().StringVar(&loadBalancerIP, "loadBalancerIP", "192.168.0.192", "the local network interface for colony to use (required)")

	return cmd
}

func init() {
}

func createDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(dir, 0o777)
		if err != nil {
			return fmt.Errorf("unable to create directory %q: %w", dir, err)
		}
	}
	return nil
}

func readFilesInDir(dir string) ([]string, error) {

	var templateFiles []string
	// Open the directory
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open directory: %v", err)
	}

	// Loop through each file in the directory
	for _, file := range files {
		if file.Mode().IsRegular() { // Check if it's a regular file
			filePath := filepath.Join(dir, file.Name())
			content, err := ioutil.ReadFile(filePath) // Read file content
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
			}
			templateFiles = append(templateFiles, string(content))
		}
	}

	return templateFiles, nil
}
