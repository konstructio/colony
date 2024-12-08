//! if the user adds an ipmi object, we should be able to create
//! the Secret and Machine object then trigger a reboot job that
//!
//!
//!
//!

package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

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

			log.Info("add-ipmi")

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

			if err := k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
			}

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

			//! we need to create a Job that will reboot the machine
			//! we need to watch for a hardware object to be created
			//! stash the hardware object id in the secret for this ipmi

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
