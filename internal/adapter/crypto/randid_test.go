package crypto

import "testing"

func TestRandomURLSafeID(t *testing.T) {
	a, err := RandomURLSafeID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := RandomURLSafeID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("two consecutive ids should differ")
	}
	if len(a) != 43 {
		t.Fatalf("expected 32-byte url-safe (43 chars), got %d", len(a))
	}
}
