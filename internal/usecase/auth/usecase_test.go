package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	domprofile "github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
)

// --- fakes ---------------------------------------------------------------

type fakeProvider struct {
	name        string
	authURL     func(state, verifier, redirect string) string
	exchange    func(ctx context.Context, code, verifier, redirect string) (provider.Token, error)
	getIdentity func(ctx context.Context, t provider.Token) (provider.Identity, error)
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) BuildAuthURL(state, verifier, redirect string) string {
	return f.authURL(state, verifier, redirect)
}
func (f *fakeProvider) ExchangeCode(ctx context.Context, code, verifier, redirect string) (provider.Token, error) {
	return f.exchange(ctx, code, verifier, redirect)
}
func (f *fakeProvider) GetIdentity(ctx context.Context, t provider.Token) (provider.Identity, error) {
	return f.getIdentity(ctx, t)
}
func (f *fakeProvider) ListResources(context.Context, provider.Token) ([]domprofile.Resource, error) {
	return nil, nil
}

type fakeRegistry struct {
	get func(name string) (provider.Provider, error)
}

func (r *fakeRegistry) Get(name string) (provider.Provider, error) { return r.get(name) }
func (r *fakeRegistry) Names() []string                            { return nil }

type fakeStateStore struct {
	put     func(ctx context.Context, rec domauth.StateRecord, ttl time.Duration) error
	consume func(ctx context.Context, state string) (domauth.StateRecord, error)

	lastPut    domauth.StateRecord
	lastPutTTL time.Duration
}

func (s *fakeStateStore) Put(ctx context.Context, rec domauth.StateRecord, ttl time.Duration) error {
	s.lastPut = rec
	s.lastPutTTL = ttl
	if s.put != nil {
		return s.put(ctx, rec, ttl)
	}
	return nil
}
func (s *fakeStateStore) Consume(ctx context.Context, state string) (domauth.StateRecord, error) {
	return s.consume(ctx, state)
}

type fakeSessionStore struct {
	put    func(ctx context.Context, sess domauth.Session, ttl time.Duration) error
	del    func(ctx context.Context, id string) error
	delHit string

	lastPut    domauth.Session
	lastPutTTL time.Duration
}

func (s *fakeSessionStore) Put(ctx context.Context, sess domauth.Session, ttl time.Duration) error {
	s.lastPut = sess
	s.lastPutTTL = ttl
	if s.put != nil {
		return s.put(ctx, sess, ttl)
	}
	return nil
}
func (s *fakeSessionStore) Get(context.Context, string) (domauth.Session, error) {
	return domauth.Session{}, nil
}
func (s *fakeSessionStore) Delete(ctx context.Context, id string) error {
	s.delHit = id
	if s.del != nil {
		return s.del(ctx, id)
	}
	return nil
}

func newLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newUC(t *testing.T, reg provider.Registry, states store.StateStore, sessions store.SessionStore, cfg Config) *UseCase {
	t.Helper()
	uc := New(newLog(), reg, states, sessions, cfg)
	uc.randIDFn = func() (string, error) { return "rand-id", nil }
	uc.verifierFn = func() string { return "verif" }
	uc.nowFn = func() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }
	return uc
}

// --- StartFlow -----------------------------------------------------------

