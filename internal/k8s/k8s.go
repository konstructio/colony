package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/konstructio/colony/internal/logger"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

type Client struct {
	clientSet  kubernetes.Interface
	dynamic    dynamic.Interface
	config     *rest.Config
	NameSpace  string
	SecretName string
	logger     *logger.Logger
}

func New(logger *logger.Logger, kubeConfig string) (*Client, error) {
	// Build configuration instance from the provided config file
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to locate kubeconfig file - checked path: %s", kubeConfig)
	}

	// Create clientset, which is used to run operations against the API
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %s", err)
	}

	dynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %s", err)
	}

	return &Client{
		// ClientSet:  k8s.CreateKubeConfig(false, "/home/vagrant/.kube/config").Clientset,
		clientSet:  clientset,
		dynamic:    dynamic,
		config:     config,
		NameSpace:  "tink-system",
		SecretName: "colony-api",
		logger:     logger,
	}, nil
}

func (c *Client) CreateAPIKeySecret(ctx context.Context, apiKey string) error {
	// Create the secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "colony-api",
			Namespace: "tink-system",
		},
		Data: map[string][]byte{
			"api-key": []byte(apiKey),
		},
	}

	s, err := c.clientSet.CoreV1().Secrets(secret.GetNamespace()).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating secret: %w", err)
	}

	c.logger.Debugf("created Secret %s in Namespace %s\n", s.Name, s.Namespace)

	return nil
}

func (c *Client) ApplyManifests(ctx context.Context, manifests []string) error {
	for _, manifest := range manifests {
		// Unmarshal the manifest into an unstructured object
		// TBD: decide if the YAML manifest would have more than one
		// resource definition, since then we need a loop
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(manifest), &obj.Object); err != nil {
			return fmt.Errorf("error unmarshalling manifest: %w", err)
		}

		// Get the GVK
		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		}

		// Create the resource
		_, err := c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					_, err := c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Update(ctx, &obj, metav1.UpdateOptions{})
					return err
				})

				if retryErr != nil {
					return fmt.Errorf("error updating resource: %w", retryErr)
				}
			}

			return fmt.Errorf("error creating resource: %w", err)
		}
	}

	return nil
}
