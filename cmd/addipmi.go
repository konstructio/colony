package cmd

import (
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/exec"
	"github.com/konstructio/colony/internal/k8s"
	"github.com/konstructio/colony/internal/logger"
	"github.com/konstructio/colony/manifests"
	"github.com/spf13/cobra"
)

type IPMIAuth struct {
	HardwareID  string
	IP          string
	Password    string
	Username    string
	InsecureTLS bool
}

func getAddIPMICommand() *cobra.Command {
	var ipmiAuthFile string
	var templateFiles []string

	getAddIPMICmd := &cobra.Command{
		Use:   "add-ipmi",
		Short: "adds an IPMI auth to the cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := logger.New(logger.Debug)

			ctx := cmd.Context()

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error getting user home directory: %w", err)
			}

			err = exec.CreateDirIfNotExist(filepath.Join(homeDir, constants.ColonyDir, "ipmi"))
			if err != nil {
				return fmt.Errorf("error creating directory templates: %w", err)
			}

			ipmiEntries, err := parseCSV(ipmiAuthFile)
			if err != nil {
				return fmt.Errorf("failed to parse csv: %w", err)
			}

			fileTypes := []string{"machine", "secret"}

			for _, entry := range ipmiEntries {
				log.Infof("found entry for host ip: %q\n", entry.IP)

				for _, t := range fileTypes {
					file, err := manifests.IPMI.ReadFile(fmt.Sprintf("ipmi/ipmi-%s.yaml.tmpl", t))
					if err != nil {
						return fmt.Errorf("error reading templates file: %w", err)
					}

					tmpl, err := template.New("ipmi").Funcs(template.FuncMap{
						"dotsToHyphens": func(s string) string {
							return strings.ReplaceAll(s, ".", "-")
						},
						"base64Encode": func(s string) string {
							return base64.StdEncoding.EncodeToString([]byte(s))
						},
					}).Parse(string(file))
					if err != nil {
						return fmt.Errorf("error parsing template: %w", err)
					}

					outputFile, err := os.Create(filepath.Join(homeDir, constants.ColonyDir, "ipmi", fmt.Sprintf("%s-%s.yaml", strings.ReplaceAll(entry.IP, ".", "-"), t)))
					if err != nil {
						return fmt.Errorf("error creating output file: %w", err)
					}
					defer outputFile.Close()

					err = tmpl.Execute(outputFile, entry)
					if err != nil {
						return fmt.Errorf("error executing template: %w", err)
					}
				}
			}

			files, err := os.ReadDir(filepath.Join(homeDir, constants.ColonyDir, "ipmi"))
			if err != nil {
				return fmt.Errorf("failed to open directory: %w", err)
			}

			for _, file := range files {
				content, err := os.ReadFile(filepath.Join(homeDir, constants.ColonyDir, "ipmi", file.Name()))
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				templateFiles = append(templateFiles, string(content))
			}

			k8sClient, err := k8s.New(log, filepath.Join(homeDir, constants.ColonyDir, constants.KubeconfigHostPath))
			if err != nil {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			if err := k8sClient.LoadMappingsFromKubernetes(); err != nil {
				return fmt.Errorf("error loading dynamic mappings from kubernetes: %w", err)
			}

			if err := k8sClient.ApplyManifests(ctx, templateFiles); err != nil {
				return fmt.Errorf("error applying templates: %w", err)
			}

			return nil
		},
	}
	getAddIPMICmd.Flags().StringVar(&ipmiAuthFile, "ipmi-auth-file", "", "path to csv file containging IPMI auth data")

	getAddIPMICmd.MarkFlagRequired("ipmi-auth-file")

	return getAddIPMICmd
}

func parseCSV(filename string) ([]IPMIAuth, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var ipmiEntries []IPMIAuth
	for _, record := range records {
		enabled, _ := strconv.ParseBool(record[4])
		entry := IPMIAuth{
			HardwareID:  record[0],
			IP:          record[1],
			Username:    record[2],
			Password:    record[3],
			InsecureTLS: enabled,
		}
		ipmiEntries = append(ipmiEntries, entry)
	}

	return ipmiEntries, nil
}