func TestStartFlow_HappyPath(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		authURL: func(state, verifier, redirect string) string {
			if state != "rand-id" || verifier != "verif" {
				t.Errorf("BuildAuthURL got state=%q verifier=%q", state, verifier)
			}
			if redirect != "https://app.example.com/auth/github/callback" {
				t.Errorf("redirect uri wrong: %q", redirect)
			}
			return "https://provider.example/authorize?x=1"
		},
	}
	states := &fakeStateStore{}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states,
		&fakeSessionStore{},
		Config{StateTTL: 5 * time.Minute, SessionTTL: time.Hour, BaseURL: "https://app.example.com"},
	)
	// Stub randID to return distinct values per call (only one call expected here).
	calls := 0
	uc.randIDFn = func() (string, error) {
		calls++
		return "rand-id", nil
	}

	res, err := uc.StartFlow(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "rand-id" {
		t.Errorf("State=%q", res.State)
	}
	if res.AuthorizationURL != "https://provider.example/authorize?x=1" {
		t.Errorf("AuthorizationURL=%q", res.AuthorizationURL)
	}
	if states.lastPut.State != "rand-id" || states.lastPut.CodeVerifier != "verif" ||
		states.lastPut.Provider != "github" ||
		states.lastPut.RedirectURI != "https://app.example.com/auth/github/callback" {
		t.Errorf("state record: %+v", states.lastPut)
	}
	if states.lastPutTTL != 5*time.Minute {
		t.Errorf("state ttl=%v", states.lastPutTTL)
	}
	if states.lastPut.CreatedAt.IsZero() {
		t.Error("CreatedAt not stamped")
	}
	if calls != 1 {
		t.Errorf("randID calls=%d", calls)
	}
}

func TestStartFlow_TrimsTrailingSlashOnBaseURL(t *testing.T) {
	var seenRedirect string
	p := &fakeProvider{
		name: "github",
		authURL: func(_, _, redirect string) string {
			seenRedirect = redirect
			return ""
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		&fakeStateStore{},
		&fakeSessionStore{},
		Config{BaseURL: "https://app.example.com/"},
	)
	if _, err := uc.StartFlow(context.Background(), "github"); err != nil {
		t.Fatal(err)
	}
	if seenRedirect != "https://app.example.com/auth/github/callback" {
		t.Errorf("redirect=%q", seenRedirect)
	}
}

func TestStartFlow_UnknownProvider(t *testing.T) {
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return nil, provider.ErrUnknownProvider }},
		&fakeStateStore{},
		&fakeSessionStore{},
		Config{},
	)
	_, err := uc.StartFlow(context.Background(), "nope")
	if !errors.Is(err, provider.ErrUnknownProvider) {
		t.Fatalf("err=%v", err)
	}
}

func TestStartFlow_RandIDError(t *testing.T) {
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return &fakeProvider{name: "github"}, nil }},
		&fakeStateStore{},
		&fakeSessionStore{},
		Config{},
	)
	uc.randIDFn = func() (string, error) { return "", errors.New("randboom") }

	_, err := uc.StartFlow(context.Background(), "github")
	if err == nil || !strings.Contains(err.Error(), "randboom") {
		t.Fatalf("err=%v", err)
	}
}

