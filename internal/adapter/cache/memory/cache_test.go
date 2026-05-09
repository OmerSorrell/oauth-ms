package memory

import (
	"context"
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := New()
	defer c.Stop()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected hit")
	}
	if string(got) != "v" {
		t.Fatalf("got %q, want %q", got, "v")
	}
}

func TestCache_Miss(t *testing.T) {
	c := New()
	defer c.Stop()
	_, ok, err := c.Get(context.Background(), "absent")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("expected miss")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := New()
	defer c.Stop()
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, _ := c.Get(ctx, "k")
	if ok {
		t.Fatalf("expected miss after TTL")
	}
}

func TestCache_Delete(t *testing.T) {
	c := New()
	defer c.Stop()
	ctx := context.Background()
	_ = c.Set(ctx, "k", []byte("v"), time.Minute)
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := c.Get(ctx, "k")
	if ok {
		t.Fatalf("expected miss after delete")
	}
}
