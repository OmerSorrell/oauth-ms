package cached

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/adapter/cache/memory"
	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
)

func TestStateStore_PutConsume(t *testing.T) {
	c := memory.New()
	defer c.Stop()
	s := NewStateStore(c)
	ctx := context.Background()

	rec := domauth.StateRecord{State: "abc", CodeVerifier: "v", Provider: "github", RedirectURI: "http://x/cb"}
	if err := s.Put(ctx, rec, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := s.Consume(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.CodeVerifier != "v" || got.Provider != "github" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestStateStore_ConsumeIsSingleUse(t *testing.T) {
	c := memory.New()
	defer c.Stop()
	s := NewStateStore(c)
	ctx := context.Background()
	_ = s.Put(ctx, domauth.StateRecord{State: "abc", CodeVerifier: "v", Provider: "github"}, time.Minute)
	if _, err := s.Consume(ctx, "abc"); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	_, err := s.Consume(ctx, "abc")
	if !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected ErrStateNotFound on replay, got %v", err)
	}
}

func TestStateStore_UnknownState(t *testing.T) {
	c := memory.New()
	defer c.Stop()
	s := NewStateStore(c)
	_, err := s.Consume(context.Background(), "never-stored")
	if !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected ErrStateNotFound, got %v", err)
	}
}
