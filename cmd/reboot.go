package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/konstructio/colony/configs"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/internal/utils"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getRebootCommand() *cobra.Command {
	var hardwareID, bootDevice string
	var efiBoot bool
	rebootCmd := &cobra.Command{
		Use:   "reboot",
		Short: "reboots the server passed with hardware id",
		RunE: func(cmd *cobra.Command, _ []string) error {

			ctx := cmd.Context()
			log := logger.New(logger.Debug)
			log.Info("colony cli version: ", configs.Version)
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

			log.Infof("rebooting hardware with id %q", hardwareID)
			log.Infof("boot device %q", bootDevice)
			log.Infof("efi boot %t", efiBoot)

			file, err := manifests.IPMI.ReadFile("ipmi/ipmi-off-pxe-on.yaml.tmpl")
			if err != nil {
				return fmt.Errorf("error reading templates file: %w", err)
			}
			randomSuffix := utils.RandomString(6)

			tmpl, err := template.New("ipmi").Funcs(template.FuncMap{
				"replaceDotsWithDash": func(s string) string {
					return strings.ReplaceAll(s, ".", "-")
				},
			}).Parse(string(file))
			if err != nil {
				return fmt.Errorf("error parsing template: %w", err)
			}

			var outputBuffer bytes.Buffer

			ip, err := k8sClient.GetHardwareMachineRefFromSecretLabel(ctx, constants.ColonyNamespace, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("colony.konstruct.io/hardware-id=%s", hardwareID),
			})
			if err != nil {
				return fmt.Errorf("error getting machine ref: %w", err)
			}

			err = tmpl.Execute(&outputBuffer, RufioPowerCycleRequest{
				IP:           ip,
				EFIBoot:      efiBoot,
				BootDevice:   bootDevice,
				RandomSuffix: randomSuffix,
			})
			if err != nil {
				return fmt.Errorf("error executing template: %w", err)
			}

			log.Info(outputBuffer.String())

			if err := k8sClient.ApplyManifests(ctx, []string{outputBuffer.String()}); err != nil {
				return fmt.Errorf("error applying rufiojob: %w", err)
			}

			err = k8sClient.FetchAndWaitForRufioJobs(ctx, k8s.RufioJobWaitRequest{
				LabelValue:   strings.ReplaceAll(ip, ".", "-"),
				Namespace:    constants.ColonyNamespace,
				WaitTimeout:  300,
				RandomSuffix: randomSuffix,
			})

			if err != nil {
				return fmt.Errorf("error get machine: %w", err)
			}

			return nil
		},
	}

	rebootCmd.Flags().StringVar(&hardwareID, "hardware-id", "", "hardware id of the server to reboot")
	rebootCmd.Flags().StringVar(&bootDevice, "boot-device", "pxe", "the bootdev to set (pxe, bios) defaults to pxe")
	rebootCmd.Flags().BoolVar(&efiBoot, "efiBoot", true, "boot device option (uefi, legacy) defaults to uefi")
	rebootCmd.MarkFlagRequired("hardware-id")
	return rebootCmd
}
