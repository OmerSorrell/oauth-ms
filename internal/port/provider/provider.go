// Package provider defines the OAuth-provider strategy interface.
//
// Adding a new provider means: (1) create a new package under
// internal/adapter/provider/<name>/ implementing Provider; (2) register it in
// cmd/dependencies/providers.go. No domain, use-case, or transport code changes.
package provider

import (
	"context"
	"errors"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/domain/profile"
)

// ErrUnknownProvider is returned by Registry.Get when the requested provider isn't registered.
var ErrUnknownProvider = errors.New("provider: unknown")

// Token is the access-token bag passed back into a Provider on subsequent calls.
type Token struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
	Scopes      []string
}

// Identity is what we capture at callback time.
//
// Storing it in the session lets /profile fan out without a serial GetIdentity round-trip.
type Identity struct {
	ProviderUserID string
	DisplayName    string
}

// Provider is the strategy every OAuth provider implements.
type Provider interface {
	// Name is the URL segment under /auth/{name}.
	Name() string
	// BuildAuthURL composes the provider's authorization URL with state + PKCE.
	// The provider derives the S256 code_challenge from the supplied verifier.
	BuildAuthURL(state, codeVerifier, redirectURI string) string
	// ExchangeCode swaps an authorization code (with the matching PKCE verifier) for a token.
	ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (Token, error)
	// GetIdentity returns the provider-side user identity for the given token.
	GetIdentity(ctx context.Context, t Token) (Identity, error)
	// ListResources returns the user's normalized resources (repos for GitHub, etc.).
	ListResources(ctx context.Context, t Token) ([]profile.Resource, error)
}
