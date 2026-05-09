// Package httpclient is the live HTTP adapter for the internal users service.
//
// It speaks GET {base}/api/users/{id} with X-Internal-API-Key, applies a request
// timeout, and maps non-2xx responses to typed errors from port/internalsvc.
package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
)

const apiKeyHeader = "X-Internal-API-Key"

// Config bundles the adapter's configuration. APIKey is loaded from env, never logged.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// Client implements internalsvc.InternalService over HTTP.
type Client struct {
	baseURL *url.URL
	apiKey  string
	httpc   *http.Client
}

func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("internalsvc httpclient: base url required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("internalsvc httpclient: api key required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("internalsvc httpclient: parse base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: u,
		apiKey:  cfg.APIKey,
		httpc:   &http.Client{Timeout: timeout},
	}, nil
}

type userResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	License string `json:"license"`
	Role    string `json:"role"`
}

func (c *Client) GetUser(ctx context.Context, userID string) (profile.User, error) {
	u := *c.baseURL
	u.Path = path.Join(u.Path, "/api/users", url.PathEscape(userID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return profile.User{}, fmt.Errorf("internalsvc httpclient: new request: %w", err)
	}
	req.Header.Set(apiKeyHeader, c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return profile.User{}, fmt.Errorf("internalsvc httpclient: do: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return profile.User{}, internalsvc.ErrUserNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return profile.User{}, internalsvc.ErrUnauthorized
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return profile.User{}, fmt.Errorf("internalsvc httpclient: status %d: %s", resp.StatusCode, string(body))
	}

	var r userResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return profile.User{}, fmt.Errorf("internalsvc httpclient: decode: %w", err)
	}
	return profile.User{
		ID:          r.ID,
		DisplayName: r.Name,
		License:     r.License,
		Role:        r.Role,
	}, nil
}

var _ internalsvc.InternalService = (*Client)(nil)
