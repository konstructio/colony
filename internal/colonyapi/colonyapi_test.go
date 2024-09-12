package colonyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	testValidToken = "super-duper-valid-token"
)

func TestAPI_ValidateApiKey(t *testing.T) {

	t.Run("valid API key", func(t *testing.T) {
		response := map[string]interface{}{
			"isValid": true,
		}

		mockServer := createServer(t, response, validateEndpoint)

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		err := api.ValidateApiKey(context.TODO())
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}
	})

	t.Run("invalid API key", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, `{"isValid": false}`)
		}))

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		err := api.ValidateApiKey(context.TODO())
		if !errors.Is(err, errInvalidKey) {
			t.Fatalf("expected %s, but got: %s", errInvalidKey, err)
		}
	})

}

func TestAPI_GetSystemTemplates(t *testing.T) {

	t.Run("valid response", func(t *testing.T) {

		response := []Template{{
			ID:             "k1",
			Name:           "name",
			Label:          "label",
			IsTinkTemplate: true,
			IsSystem:       true,
			Template:       "template_data",
		}}

		mockServer := createServer(t, response, templateEndpoint)

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		templates, err := api.GetSystemTemplates(context.TODO())
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}

		if reflect.DeepEqual(response, templates) {
			t.Fatalf("expected %#v got %#v", response, templates)
		}
	})

	t.Run("connection reset by peer", func(t *testing.T) {

		myListener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("error creating listener %s", err)
		}
		address := myListener.Addr().String()

		go func() {

			for {
				con, err := myListener.Accept()
				if err != nil {
					t.Log(err)
				}
				con.Close()
			}
		}()

		api := New(address, testValidToken)

		_, err = api.GetSystemTemplates(context.TODO())
		if err == nil {
			t.Fatal("was expecting error but got none")
		}

	})
}

func createServer(t *testing.T, response interface{}, apiEndpoint string) *httptest.Server {

	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+apiEndpoint, func(w http.ResponseWriter, r *http.Request) {

		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", testValidToken) {
			t.Fatalf("expected to get a bearer token %s but got: %s", fmt.Sprintf("Bearer %s", testValidToken), r.Header.Get("Authorization"))
		}

		json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}
