package caddy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client queries the Caddy Admin API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Caddy Admin API client.
// adminAddr is e.g. "localhost:2019" or "caddy:2019"
func NewClient(adminAddr string) *Client {
	return &Client{
		baseURL: "http://" + adminAddr,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetConfig fetches the full Caddy config from /config/
func (c *Client) GetConfig() (*CaddyConfig, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/config/")
	if err != nil {
		return nil, fmt.Errorf("caddy admin api unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cfg CaddyConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// IsRunning returns true if Caddy admin API is reachable
func (c *Client) IsRunning() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/config/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
