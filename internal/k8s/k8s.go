package k8s

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/konstructio/colony/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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
		return nil, fmt.Errorf("unable to locate kubeconfig file - checked path: %q", kubeConfig)
	}

	// Create clientset, which is used to run operations against the API
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	dynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	discovery, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		return nil, fmt.Errorf("error getting API group resources: %w", err)
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

	c.logger.Debugf("created Secret %q in Namespace %q", s.Name, s.Namespace)

	return nil
}

func (c *Client) PatchClusterRole(ctx context.Context, clusterRoleName string, clusterRolePatchBytes []byte) error {
	updatedRole, err := c.clientSet.RbacV1().ClusterRoles().Patch(
		ctx,
		clusterRoleName,
		types.JSONPatchType,
		clusterRolePatchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("error patching ClusterRole: %w", err)
	}
	c.logger.Infof("successfully patched ClusterRole %s", updatedRole.Name)

	return nil
}

func (c *Client) CreateSecret(ctx context.Context, secret *v1.Secret) error {
	s, err := c.clientSet.CoreV1().Secrets(secret.GetNamespace()).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating secret: %w", err)
	}

	c.logger.Debugf("created Secret %q in Namespace %q", s.Name, s.Namespace)

	return nil
}

func (c *Client) CreateConfigMap(ctx context.Context, configMap *v1.ConfigMap) error {
	_, err := c.clientSet.CoreV1().ConfigMaps(configMap.GetNamespace()).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating ConfigMap: %w", err)
	}

	c.logger.Infof("ConfigMap %s created successfully in namespace %s", configMap.Name, configMap.Namespace)

	return nil
}

func (c *Client) CreateJob(ctx context.Context, job *batchv1.Job) error {
	job, err := c.clientSet.BatchV1().Jobs(job.GetNamespace()).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating Job: %w", err)
	}

	c.logger.Infof("job %s created successfully", job.Name)

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
			return fmt.Errorf("unable to map manifest to a Kubernetes resource: %w", err)
		}

		// Find the preferred version mapping
		gvr := mapping.Resource

		// Create the resource
		_, err = c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			if k8sErrors.IsAlreadyExists(err) {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					existingObj, getErr := c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
					if getErr != nil {
						return fmt.Errorf("error getting existing resource: %w", getErr)
					}

					obj.SetResourceVersion(existingObj.GetResourceVersion())
					_, err := c.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Update(ctx, &obj, metav1.UpdateOptions{})
					if err != nil {
						return fmt.Errorf("error updating resource: %w", err)
					}

					return nil
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

// WaitForDeploymentReady waits for a target Deployment to become ready
func (c *Client) WaitForDeploymentReady(ctx context.Context, deployment *appsv1.Deployment, timeoutSeconds int) (bool, error) {
	deploymentName := deployment.Name
	namespace := deployment.Namespace

	// Get the desired number of replicas from the deployment spec
	if deployment.Spec.Replicas == nil {
		return false, fmt.Errorf("deployment %s in Namespace %s has nil Spec.Replicas", deploymentName, namespace)
	}
	desiredReplicas := *deployment.Spec.Replicas

	c.logger.Infof("waiting for deployment %q in namespace %q to be ready - this could take up to %d seconds", deploymentName, namespace, timeoutSeconds)

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		// Get the latest Deployment object
		currentDeployment, err := c.clientSet.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			// If we couldn't connect, retry
			if isNetworkingError(err) {
				c.logger.Warn("connection error, retrying: %s", err)
				return false, nil
			}

			return false, fmt.Errorf("error listing statefulsets: %w", err)
		}

		if currentDeployment.Status.ReadyReplicas == desiredReplicas {
			c.logger.Infof("all pods in deployment %q are ready", deploymentName)
			return true, nil
		}

		// Deployment is not yet ready, continue polling
		return false, nil
	})
	if err != nil {
		return false, fmt.Errorf("the Deployment %q in Namespace %q was not ready within the timeout period: %w", deploymentName, namespace, err)
	}

	return true, nil
}

// isNetworkingError checks if the error is a networking error
// that could be due to the cluster not being ready yet. It's the
// responsibility of the caller to decide if these errors are fatal
// or if they should be retried.
func isNetworkingError(err error) bool {
	// Check if the error is a networking error, which could be
	// when the cluster is starting up or when the network pieces
	// aren't yet ready
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// Check if the error is a timeout error
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func (c *Client) ReturnDeploymentObject(ctx context.Context, matchLabel string, matchLabelValue string, namespace string, timeoutSeconds int) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment

	err := wait.PollUntilContextTimeout(ctx, 15*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		deployments, err := c.clientSet.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", matchLabel, matchLabelValue),
		})
		if err != nil {
			// if we couldn't connect, ask to try again
			if isNetworkingError(err) {
				return false, nil
			}

			// if we got an error, return it
			return false, fmt.Errorf("error getting Deployment: %w", err)
		}

		// if we couldn't find any deployments, ask to try again
		if len(deployments.Items) == 0 {
			return false, nil
		}

		// fetch the first item from the list matching the labels
		deployment = &deployments.Items[0]

		// Check if it has at least one replica, if not, ask to try again
		if deployment.Status.Replicas == 0 {
			return false, nil
		}

		// if we found a deployment, return it
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for Deployment: %w", err)
	}
	return deployment, nil
}
