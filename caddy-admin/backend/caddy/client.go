package caddy

import (
	"bytes"
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

// AddRoute prepends a route to srv0's route list.
func (c *Client) AddRoute(routeJSON json.RawMessage) error {
	url := c.baseURL + "/config/apps/http/servers/srv0/routes/0"
	_, err := c.do(http.MethodPut, url, routeJSON)
	return err
}

// RemoveRoute deletes a route by its @id. 404 is treated as success.
func (c *Client) RemoveRoute(name string) error {
	url := c.baseURL + "/id/svc-" + name
	resp, err := c.do(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return nil
}

// UpsertRoute removes then adds a route for the given service.
func (c *Client) UpsertRoute(svc ServiceConfig) error {
	_ = c.RemoveRoute(svc.Name)
	route := BuildCaddyRoute(svc)
	return c.AddRoute(route)
}

func (c *Client) do(method, url string, body json.RawMessage) (*http.Response, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caddy admin api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return resp, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, string(respBody))
	}
	return resp, nil
}
