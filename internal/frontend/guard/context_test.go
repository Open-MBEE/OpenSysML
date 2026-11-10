package guard

import (
	"context"
	"testing"
	"time"
)

// A principal put in a context is read back whole, and a context without one
// says so rather than yielding a zero principal as if it were accepted.
func TestPrincipalFromContext(t *testing.T) {
	if principal, ok := PrincipalFrom(context.Background()); ok {
		t.Fatalf("an empty context holds a principal: %+v", principal)
	}
	want := Principal{Subject: "alice", Name: "Alice", Scopes: []string{"sysml:read"}, Expires: time.Unix(1, 0), Source: "api-key"}
	ctx := WithPrincipal(context.Background(), want)
	got, ok := PrincipalFrom(ctx)
	if !ok {
		t.Fatal("the principal was not found")
	}
	if got.Subject != want.Subject || got.Name != want.Name || got.Source != want.Source || !got.Expires.Equal(want.Expires) || len(got.Scopes) != 1 || got.Scopes[0] != "sysml:read" {
		t.Errorf("principal = %+v, want %+v", got, want)
	}
	if _, ok := PrincipalFrom(context.WithValue(ctx, principalKey{}, "not a principal")); ok {
		t.Error("a value of another type under the key passed as a principal")
	}
}
