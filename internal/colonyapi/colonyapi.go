package colonyapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type API struct {
	client  *http.Client
	baseURL string
	token   string
}

// New creates a new colony API client
func New(baseURL, token string) *API {
	return &API{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				MaxIdleConnsPerHost: 100,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (a *API) ValidateApiKey(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v1/token/validate", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+a.token)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var r struct {
		IsValid bool `json:"isValid"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if !r.IsValid {
		return fmt.Errorf("invalid api key")
	}

	return nil
}

type Template struct {
	ID             string `json:"id" `
	Name           string `json:"name"`
	Label          string `json:"label"`
	IsTinkTemplate bool   `json:"isTinkTemplate"`
	IsSystem       bool   `json:"isSystem"`
	Template       string `json:"template"`
}

func (a *API) GetSystemTemplates(ctx context.Context) ([]Template, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v1/templates/all/system", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+a.token)

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer res.Body.Close()

	var templates []Template
	if err := json.NewDecoder(res.Body).Decode(&templates); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return templates, nil
}
