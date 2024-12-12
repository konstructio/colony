package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/internal/utils"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeprovisionWorkflowRequest struct {
	Mac          string
	RandomSuffix string
}

func getDeprovisionCommand() *cobra.Command {
	var hardwareID, bootDevice string
	var efiBoot, destroy bool
	deprovisionCmd := &cobra.Command{
		Use:   "deprovision",
		Short: "remove a hardware from your colony data center - very destructive",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := logger.New(logger.Debug)

			randomSuffix := utils.RandomString(6)

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
			log.Infof("destroy %t", destroy)

			// TODO if the machine state is powered on, restart it so the workflow will run

			// todo
			//! POST to api to mark the hardware removed
			// get hardware and remove ipxe
			hw, err := k8sClient.HardwareRemoveIPXE(ctx, k8s.UpdateHardwareRequest{
				HardwareID: hardwareID,
				Namespace:  constants.ColonyNamespace,
				RemoveIpXE: true,
			})
			if err != nil {
				return fmt.Errorf("error getting hardware: %w", err)
			}
			log.Infof("hardware: %v", hw)

			//! detokenize and apply the workflow

			file, err := manifests.Workflow.ReadFile("workflow/wipe-disks.yaml.tmpl")
			if err != nil {
				return fmt.Errorf("error reading templates file: %w", err)
			}

			tmpl, err := template.New("ipmi").Funcs(template.FuncMap{
				"replaceColonsWithHyphens": func(s string) string {
					return strings.ReplaceAll(s, ":", "-")
				},
			}).Parse(string(file))
			if err != nil {
				return fmt.Errorf("error parsing template: %w", err)
			}

			var outputBuffer bytes.Buffer

			err = tmpl.Execute(&outputBuffer, DeprovisionWorkflowRequest{
				Mac:          hw.Spec.Interfaces[0].DHCP.MAC,
				RandomSuffix: randomSuffix,
			})
			if err != nil {
				return fmt.Errorf("error executing template: %w", err)
			}

			log.Info(outputBuffer.String())

			ip, err := k8sClient.GetHardwareMachineRefFromSecretLabel(ctx, constants.ColonyNamespace, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("colony.konstruct.io/hardware-id=%s", hardwareID),
			})
			if err != nil {
				return fmt.Errorf("error getting machine ref secret: %w", err)
			}

			//! NOT UNTIL WE'RE SURE
			if err := k8sClient.ApplyManifests(ctx, []string{outputBuffer.String()}); err != nil {
				return fmt.Errorf("error applying rufiojob: %w", err)
			}

			err = k8sClient.FetchAndWaitForWorkflow(ctx, k8s.WorkflowWaitRequest{
				LabelValue:   strings.ReplaceAll(ip, ".", "-"),
				Namespace:    constants.ColonyNamespace,
				WaitTimeout:  300,
				RandomSuffix: randomSuffix,
			})
			if err != nil {
				return fmt.Errorf("error waiting for workflow: %w", err)
			}

			// reboot
			file2, err := manifests.IPMI.ReadFile("ipmi/ipmi-off-pxe-on.yaml.tmpl")
			if err != nil {
				return fmt.Errorf("error reading templates file: %w", err)
			}

			tmpl2, err := template.New("ipmi").Funcs(template.FuncMap{
				"replaceDotsWithDash": func(s string) string {
					return strings.ReplaceAll(s, ".", "-")
				},
			}).Parse(string(file2))
			if err != nil {
				return fmt.Errorf("error parsing template: %w", err)
			}

			var outputBuffer2 bytes.Buffer

			err = tmpl2.Execute(&outputBuffer2, RufioPowerCycleRequest{
				IP:           ip,
				EFIBoot:      efiBoot,
				BootDevice:   bootDevice,
				RandomSuffix: randomSuffix,
			})
			if err != nil {
				return fmt.Errorf("error executing template: %w", err)
			}

			log.Info(outputBuffer2.String())

			if err := k8sClient.ApplyManifests(ctx, []string{outputBuffer2.String()}); err != nil {
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
	deprovisionCmd.Flags().StringVar(&hardwareID, "hardware-id", "", "hardware id of the server to deprovision - WARNING: you can not recover this server")
	deprovisionCmd.Flags().StringVar(&bootDevice, "boot-device", "pxe", "the bootdev to set (pxe, bios) defaults to pxe")
	deprovisionCmd.Flags().BoolVar(&efiBoot, "efiBoot", true, "boot device option (uefi, legacy) defaults to uefi")
	deprovisionCmd.Flags().BoolVar(&destroy, "destroy", false, "whether to destroy the machine and its associated resources")
	deprovisionCmd.MarkFlagRequired("hardware-id")
	return deprovisionCmd
}
