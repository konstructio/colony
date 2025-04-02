package colony

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type API struct {
	client  *http.Client
	baseURL string
	token   string
}

var ErrDataCenterAlreadyRegistered = errors.New("data center already has an agent registered")

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
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

type RegisterAgentRequest struct {
	DataCenterID string `json:"datacenter_id"`
	State        []byte `json:"state"`
}

type RegisterAgentResponse struct {
	Data Agent `json:"data"`
}

type Agent struct {
	ID string `json:"id"`
}

func (a *API) RegisterAgent(ctx context.Context, dataCenterID string) (*Agent, error) {
	registerAgentRequest := RegisterAgentRequest{
		DataCenterID: dataCenterID,
		State:        []byte(`{"status": "initializing"}`),
	}

	body, err := json.Marshal(registerAgentRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling register agent request: %w", err)
	}

	registerAgentEndpoint := fmt.Sprintf("%s/api/v1/agents", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerAgentEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+a.token)

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusConflict {
		return nil, ErrDataCenterAlreadyRegistered
	}

	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var resp RegisterAgentResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &resp.Data, nil
}

type HeartbeatRequest struct {
	State []byte `json:"state"`
}

func (a *API) Heartbeat(ctx context.Context, agentID string) error {
	initialState := HeartbeatRequest{
		State: []byte(`{"status": "initializing"}`),
	}

	body, err := json.Marshal(initialState)
	if err != nil {
		return fmt.Errorf("error marshalling initial state: %w", err)
	}

	heartbeatEndpoint := fmt.Sprintf("%s/api/v1/agents/%s/heartbeat", a.baseURL, agentID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, heartbeatEndpoint, bytes.NewReader(body))
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

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}
