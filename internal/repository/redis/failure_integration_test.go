//go:build integration

package redis

import (
	"testing"
	"time"
)

func TestIntegrationSequenceFailureDoesNotConsumeOldSession(t *testing.T) {
	r, ctx := integrationRepo(t)
	old := session("old-incr", time.Minute)
	if _, err := r.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := r.client.Set(ctx, r.prefix+"sequence", "not-an-integer", 0).Err(); err != nil {
		t.Fatal(err)
	}
	next := session("next-incr", time.Minute)
	if err := r.Rotate(ctx, old.TokenHash, next); err == nil {
		t.Fatal("expected INCR failure")
	}
	current, err := r.GetByTokenHash(ctx, old.TokenHash)
	if err != nil || current.RevokedAt != nil {
		t.Fatal("failed rotation revoked old session", err)
	}
	if n := r.client.Exists(ctx, r.key(next.TokenHash)).Val(); n != 0 {
		t.Fatal("failed rotation wrote new session")
	}
}
func TestIntegrationInvalidExpirationDoesNotWrite(t *testing.T) {
	r, ctx := integrationRepo(t)
	bad := session("too-far", time.Minute)
	bad.ExpiresAt = time.Date(300000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := r.Create(ctx, bad); err == nil {
		t.Fatal("invalid expiration accepted")
	}
	if r.client.Exists(ctx, r.key(bad.TokenHash)).Val() != 0 {
		t.Fatal("invalid expiry created a hash")
	}
}
