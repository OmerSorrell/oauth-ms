// Package github implements port/provider.Provider against the GitHub OAuth + REST API.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
)

// Name is the URL segment under /auth/{name} this provider responds to.
const Name = "github"

const (
	authzURL    = "https://github.com/login/oauth/authorize"
	tokenURL    = "https://github.com/login/oauth/access_token"
	apiUserURL  = "https://api.github.com/user"
	apiReposURL = "https://api.github.com/user/repos"
)

// Config holds the GitHub OAuth app credentials and requested scopes.
type Config struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Provider is the GitHub strategy.
type Provider struct {
	cfg   Config
	httpc *http.Client
}

func New(cfg Config, httpc *http.Client) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("github: client_id and client_secret are required")
	}
	if httpc == nil {
		httpc = http.DefaultClient
	}
	return &Provider{cfg: cfg, httpc: httpc}, nil
}

func (p *Provider) Name() string { return Name }

// oauth2Config returns a fresh oauth2.Config bound to the given redirect URI.
// A new value per call avoids mutating shared state when the redirect varies.
func (p *Provider) oauth2Config(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: authzURL, TokenURL: tokenURL},
		RedirectURL:  redirectURI,
		Scopes:       p.cfg.Scopes,
	}
}

func (p *Provider) BuildAuthURL(state, codeVerifier, redirectURI string) string {
	return p.oauth2Config(redirectURI).AuthCodeURL(state,
		oauth2.S256ChallengeOption(codeVerifier),
		oauth2.SetAuthURLParam("allow_signup", "true"),
	)
}

func (p *Provider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (provider.Token, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpc)
	tok, err := p.oauth2Config(redirectURI).Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return provider.Token{}, fmt.Errorf("github: token exchange: %w", err)
	}
	var scopes []string
	if scopeRaw, ok := tok.Extra("scope").(string); ok && scopeRaw != "" {
		scopes = strings.Fields(strings.ReplaceAll(scopeRaw, ",", " "))
	}
	return provider.Token{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		ExpiresAt:   tok.Expiry,
		Scopes:      scopes,
	}, nil
}

type ghUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

func (p *Provider) GetIdentity(ctx context.Context, t provider.Token) (provider.Identity, error) {
	var u ghUser
	if err := p.apiGet(ctx, t, apiUserURL, &u); err != nil {
		return provider.Identity{}, err
	}
	name := u.Name
	if name == "" {
		name = u.Login
	}
	return provider.Identity{
		ProviderUserID: strconv.FormatInt(u.ID, 10),
		DisplayName:    name,
	}, nil
}

type ghRepo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stargazers  int64  `json:"stargazers_count"`
}

func (p *Provider) ListResources(ctx context.Context, t provider.Token) ([]profile.Resource, error) {
	// First page only for the skeleton; pagination is a follow-up.
	u := apiReposURL + "?per_page=100&sort=updated&type=owner"
	var repos []ghRepo
	if err := p.apiGet(ctx, t, u, &repos); err != nil {
		return nil, err
	}
	out := make([]profile.Resource, 0, len(repos))
	for _, r := range repos {
		out = append(out, profile.Resource{
			Name:        r.Name,
			Description: r.Description,
			Metric:      r.Stargazers,
			MetricLabel: "stars",
		})
	}
	return out, nil
}

func (p *Provider) apiGet(ctx context.Context, t provider.Token, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("github api: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("github api: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github api %s: status %d: %s", u, resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("github api %s: decode: %w", u, err)
	}
	return nil
}

var _ provider.Provider = (*Provider)(nil)
