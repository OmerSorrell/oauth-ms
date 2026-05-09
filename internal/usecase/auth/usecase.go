// Package auth contains the auth-flow use case: start the OAuth handshake and
// finish it on callback. Provider specifics live in adapter/provider/*; this
// package speaks only to ports.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/OmerSorrell/oauth-ms/internal/adapter/crypto"
	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
)

var (
	// ErrStateMismatch indicates the callback's state query param disagrees with the
	// short-lived oauth_state cookie. Defends against CSRF.
	ErrStateMismatch = errors.New("auth: state mismatch")
	// ErrProviderMismatch indicates the consumed state record was issued for a
	// different provider than the callback URL claims.
	ErrProviderMismatch = errors.New("auth: state belongs to different provider")
)

// Config bundles the auth use case's configuration.
type Config struct {
	StateTTL   time.Duration
	SessionTTL time.Duration
	// BaseURL is the connector's externally reachable origin (no trailing slash).
	// The redirect URI for provider {p} is {BaseURL}/auth/{p}/callback.
	BaseURL string
}

// UseCase implements StartFlow and HandleCallback against the configured ports.
//
// Function-typed fields (randIDFn, verifierFn, nowFn) are exposed via setters so
// tests can stub them deterministically without touching package-level state.
type UseCase struct {
	log      *slog.Logger
	registry provider.Registry
	states   store.StateStore
	sessions store.SessionStore
	cfg      Config

	randIDFn   func() (string, error)
	verifierFn func() string
	nowFn      func() time.Time
}

func New(log *slog.Logger, reg provider.Registry, states store.StateStore, sessions store.SessionStore, cfg Config) *UseCase {
	return &UseCase{
		log:        log,
		registry:   reg,
		states:     states,
		sessions:   sessions,
		cfg:        cfg,
		randIDFn:   crypto.RandomURLSafeID,
		verifierFn: oauth2.GenerateVerifier,
		nowFn:      time.Now,
	}
}

// StartFlowResult is what the HTTP handler needs to respond to GET /auth/{provider}.
type StartFlowResult struct {
	AuthorizationURL string
	State            string // handler stores this in the short-lived oauth_state cookie
}

// StartFlow validates the provider, mints state + PKCE, persists the StateRecord,
// and returns the provider's authorization URL.
func (uc *UseCase) StartFlow(ctx context.Context, providerName string) (StartFlowResult, error) {
	p, err := uc.registry.Get(providerName)
	if err != nil {
		return StartFlowResult{}, err
	}

	state, err := uc.randIDFn()
	if err != nil {
		return StartFlowResult{}, fmt.Errorf("auth.StartFlow: state: %w", err)
	}
	verifier := uc.verifierFn()

	redirectURI := uc.callbackURI(providerName)
	rec := domauth.StateRecord{
		State:        state,
		CodeVerifier: verifier,
		Provider:     providerName,
		RedirectURI:  redirectURI,
		CreatedAt:    uc.nowFn(),
	}
	if err := uc.states.Put(ctx, rec, uc.cfg.StateTTL); err != nil {
		return StartFlowResult{}, fmt.Errorf("auth.StartFlow: state put: %w", err)
	}

	return StartFlowResult{
		AuthorizationURL: p.BuildAuthURL(state, verifier, redirectURI),
		State:            state,
	}, nil
}

// CallbackInput is the handler-shaped input to HandleCallback.
type CallbackInput struct {
	Provider    string // from URL
	State       string // from query
	StateCookie string // from short-lived oauth_state cookie
	Code        string // from query
}

// CallbackResult tells the handler the new session ID to set in the cookie.
type CallbackResult struct {
	SessionID string
	Session   domauth.Session
}

// HandleCallback enforces CSRF/state consistency, exchanges the code, captures the
// provider identity, and creates a server-side session.
func (uc *UseCase) HandleCallback(ctx context.Context, in CallbackInput) (CallbackResult, error) {
	if in.State == "" || in.StateCookie == "" || in.State != in.StateCookie {
		return CallbackResult{}, ErrStateMismatch
	}

	rec, err := uc.states.Consume(ctx, in.State)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.HandleCallback: consume state: %w", err)
	}
	if rec.Provider != in.Provider {
		return CallbackResult{}, ErrProviderMismatch
	}

	p, err := uc.registry.Get(in.Provider)
	if err != nil {
		return CallbackResult{}, err
	}

	token, err := p.ExchangeCode(ctx, in.Code, rec.CodeVerifier, rec.RedirectURI)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.HandleCallback: exchange: %w", err)
	}

	identity, err := p.GetIdentity(ctx, token)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.HandleCallback: identity: %w", err)
	}

	sid, err := uc.randIDFn()
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.HandleCallback: session id: %w", err)
	}
	sess := domauth.Session{
		ID:                   sid,
		Provider:             in.Provider,
		AccessToken:          token.AccessToken,
		TokenType:            token.TokenType,
		AccessTokenExpiresAt: token.ExpiresAt,
		Scopes:               token.Scopes,
		ProviderUserID:       identity.ProviderUserID,
		DisplayName:          identity.DisplayName,
		CreatedAt:            uc.nowFn(),
	}
	if err := uc.sessions.Put(ctx, sess, uc.cfg.SessionTTL); err != nil {
		return CallbackResult{}, fmt.Errorf("auth.HandleCallback: session put: %w", err)
	}
	return CallbackResult{SessionID: sid, Session: sess}, nil
}

// Logout deletes the server-side session record. The handler is also expected to
// expire the cookie. Idempotent on empty / unknown IDs.
func (uc *UseCase) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if err := uc.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("auth.Logout: %w", err)
	}
	return nil
}

func (uc *UseCase) callbackURI(providerName string) string {
	return strings.TrimRight(uc.cfg.BaseURL, "/") + "/auth/" + providerName + "/callback"
}
