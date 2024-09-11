package colonyapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPI_ValidateApiKey(t *testing.T) {
	validToken := "super-duper-valid-token"
	apiEndpoint := "/api/v1/token/validate"

	t.Run("valid api key", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path != apiEndpoint {
				t.Fatalf("expected to request %s but got: %s", apiEndpoint, r.URL.Path)
			}

			if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", validToken) {
				t.Fatalf("expected to get a bearer token %s but got: %s", fmt.Sprintf("Bearer %s", validToken), r.Header.Get("Authorization"))
			}

			fmt.Fprintln(w, `{"isValid": true}`)
		}))

		defer mockServer.Close()

		api := New(mockServer.URL, validToken)

		err := api.ValidateApiKey(context.TODO())
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}
	})

	t.Run("invalid api key", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, `{"isValid": false}`)
		}))

		defer mockServer.Close()

		api := New(mockServer.URL, validToken)

		err := api.ValidateApiKey(context.TODO())
		if !errors.Is(err, invalidKeyError) {
			t.Fatalf("expected %s, but got: %s", invalidKeyError, err)
		}
	})

}

func TestAPI_GetSystemTemplates(t *testing.T) {
	ctx := context.TODO()
	validToken := "super-duper-valid-token"
	apiEndpoint := "/api/v1/templates/all/system"
	response := `[{"id":"k1","name":"name","label":"label","isTinkTemplate":true,"isSystem":true,"template":"template_data"}]`

	t.Run("valid response", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path != apiEndpoint {
				t.Fatalf("expected to request %s but got: %s", apiEndpoint, r.URL.Path)
			}

			if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", validToken) {
				t.Fatalf("expected to get a bearer token %s but got: %s", fmt.Sprintf("Bearer %s", validToken), r.Header.Get("Authorization"))
			}

			fmt.Fprintln(w, response)
		}))

		defer mockServer.Close()

		api := New(mockServer.URL, validToken)

		templates, err := api.GetSystemTemplates(ctx)
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}

		_ = templates
	})
}
