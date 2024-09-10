package colony

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createServer(t *testing.T, path string, fn http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(path, fn)
	return httptest.NewServer(mux)
}

func Test_ValidateAPIKey(t *testing.T) {
	t.Run("full request flow", func(t *testing.T) {
		fakeToken := "my-super-secret"

		srv := createServer(t, validateAPIKeyURL, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+fakeToken {
				t.Fatalf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"isValid": true}`))
		})
		defer srv.Close()

		ctx := context.Background()
		api := New(srv.URL, fakeToken)

		if err := api.ValidateAPIKey(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("malformed URL", func(t *testing.T) {
		baseURL := ":"
		api := New(baseURL, "token")

		err := api.ValidateAPIKey(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unable to connect", func(t *testing.T) {
		srv := httptest.NewServer(nil)
		srv.Close()

		api := New(srv.URL, "token")

		err := api.ValidateAPIKey(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid status code", func(t *testing.T) {
		srv := createServer(t, validateAPIKeyURL, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		defer srv.Close()

		api := New(srv.URL, "token")

		err := api.ValidateAPIKey(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid response body", func(t *testing.T) {
		srv := createServer(t, validateAPIKeyURL, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not JSON`))
		})
		defer srv.Close()

		api := New(srv.URL, "token")

		err := api.ValidateAPIKey(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid API key", func(t *testing.T) {
		srv := createServer(t, validateAPIKeyURL, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"isValid": false}`))
		})
		defer srv.Close()

		api := New(srv.URL, "token")

		err := api.ValidateAPIKey(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
