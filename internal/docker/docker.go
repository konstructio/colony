package docker

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/download"
	"github.com/konstructio/colony/internal/logger"
)

var ErrK3sContainerNotFound = fmt.Errorf("colony k3s container not found")

type Client struct {
	cli *client.Client
	log *logger.Logger
}

type ColonyTokens struct {
	LoadBalancerIP        string
	LoadBalancerInterface string
}

func New(logger *logger.Logger) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %w", err)
	}

	return &Client{
		cli: cli,
		log: logger,
	}, nil
}

func getColonyK3sContainer(ctx context.Context, c *Client) (*types.Container, error) {

	containers, err := c.cli.ContainerList(ctx, containerTypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("error listing containers on host: %w", err)
	}

	for _, container := range containers {
		if container.Names[0] == "/"+constants.ColonyK3sContainerName {
			return &container, nil
		}
	}
	return nil, ErrK3sContainerNotFound
}

func (c *Client) RemoveColonyK3sContainer(ctx context.Context) error {

	defer c.cli.Close()

	k3scontainer, err := getColonyK3sContainer(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting %q container: %w ", constants.ColonyK3sContainerName, err)
	}
	c.log.Info(fmt.Sprintf("found container name %q with ID %q  ", strings.TrimPrefix(k3scontainer.Names[0], "/"), k3scontainer.ID[:constants.DefaultDockerIDLength]))

	err = c.cli.ContainerStop(ctx, k3scontainer.ID, containerTypes.StopOptions{})
	if err != nil {
		return fmt.Errorf("error stopping container: %w", err)
	}
	err = c.cli.ContainerRemove(ctx, k3scontainer.ID, containerTypes.RemoveOptions{Force: true})
	if err != nil {
		return fmt.Errorf("error removing container: %w", err)
	}

	for _, mount := range k3scontainer.Mounts {
		if mount.Type == "volume" {
			if err := c.cli.VolumeRemove(ctx, mount.Name, false); err != nil {
				fmt.Printf("unable to delete volume %s", err)
			}
		}
	}

	return nil
}

func (c *Client) CreateColonyK3sContainer(ctx context.Context, loadBalancerIP, loadBalancerInterface, pwd string) error {
	log := logger.New(logger.Debug)

	// TODO  tag a new repo for permanent housing, removes templates from database
	colonyTemplateURL := "https://raw.githubusercontent.com/jarededwards/k3s-datacenter/refs/heads/main/helm/colony.yaml.tmpl"
	colonyTemplateYaml := filepath.Join(pwd, fmt.Sprintf("%s.tmpl", constants.ColonyYamlPath))

	err := download.FileFromURL(colonyTemplateURL, colonyTemplateYaml)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	} else {
		log.Info("downloaded colony.yaml successfully:", colonyTemplateYaml)
	}

	err = hydrateTemplate(pwd, ColonyTokens{
		LoadBalancerIP:        loadBalancerIP,
		LoadBalancerInterface: loadBalancerInterface,
	})

	if err != nil {
		return fmt.Errorf("error hydrating template: %w", err)
	}

	defer c.cli.Close()

	// check for an existing colony-k3s container
	k3sColonyContainer, err := getColonyK3sContainer(ctx, c)
	if k3sColonyContainer != nil {
		return fmt.Errorf("%q container already exists. please remove before continuing or run `colony destroy`", constants.ColonyK3sContainerName)
	}
	if err != nil && err != ErrK3sContainerNotFound {
		return fmt.Errorf("docker error: %w", err)
	}

	if err != nil {
		return fmt.Errorf("error getting %q container: %w ", constants.ColonyK3sContainerName, err)
	}

	// Pull the rancher/k3s image if it’s not already available
	imageName := "rancher/k3s:v1.30.2-k3s1"
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("error pulling image %q: %w ", imageName, err)
	}
	log.Infof("Pulled image %q successfully", imageName)

	defer reader.Close()
	// c.cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	io.Copy(os.Stdout, reader)

	env := []string{
		fmt.Sprintf("K3S_KUBECONFIG_OUTPUT=%s", constants.KubeconfigDockerPath),
		"K3S_KUBECONFIG_MODE=666",
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: pwd,
			Target: "/output",
		},
		{
			Type:   mount.TypeVolume,
			Source: "k3s-server",
			Target: "/var/lib/rancher/k3s",
		},
		{
			Type:   mount.TypeBind,
			Source: filepath.Join(pwd, constants.ColonyYamlPath),
			Target: fmt.Sprintf("/var/lib/rancher/k3s/server/manifests/%s", constants.ColonyYamlPath),
		},
		{
			Type:   mount.TypeTmpfs,
			Target: "/run",
		},
		{
			Type:   mount.TypeTmpfs,
			Target: "/var/run",
		},
	}

	resp, err := c.cli.ContainerCreate(ctx, &containerTypes.Config{
		Image: imageName,
		Env:   env,
		Cmd: []string{
			"server",
			"--disable=traefik,servicelb",
			"--tls-san=colony",
			"--node-label=colony.konstruct.io/node-type=colony",
		},
	}, &containerTypes.HostConfig{
		Privileged:  true,
		NetworkMode: "host",
		Mounts:      mounts,
	}, nil, nil, constants.ColonyK3sContainerName)

	if err != nil {
		log.Error("Error creating container: %w", err)
	}

	log.Infof("created container with ID %q", resp.ID)

	if err := c.cli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		panic(err)
	}

	waitInterval := 2 * time.Second
	timeout := 15 * time.Second

	log.Infof("Checking for file %s every %d...\n", filepath.Join(pwd, constants.KubeconfigHostPath), waitInterval)
	err = waitForFile(log, filepath.Join(pwd, constants.KubeconfigHostPath), waitInterval, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for kubeconfig file: %w", err)
	}

	return nil

}

func waitForFile(log *logger.Logger, filename string, interval, timeout time.Duration) error {
	timeoutCh := time.After(timeout)

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout reached while waiting for file %s", filename)
		default:
			if _, err := os.Stat(filename); err == nil {
				log.Infof("%s created file\n", filename)
				return nil
			} else if os.IsNotExist(err) {
				log.Infof("waiting for file %s...\n", filename)
				time.Sleep(interval) // Wait before checking again
			} else {
				return fmt.Errorf("error checking file: %w", err)
			}
		}
	}
}

func hydrateTemplate(pwd string, colonyTokens ColonyTokens) error {

	tmpl, err := template.ParseFiles(filepath.Join(pwd, fmt.Sprintf("%s.tmpl", constants.ColonyYamlPath)))
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	outputFile, err := os.Create(filepath.Join(pwd, constants.ColonyYamlPath))
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outputFile.Close()

	err = tmpl.Execute(outputFile, &ColonyTokens{
		LoadBalancerIP:        colonyTokens.LoadBalancerIP,
		LoadBalancerInterface: colonyTokens.LoadBalancerInterface,
	})
	if err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	return nil
}