func TestStartFlow_StatePutError(t *testing.T) {
	p := &fakeProvider{name: "github", authURL: func(string, string, string) string { return "" }}
	states := &fakeStateStore{
		put: func(context.Context, domauth.StateRecord, time.Duration) error {
			return errors.New("putboom")
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states,
		&fakeSessionStore{},
		Config{},
	)
	_, err := uc.StartFlow(context.Background(), "github")
	if err == nil || !strings.Contains(err.Error(), "putboom") {
		t.Fatalf("err=%v", err)
	}
}

// --- HandleCallback ------------------------------------------------------

func okProvider(t *testing.T, identity provider.Identity, tok provider.Token) *fakeProvider {
	t.Helper()
	return &fakeProvider{
		name: "github",
		exchange: func(_ context.Context, code, verifier, redirect string) (provider.Token, error) {
			if code != "code-xyz" || verifier != "verif-stored" {
				t.Errorf("exchange args: code=%q verifier=%q", code, verifier)
			}
			if redirect != "https://app.example.com/auth/github/callback" {
				t.Errorf("exchange redirect=%q", redirect)
			}
			return tok, nil
		},
		getIdentity: func(context.Context, provider.Token) (provider.Identity, error) {
			return identity, nil
		},
	}
}

func TestHandleCallback_HappyPath(t *testing.T) {
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	tok := provider.Token{
		AccessToken: "tok",
		TokenType:   "bearer",
		ExpiresAt:   expires,
		Scopes:      []string{"read:user"},
	}
	identity := provider.Identity{ProviderUserID: "u-42", DisplayName: "Demo"}

	p := okProvider(t, identity, tok)
	states := &fakeStateStore{
		consume: func(_ context.Context, s string) (domauth.StateRecord, error) {
			if s != "abc" {
				t.Errorf("consume state=%q", s)
			}
			return domauth.StateRecord{
				State:        "abc",
				CodeVerifier: "verif-stored",
				Provider:     "github",
				RedirectURI:  "https://app.example.com/auth/github/callback",
			}, nil
		},
	}
	sessions := &fakeSessionStore{}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states, sessions,
		Config{SessionTTL: 30 * time.Minute, BaseURL: "https://app.example.com"},
	)
	uc.randIDFn = func() (string, error) { return "sid-1", nil }
	uc.nowFn = func() time.Time { return now }

	res, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider:    "github",
		State:       "abc",
		StateCookie: "abc",
		Code:        "code-xyz",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "sid-1" {
		t.Errorf("SessionID=%q", res.SessionID)
	}
	want := domauth.Session{
		ID:                   "sid-1",
		Provider:             "github",
		AccessToken:          "tok",
		TokenType:            "bearer",
		AccessTokenExpiresAt: expires,
		Scopes:               []string{"read:user"},
		ProviderUserID:       "u-42",
		DisplayName:          "Demo",
		CreatedAt:            now,
	}
	if res.Session.ID != want.ID || res.Session.Provider != want.Provider ||
		res.Session.AccessToken != want.AccessToken || res.Session.TokenType != want.TokenType ||
		!res.Session.AccessTokenExpiresAt.Equal(want.AccessTokenExpiresAt) ||
		res.Session.ProviderUserID != want.ProviderUserID ||
		res.Session.DisplayName != want.DisplayName ||
		!res.Session.CreatedAt.Equal(want.CreatedAt) ||
		len(res.Session.Scopes) != 1 || res.Session.Scopes[0] != "read:user" {
		t.Errorf("session=%+v want=%+v", res.Session, want)
	}
	if sessions.lastPutTTL != 30*time.Minute {
		t.Errorf("session ttl=%v", sessions.lastPutTTL)
	}
	if sessions.lastPut.ID != "sid-1" {
		t.Errorf("session put: %+v", sessions.lastPut)
	}
}

