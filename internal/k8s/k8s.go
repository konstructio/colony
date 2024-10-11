package k8s

import (
	"context"
	"fmt"

	"github.com/konstructio/colony/internal/logger"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

type Client struct {
	clientSet  kubernetes.Interface
	dynamic    dynamic.Interface
	restmapper meta.RESTMapper
	config     *rest.Config
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

	discovery, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %s", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		return nil, fmt.Errorf("error getting API group resources: %s", err)
	}

	restmapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	return &Client{
		clientSet:  clientset,
		dynamic:    dynamic,
		restmapper: restmapper,
		config:     config,
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
	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	for _, manifest := range manifests {
		var obj unstructured.Unstructured
		_, gvk, err := decoderUnstructured.Decode([]byte(manifest), nil, &obj)
		if err != nil {
			return fmt.Errorf("error decoding manifest: %w", err)
		}

		// Set the appropriate GVK
		obj.SetGroupVersionKind(*gvk)

		// Use the restmapper to get the GVR
		mapping, err := c.restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("error getting rest mapping: %w", err)
		}

		// Find the preferred version mapping
		gvr := mapping.Resource

		// Create the resource
		_, err = c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					existingObj, getErr := c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
					if getErr != nil {
						return getErr
					}
					obj.SetResourceVersion(existingObj.GetResourceVersion())
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
