package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)
	first, err := h.Hash("  secret123  ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Hash("  secret123  ")
	if err != nil {
		t.Fatal(err)
	}
	if first == "  secret123  " || first == second {
		t.Fatal("password must be hashed with an independent salt")
	}
	if err := h.Compare(first, "  secret123  "); err != nil {
		t.Fatalf("valid password=%v", err)
	}
	if err := h.Compare(first, "secret123"); err == nil {
		t.Fatal("password spaces must not be silently removed")
	}
	if err := h.Compare(first, "wrong-password"); err == nil {
		t.Fatal("wrong password accepted")
	}
}
