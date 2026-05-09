package profile

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	domprofile "github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
)

type fakeProvider struct {
	name string
	list func(ctx context.Context, t provider.Token) ([]domprofile.Resource, error)
}

func (f *fakeProvider) Name() string                                  { return f.name }
func (f *fakeProvider) BuildAuthURL(string, string, string) string    { return "" }
func (f *fakeProvider) ExchangeCode(context.Context, string, string, string) (provider.Token, error) {
	return provider.Token{}, nil
}
func (f *fakeProvider) GetIdentity(context.Context, provider.Token) (provider.Identity, error) {
	return provider.Identity{}, nil
}
func (f *fakeProvider) ListResources(ctx context.Context, t provider.Token) ([]domprofile.Resource, error) {
	return f.list(ctx, t)
}

type fakeRegistry struct{ p provider.Provider }

func (r *fakeRegistry) Get(string) (provider.Provider, error) { return r.p, nil }
func (r *fakeRegistry) Names() []string                       { return []string{r.p.Name()} }

type fakeInternal struct {
	fn func(ctx context.Context, id string) (domprofile.User, error)
}

func (f *fakeInternal) GetUser(ctx context.Context, id string) (domprofile.User, error) {
	return f.fn(ctx, id)
}

func newLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestGetProfile_HappyPath(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		list: func(context.Context, provider.Token) ([]domprofile.Resource, error) {
			return []domprofile.Resource{{Name: "r1", Metric: 7, MetricLabel: "stars"}}, nil
		},
	}
	svc := &fakeInternal{
		fn: func(_ context.Context, id string) (domprofile.User, error) {
			return domprofile.User{ID: id, DisplayName: "Demo", License: "Pro", Role: "admin"}, nil
		},
	}
	uc := New(newLog(), &fakeRegistry{p: p}, svc)
	np, err := uc.GetProfile(context.Background(), domauth.Session{Provider: "github", ProviderUserID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if np.User.ID != "42" || np.User.License != "Pro" {
		t.Fatalf("user wrong: %+v", np.User)
	}
	if len(np.Resources) != 1 || np.Resources[0].Metric != 7 {
		t.Fatalf("resources wrong: %+v", np.Resources)
	}
}

func TestGetProfile_ProviderError(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		list: func(context.Context, provider.Token) ([]domprofile.Resource, error) {
			return nil, errors.New("provider boom")
		},
	}
	svc := &fakeInternal{
		fn: func(context.Context, string) (domprofile.User, error) { return domprofile.User{}, nil },
	}
	uc := New(newLog(), &fakeRegistry{p: p}, svc)
	_, err := uc.GetProfile(context.Background(), domauth.Session{Provider: "github", ProviderUserID: "42"})
	if err == nil || !strings.Contains(err.Error(), "provider boom") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestGetProfile_InternalUserNotFound_Bubbled(t *testing.T) {
	p := &fakeProvider{
		name: "github",
		list: func(context.Context, provider.Token) ([]domprofile.Resource, error) {
			return []domprofile.Resource{}, nil
		},
	}
	svc := &fakeInternal{
		fn: func(context.Context, string) (domprofile.User, error) {
			return domprofile.User{}, internalsvc.ErrUserNotFound
		},
	}
	uc := New(newLog(), &fakeRegistry{p: p}, svc)
	_, err := uc.GetProfile(context.Background(), domauth.Session{Provider: "github", ProviderUserID: "42"})
	if !errors.Is(err, internalsvc.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
