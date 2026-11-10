package oidc_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/oidc"
)

const opaque = "Bearer 9f8e7d6c5b4a39281706f5e4d3c2b1a0"

func introspecting(t *testing.T, is *issuer, now *time.Time) *oidc.Authenticator {
	t.Helper()
	return newAuthenticator(t, is, func(c *oidc.Config) {
		c.Introspect = is.URL + "/introspect"
		c.Client = oidc.Client{ID: "sysml-mcp", Secret: "s3cret/"}
		if now != nil {
			c.Now = func() time.Time { return *now }
		}
	})
}

func TestIntrospectionAcceptsAnActiveToken(t *testing.T) {
	is := newIssuer(t)
	now := time.Now()
	a := introspecting(t, is, &now)
	var hits atomic.Int32
	exp := now.Add(time.Hour).Unix()
	is.answer(t, &hits, http.StatusOK, map[string]any{
		"active": true, "sub": "bob", "username": "bob@example.org", "scope": "sysml:read sysml:read",
		"exp": exp, "iss": is.URL, "aud": []string{resource},
	})
	p, refusal := a.Verify(context.Background(), opaque, nil)
	if refusal != guard.Accepted {
		t.Fatalf("refusal = %v", refusal)
	}
	if p.Subject != "bob" || p.Name != "bob@example.org" || p.Source != oidc.Source || !p.Expires.Equal(time.Unix(exp, 0)) {
		t.Errorf("principal = %+v", p)
	}
	if !slices.Equal(p.Scopes, []string{"sysml:read"}) {
		t.Errorf("scopes = %q", p.Scopes)
	}
	// The verdict is kept until exp: the provider is not asked again.
	if _, refusal := a.Verify(context.Background(), opaque, nil); refusal != guard.Accepted {
		t.Fatalf("cached refusal = %v", refusal)
	}
	if hits.Load() != 1 {
		t.Errorf("introspected %d times, want 1", hits.Load())
	}
	// Once exp passes, the cached answer is dropped and the provider asked anew.
	now = now.Add(2 * time.Hour)
	is.answer(t, &hits, http.StatusOK, map[string]any{"active": false})
	if _, refusal := a.Verify(context.Background(), opaque, nil); refusal != guard.Mismatch {
		t.Errorf("after expiry: %v", refusal)
	}
	if hits.Load() != 2 {
		t.Errorf("introspected %d times, want 2", hits.Load())
	}
}

func TestIntrospectionRefusals(t *testing.T) {
	is := newIssuer(t)
	now := time.Now()
	a := introspecting(t, is, &now)
	active := map[string]any{"active": true, "sub": "bob", "exp": now.Add(time.Hour).Unix(), "aud": resource}
	for _, tc := range []struct {
		name   string
		status int
		body   map[string]any
		want   guard.Refusal
	}{
		{"inactive", http.StatusOK, map[string]any{"active": false}, guard.Mismatch},
		{"no subject", http.StatusOK, map[string]any{"active": true, "aud": resource}, guard.Mismatch},
		{"no audience", http.StatusOK, without(active, "aud"), guard.Mismatch},
		{"other issuer", http.StatusOK, with(active, "iss", "https://other.example.org"), guard.Mismatch},
		{"other audience", http.StatusOK, with(active, "aud", "https://other.example.org/"), guard.Mismatch},
		{"expired", http.StatusOK, with(active, "exp", now.Add(-time.Hour).Unix()), guard.Expired},
		{"endpoint failing", http.StatusInternalServerError, map[string]any{}, guard.Unavailable},
		{"client refused", http.StatusUnauthorized, map[string]any{"error": "invalid_client"}, guard.Unavailable},
	} {
		var hits atomic.Int32
		is.answer(t, &hits, tc.status, tc.body)
		p, refusal := a.Verify(context.Background(), opaque, nil)
		if refusal != tc.want || !zero(p) {
			t.Errorf("%s: %+v, %v; want %v", tc.name, p, refusal, tc.want)
		}
	}
	is.Close()
	if _, refusal := a.Verify(context.Background(), opaque, nil); refusal != guard.Unavailable {
		t.Errorf("endpoint down: %v", refusal)
	}
}

func TestIntrospectionKeepsNoAnswerWithoutExpiry(t *testing.T) {
	is := newIssuer(t)
	a := introspecting(t, is, nil)
	var hits atomic.Int32
	is.answer(t, &hits, http.StatusOK, map[string]any{"active": true, "sub": "bob", "aud": resource})
	for i := 0; i < 2; i++ {
		p, refusal := a.Verify(context.Background(), opaque, nil)
		if refusal != guard.Accepted || !p.Expires.IsZero() || p.Name != "bob" {
			t.Fatalf("%d: %+v, %v", i, p, refusal)
		}
	}
	if hits.Load() != 2 {
		t.Errorf("introspected %d times, want 2", hits.Load())
	}
}

func TestIntrospectionLeavesJWTsToTheVerifier(t *testing.T) {
	is := newIssuer(t)
	a := introspecting(t, is, nil)
	var hits atomic.Int32
	is.answer(t, &hits, http.StatusOK, map[string]any{"active": true, "sub": "bob", "aud": resource})
	if _, refusal := a.Verify(context.Background(), bearer(is.token(t, is.standard("other"))), nil); refusal != guard.Mismatch {
		t.Errorf("foreign JWT: %v", refusal)
	}
	if hits.Load() != 0 {
		t.Errorf("a JWT was introspected")
	}
}

func TestReadClientFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	c, err := oidc.ReadClientFile(write("ok.json", `{"client_id":"sysml-mcp","client_secret":"s3cret"}`))
	if err != nil || c != (oidc.Client{ID: "sysml-mcp", Secret: "s3cret"}) {
		t.Errorf("ReadClientFile = %+v, %v", c, err)
	}
	for name, body := range map[string]string{
		"no-secret.json": `{"client_id":"sysml-mcp"}`,
		"no-id.json":     `{"client_secret":"s3cret"}`,
		"bad.json":       `client_id=sysml-mcp`,
	} {
		if _, err := oidc.ReadClientFile(write(name, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := oidc.ReadClientFile(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file accepted")
	}
}

func without(m map[string]any, k string) map[string]any {
	out := with(m, k, nil)
	delete(out, k)
	return out
}

func with(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for key, val := range m {
		out[key] = val
	}
	out[k] = v
	return out
}
