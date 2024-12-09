package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	tinkv1alpha1 "github.com/kubefirst/tink/api/v1alpha1"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"
)

type IPMIAuth struct {
	HardwareID   string
	IP           string
	Password     string
	Username     string
	InsecureTLS  bool
	AutoDiscover bool
}

type RufioJobOffPXEOn struct {
	RandomSuffix string
	IP           string
}

func getAddIPMICommand() *cobra.Command {
	var ip, username, password string
	var autoDiscover, insecureTLS bool
	var templates []string

	getAddIPMICmd := &cobra.Command{
		Use:   "add-ipmi",
		Short: "adds an IPMI auth to the cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := logger.New(logger.Debug)

			ctx := cmd.Context()

			log.Infof("adding ipmi information for host %q - auto discovery %t", ip, autoDiscover)

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error getting user home directory: %w", err)
			}

			fileTypes := []string{"machine", "secret"}

			for _, t := range fileTypes {
				file, err := manifests.IPMI.ReadFile(fmt.Sprintf("ipmi/ipmi-%s.yaml.tmpl", t))
				if err != nil {
					return fmt.Errorf("error reading templates file: %w", err)
				}

				tmpl, err := template.New("ipmi").Funcs(template.FuncMap{
					"base64Encode": func(s string) string {
						return base64.StdEncoding.EncodeToString([]byte(s))
					},
					"replaceDotsWithDash": func(s string) string {
						return strings.ReplaceAll(s, ".", "-")
					},
				}).Parse(string(file))
				if err != nil {
					return fmt.Errorf("error parsing template: %w", err)
				}

				var outputBuffer bytes.Buffer

				err = tmpl.Execute(&outputBuffer, IPMIAuth{
					IP:           ip,
					Username:     username,
					Password:     password,
					InsecureTLS:  insecureTLS,
					AutoDiscover: autoDiscover,
				})
				if err != nil {
					return fmt.Errorf("error executing template: %w", err)
				}
				templates = append(templates, outputBuffer.String())
			}

			k8sClient, err := k8s.New(log, filepath.Join(homeDir, constants.ColonyDir, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			if err = k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
			}

			// Create a channel to receive the hardware object
			hardwareChan := make(chan *tinkv1alpha1.Hardware, 1)
			errChan := make(chan error, 1)

			go func() {
				log.Infof("starting informer for hardware creation")
				err := k8sClient.HardwareInformer(ctx, ip, hardwareChan)
				if err != nil {
					errChan <- fmt.Errorf("error watching hardware creation: %w", err)
				}
			}()

			if err := k8sClient.ApplyManifests(ctx, templates); err != nil {
				return fmt.Errorf("error applying templates: %w", err)
			}

			err = k8sClient.FetchAndWaitForMachines(ctx, k8s.MachineDetails{
				Name:        strings.ReplaceAll(ip, ".", "-"),
				Namespace:   constants.ColonyNamespace,
				WaitTimeout: 90,
			})
			if err != nil {
				return fmt.Errorf("error get machine: %w", err)
			}

			log.Infof("machine is ready")

			if autoDiscover {

				file, err := manifests.IPMI.ReadFile("ipmi/ipmi-off-pxe-on.yaml.tmpl")
				if err != nil {
					return fmt.Errorf("error reading templates file: %w", err)
				}

				tmpl, err := template.New("ipmi").Funcs(template.FuncMap{
					"replaceDotsWithDash": func(s string) string {
						return strings.ReplaceAll(s, ".", "-")
					},
				}).Parse(string(file))
				if err != nil {
					return fmt.Errorf("error parsing template: %w", err)
				}

				var outputBuffer2 bytes.Buffer

				err = tmpl.Execute(&outputBuffer2, RufioJobOffPXEOn{
					IP: ip,
				})
				if err != nil {
					return fmt.Errorf("error executing template: %w", err)
				}

				if err := k8sClient.ApplyManifests(ctx, []string{outputBuffer2.String()}); err != nil {
					return fmt.Errorf("error applying rufiojob: %w", err)
				}

				err = k8sClient.FetchAndWaitForRufioJobs(ctx, k8s.JobDetails{
					Name:        strings.ReplaceAll(ip, ".", "-"),
					Namespace:   constants.ColonyNamespace,
					WaitTimeout: 300,
				})

				if err != nil {
					return fmt.Errorf("error get machine: %w", err)
				}
				// Wait for hardware or error
				select {
				case hardware := <-hardwareChan:

					log.Infof("added ipmi connectivity for %q", ip)
					log.Infof("associated colony hardware id: %q", hardware.Name)

				case err := <-errChan:
					log.Errorf("Error: %v", err)

				case <-ctx.Done():
					log.Info("Context cancelled")

				case <-time.After(5 * time.Minute):
					log.Info("Timeout waiting for hardware creation")
				}

			}

			return nil
		},
	}
	getAddIPMICmd.Flags().BoolVar(&autoDiscover, "auto-discover", false, "whether to auto-discover the machine note: this power cycles the machines")
	getAddIPMICmd.Flags().BoolVar(&insecureTLS, "insecure", true, "the ipmi insecure tls")
	getAddIPMICmd.Flags().StringVar(&ip, "ip", "", "the ipmi ip address")
	getAddIPMICmd.Flags().StringVar(&password, "password", "", "the ipmi password")
	getAddIPMICmd.Flags().StringVar(&username, "username", "admin", "the ipmi username")

	// getAddIPMICmd.MarkFlagRequired("hardware-id")
	getAddIPMICmd.MarkFlagRequired("ip")
	getAddIPMICmd.MarkFlagRequired("password")
	getAddIPMICmd.MarkFlagRequired("username")

	return getAddIPMICmd
}
