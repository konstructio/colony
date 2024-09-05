package k8s

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
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
	"sigs.k8s.io/yaml"
)

type Client struct {
	clientSet  *kubernetes.Clientset
	dynamic    *dynamic.DynamicClient
	config     *rest.Config
	NameSpace  string
	SecretName string
}

func New(kubeConfig string) (*Client, error) {
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

	s, err := c.clientSet.CoreV1().Secrets(c.NameSpace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating secret: %w", err)
	}

	log.Infof("created Secret %s in Namespace %s\n", s.Name, s.Namespace)

	return nil
}

func (c *Client) ApplyManifests(ctx context.Context, manifests []string) error {
	for _, manifest := range manifests {
		// Convert YAML to JSON
		jsonBytes, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			return fmt.Errorf("error converting YAML to JSON: %w", err)
		}

		u := &unstructured.Unstructured{}
		if err = u.UnmarshalJSON(jsonBytes); err != nil {
			return fmt.Errorf("error unmarshalling JSON: %w", err)
		}

		// Apply the object to the cluster
		gvr := schema.GroupVersionResource{
			Group:    u.GroupVersionKind().Group,
			Version:  u.GroupVersionKind().Version,
			Resource: strings.ToLower(u.GetKind()) + "s", // Convert kind to plural form
		}

		if _, err := c.dynamic.Resource(gvr).Namespace(c.NameSpace).Create(ctx, u, metav1.CreateOptions{}); err != nil {
			if errors.IsAlreadyExists(err) {
				// If the resource already exists, update it
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					_, err := c.dynamic.Resource(gvr).Namespace(u.GetNamespace()).Update(context.Background(), u, metav1.UpdateOptions{})
					return err
				})

				if retryErr != nil {
					return fmt.Errorf("update failed: %w", retryErr)
				}
			}
			return fmt.Errorf("error creating object: %w", err)
		}
	}

	return nil
}
