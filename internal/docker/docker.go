package docker

import (
	"context"
	"errors"
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

var ErrK3sContainerNotFound = errors.New("colony k3s container not found")

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

func (c *Client) Close() error {
	return c.cli.Close() //nolint:wrapcheck // exposing the close to upstream callers
}

func (c *Client) getColonyK3sContainer(ctx context.Context) (*types.Container, error) {
	containers, err := c.cli.ContainerList(ctx, containerTypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("error listing containers on host: %w", err)
	}

	for _, container := range containers {
		if len(container.Names) > 0 && container.Names[0] == "/"+constants.ColonyK3sContainerName {
			return &container, nil
		}
	}
	return nil, ErrK3sContainerNotFound
}

func (c *Client) RemoveColonyK3sContainer(ctx context.Context) error {
	k3scontainer, err := c.getColonyK3sContainer(ctx)
	if err != nil {
		return fmt.Errorf("error getting %q container: %w", constants.ColonyK3sContainerName, err)
	}

	if len(k3scontainer.Names) > 0 && len(k3scontainer.ID) > constants.DefaultDockerIDLength {
		c.log.Infof("found container name %q with ID %q", strings.TrimPrefix(k3scontainer.Names[0], "/"), k3scontainer.ID[:constants.DefaultDockerIDLength])
	} else {
		c.log.Warnf("found container with ID %q -- unable to parse a name or an ID out of it", k3scontainer.ID)
	}

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
				c.log.Warnf("error removing volume %q: %v, continuing...", mount.Name, err)
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
	}

	log.Infof("downloaded colony.yaml successfully to %q", colonyTemplateYaml)

	err = hydrateTemplate(pwd, ColonyTokens{
		LoadBalancerIP:        loadBalancerIP,
		LoadBalancerInterface: loadBalancerInterface,
	})
	if err != nil {
		return fmt.Errorf("error hydrating template: %w", err)
	}

	// check for an existing colony-k3s container
	k3sColonyContainer, err := c.getColonyK3sContainer(ctx)
	if k3sColonyContainer != nil {
		return fmt.Errorf("%q container already exists. please remove before continuing or run `colony destroy`", constants.ColonyK3sContainerName)
	}

	if err != nil && !errors.Is(err, ErrK3sContainerNotFound) {
		return fmt.Errorf("docker error: %w", err)
	}

	// Pull the rancher/k3s image if it's not already available
	imageName := "rancher/k3s:v1.30.2-k3s1"
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("error pulling image %q: %w", imageName, err)
	}
	defer reader.Close()

	log.Infof("pulled image %q successfully", imageName)

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
		return fmt.Errorf("error creating container: %w", err)
	}

	log.Infof("created container with ID %q", resp.ID)

	if err := c.cli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		return fmt.Errorf("error starting container: %w", err)
	}

	waitInterval := 2 * time.Second
	timeout := 15 * time.Second

	log.Infof("Checking for file %s every %d...", filepath.Join(pwd, constants.KubeconfigHostPath), waitInterval.Seconds())

	err = waitUntilFileExists(log, filepath.Join(pwd, constants.KubeconfigHostPath), waitInterval, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for kubeconfig file: %w", err)
	}

	return nil
}

func waitUntilFileExists(log *logger.Logger, filename string, interval, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout reached while waiting for file %s", filename)
		case <-ticker.C:
			if _, err := os.Stat(filename); err != nil {
				if os.IsNotExist(err) {
					log.Infof("waiting for file %q...", filename)
					continue
				}

				return fmt.Errorf("error checking file: %w", err)
			}

			log.Infof("found and stat'd file %q", filename)
			return nil
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
