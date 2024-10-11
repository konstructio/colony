package k8s

import (
	"context"
	"testing"

	"github.com/konstructio/colony/internal/logger"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	fakeServer "k8s.io/client-go/kubernetes/fake"
)

func TestClient_CreateAPIKeySecret(t *testing.T) {
	t.Run("successful creation", func(tt *testing.T) {
		var (
			secretNamespace = "tink-system"
			secretName      = "colony-api"
			secretKey       = "api-key"
			secretValue     = "super-duper-secret"

			mockServer = fakeServer.NewClientset()
		)

		client := &Client{
			clientSet: mockServer,
			logger:    logger.NOOPLogger,
		}

		ctx := context.TODO()

		err := client.CreateAPIKeySecret(ctx, secretValue)
		if err != nil {
			tt.Fatalf("not expecting an error but got : %s", err)
		}

		// check secret exists
		secret, err := client.clientSet.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				tt.Fatalf("can't find the secret %q", secretName)
			}
			tt.Fatalf("not expecting an error got %s", err)
		}

		data, found := secret.Data[secretKey]
		if !found {
			tt.Fatalf("can't find key %q in secret: %q", secretKey, secretName)
		}

		if string(data) != secretValue {
			tt.Fatalf("expected key value %q but got %q", secretValue, string(data))
		}
	})
}

func Test_ApplyManifests(t *testing.T) {
	t.Run("successful creation", func(tt *testing.T) {
		const rbacRoleYAML = `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: test`

		// Create a fake dynamic client with an empty scheme set
		scheme := runtime.NewScheme()

		// Add the networkingv1 API group to the scheme
		if err := networkingv1.AddToScheme(scheme); err != nil {
			tt.Fatalf("error adding networkingv1 to scheme: %s", err)
		}

		// Create a fake dynamic client with the scheme
		dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

		// Teach the local fake API server about the NetworkPolicy resource
		// (it's not enough to just add it to the scheme, we also need to teach the server about it)
		restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{networkingv1.SchemeGroupVersion})
		restMapper.AddSpecific(
			networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"),
			networkingv1.SchemeGroupVersion.WithResource("networkpolicies"),
			networkingv1.SchemeGroupVersion.WithResource("networkpolicies"),
			meta.RESTScopeNamespace,
		)

		client := &Client{
			dynamic:    dynamicClient, // Use the fake dynamic client
			restmapper: restMapper,    // Use the fake REST mapper
			logger:     logger.NOOPLogger,
		}

		ctx := context.TODO()

		err := client.ApplyManifests(ctx, []string{rbacRoleYAML})
		if err != nil {
			tt.Fatalf("not expecting an error but got : %s", err)
		}

		_, err = client.dynamic.Resource(schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "networkpolicies",
		}).Namespace("default").Get(ctx, "test-network-policy", v1.GetOptions{})
		if err != nil {
			tt.Fatalf("not expecting an error got %s", err)
		}
	})
}
