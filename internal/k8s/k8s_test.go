package k8s

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeServer "k8s.io/client-go/kubernetes/fake"
)

func TestClient_CreateAPIKeySecret(t *testing.T) {

	t.Run("successful creation", func(tt *testing.T) {

		secretNamespace := "tink-system"
		secretName := "colony-api"
		secretKey := "api-key"
		secretValue := "super-duper-secret"

		mockServer := fakeServer.NewClientset()

		client := &Client{
			clientSet: mockServer,
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