func TestHandleCallback_StateMismatch(t *testing.T) {
	cases := []struct {
		name string
		in   CallbackInput
	}{
		{"empty query state", CallbackInput{Provider: "github", State: "", StateCookie: "abc"}},
		{"empty cookie state", CallbackInput{Provider: "github", State: "abc", StateCookie: ""}},
		{"differ", CallbackInput{Provider: "github", State: "abc", StateCookie: "xyz"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			states := &fakeStateStore{
				consume: func(context.Context, string) (domauth.StateRecord, error) {
					t.Fatal("Consume should not be called on state mismatch")
					return domauth.StateRecord{}, nil
				},
			}
			uc := newUC(t,
				&fakeRegistry{get: func(string) (provider.Provider, error) {
					t.Fatal("registry.Get should not be called on state mismatch")
					return nil, nil
				}},
				states, &fakeSessionStore{}, Config{},
			)
			_, err := uc.HandleCallback(context.Background(), tc.in)
			if !errors.Is(err, ErrStateMismatch) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestHandleCallback_ConsumeError(t *testing.T) {
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{}, store.ErrStateNotFound
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) {
			t.Fatal("registry.Get should not be called when consume fails")
			return nil, nil
		}},
		states, &fakeSessionStore{}, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc",
	})
	if !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_ProviderMismatch(t *testing.T) {
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "google"}, nil
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) {
			t.Fatal("registry.Get should not be called on provider mismatch")
			return nil, nil
		}},
		states, &fakeSessionStore{}, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc",
	})
	if !errors.Is(err, ErrProviderMismatch) {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_RegistryError(t *testing.T) {
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "github"}, nil
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return nil, provider.ErrUnknownProvider }},
		states, &fakeSessionStore{}, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc",
	})
	if !errors.Is(err, provider.ErrUnknownProvider) {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_ExchangeError(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		exchange: func(context.Context, string, string, string) (provider.Token, error) {
			return provider.Token{}, errors.New("xchgboom")
		},
	}
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "github", CodeVerifier: "v"}, nil
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states, &fakeSessionStore{}, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc", Code: "c",
	})
	if err == nil || !strings.Contains(err.Error(), "xchgboom") {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_IdentityError(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		exchange: func(context.Context, string, string, string) (provider.Token, error) {
			return provider.Token{AccessToken: "t"}, nil
		},
		getIdentity: func(context.Context, provider.Token) (provider.Identity, error) {
			return provider.Identity{}, errors.New("identboom")
		},
	}
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "github"}, nil
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states, &fakeSessionStore{}, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc", Code: "c",
	})
	if err == nil || !strings.Contains(err.Error(), "identboom") {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_SessionIDError(t *testing.T) {
	p := okProvider(t, provider.Identity{}, provider.Token{})
	// Adjust: okProvider checks the verifier; relax it here.
	p.exchange = func(context.Context, string, string, string) (provider.Token, error) {
		return provider.Token{}, nil
	}
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "github"}, nil
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states, &fakeSessionStore{}, Config{},
	)
	uc.randIDFn = func() (string, error) { return "", errors.New("sidboom") }

	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc",
	})
	if err == nil || !strings.Contains(err.Error(), "sidboom") {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleCallback_SessionPutError(t *testing.T) {
	p := okProvider(t, provider.Identity{}, provider.Token{})
	p.exchange = func(context.Context, string, string, string) (provider.Token, error) {
		return provider.Token{}, nil
	}
	states := &fakeStateStore{
		consume: func(context.Context, string) (domauth.StateRecord, error) {
			return domauth.StateRecord{Provider: "github"}, nil
		},
	}
	sessions := &fakeSessionStore{
		put: func(context.Context, domauth.Session, time.Duration) error {
			return errors.New("sessboom")
		},
	}
	uc := newUC(t,
		&fakeRegistry{get: func(string) (provider.Provider, error) { return p, nil }},
		states, sessions, Config{},
	)
	_, err := uc.HandleCallback(context.Background(), CallbackInput{
		Provider: "github", State: "abc", StateCookie: "abc",
	})
	if err == nil || !strings.Contains(err.Error(), "sessboom") {
		t.Fatalf("err=%v", err)
	}
}

// --- Logout --------------------------------------------------------------

func TestLogout_EmptyIDIsNoop(t *testing.T) {
	sessions := &fakeSessionStore{
		del: func(context.Context, string) error {
			t.Fatal("Delete should not be called for empty session ID")
			return nil
		},
	}
	uc := newUC(t, &fakeRegistry{}, &fakeStateStore{}, sessions, Config{})
	if err := uc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestLogout_DeletesSession(t *testing.T) {
	sessions := &fakeSessionStore{}
	uc := newUC(t, &fakeRegistry{}, &fakeStateStore{}, sessions, Config{})
	if err := uc.Logout(context.Background(), "sid-1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if sessions.delHit != "sid-1" {
		t.Errorf("delHit=%q", sessions.delHit)
	}
}

func TestLogout_DeleteError(t *testing.T) {
	sessions := &fakeSessionStore{
		del: func(context.Context, string) error { return errors.New("delboom") },
	}
	uc := newUC(t, &fakeRegistry{}, &fakeStateStore{}, sessions, Config{})
	err := uc.Logout(context.Background(), "sid-1")
	if err == nil || !strings.Contains(err.Error(), "delboom") {
		t.Fatalf("err=%v", err)
	}
}
