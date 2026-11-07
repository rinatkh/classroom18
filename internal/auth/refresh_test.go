package auth

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestRefreshTokenGenerationAndHash(t *testing.T) {
	generator := NewCryptoRefreshTokenGenerator()
	first, err := generator.Generate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first == second || len(first) != 43 {
		t.Fatal("unexpected generated credential")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil || len(decoded) != 32 {
		t.Fatal("expected 32 random bytes encoded as unpadded base64url")
	}
	hash := generator.Hash(first)
	if hash != generator.Hash(first) || hash == generator.Hash(second) || hash == first {
		t.Fatal("hash contract")
	}
	digest, err := hex.DecodeString(hash)
	if err != nil || len(digest) != 32 {
		t.Fatal("expected SHA-256 hex digest")
	}
	if generator.Hash("abc") != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal("SHA-256 known vector")
	}
}
