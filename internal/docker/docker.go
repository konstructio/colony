package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/konstructio/colony/internal/constants"
	"github.com/konstructio/colony/internal/logger"
)

type Client struct {
	cli *client.Client
	log *logger.Logger
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

// func getColonyK3sContainerIDAndName(ctx context.Context, c *Client) {
func getColonyK3sContainerIDAndName(ctx context.Context, c *Client) (string, string, error) {

	containers, err := c.cli.ContainerList(ctx, containerTypes.ListOptions{All: true})
	if err != nil {
		return "", "", fmt.Errorf("error listing containers on host: %w", err)
	}

	for _, container := range containers {
		if container.Names[0] == "/"+constants.ColonyK3sContainerName {
			return container.ID, container.Names[0], nil
		}
	}
	return "", "", fmt.Errorf("container %q not found", constants.ColonyK3sContainerName)
}

func (c *Client) RemoveColonyK3sContainer(ctx context.Context) error {

	defer c.cli.Close()

	colonyK3sContainerID, colonyK3sContainerName, err := getColonyK3sContainerIDAndName(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting %q container: %w ", constants.ColonyK3sContainerName, err)
	}
	c.log.Info(fmt.Sprintf("found container name %s with ID %s  ", strings.TrimPrefix(colonyK3sContainerName, "/"), colonyK3sContainerID[:constants.DefaultDockerIDLength]))

	err = c.cli.ContainerRemove(ctx, colonyK3sContainerID, containerTypes.RemoveOptions{Force: true})
	if err != nil {
		return fmt.Errorf("error removing container: %w ", err)
	}
	return nil
}

func (c *Client) CreateColonyK3sContainer(ctx context.Context) error {
	log := logger.New(logger.Debug)

	defer c.cli.Close()

	// check for an existing colony-k3s container
	colonyK3sContainerID, _, err := getColonyK3sContainerIDAndName(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting %q container: %w ", constants.ColonyK3sContainerName, err)
	}
	if colonyK3sContainerID != "" {
		return fmt.Errorf("%q container already exists. please remove before continuing or run `colony destroy`", constants.ColonyK3sContainerName)
	}

	// Pull the rancher/k3s image if it’s not already available
	imageName := "rancher/k3s:v1.30.2-k3s1"
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("error pulling image %s: %w ", imageName, err)
	}
	fmt.Printf("Pulled image %s successfully\n", imageName)

	defer reader.Close()
	// c.cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	io.Copy(os.Stdout, reader)

	env := []string{
		fmt.Sprintf("K3S_KUBECONFIG_OUTPUT=%s", constants.KubeconfigDockerPath),
		"K3S_KUBECONFIG_MODE=666",
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w ", err)
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
			Source: fmt.Sprintf("%s/laptop/k3s-bootstrap/colony.yaml", pwd),
			Target: "/var/lib/rancher/k3s/server/manifests/colony.yaml",
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
		log.Error("Error creating container: %w ", err)
	}

	fmt.Printf("Created container with ID %s\n", resp.ID)

	if err := c.cli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		panic(err)
	}

	waitInterval := 2 * time.Second

	fmt.Printf("Checking for file %s every %v...\n", fmt.Sprintf("./%s", constants.KubeconfigHostPath), waitInterval)
	waitForKubeconfig(fmt.Sprintf("./%s", constants.KubeconfigHostPath), waitInterval)

	return nil

}

func waitForKubeconfig(filename string, interval time.Duration) {
	for {
		if _, err := os.Stat(filename); err == nil {
			fmt.Printf("%s created for cluster connectivity\n", filename)
			break
		} else if os.IsNotExist(err) {
			fmt.Printf("waiting for file %s...\n", filename)
			time.Sleep(interval) // Wait before checking again
		} else {
			fmt.Printf("error checking file: %v\n", err)
			break
		}
	}
}
