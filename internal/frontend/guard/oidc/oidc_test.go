package oidc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/oidc"
)

const resource = "https://mcp.example.org/mcp"

func newAuthenticator(t *testing.T, is *issuer, mutate func(*oidc.Config)) *oidc.Authenticator {
	t.Helper()
	cfg := oidc.Config{Issuer: is.URL, Audience: resource, Scopes: []string{"sysml:read", "sysml:write"}}
	if mutate != nil {
		mutate(&cfg)
	}
	a, err := oidc.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestNewRefusesIncompleteConfig(t *testing.T) {
	is := newIssuer(t)
	for name, cfg := range map[string]oidc.Config{
		"no issuer":             {Audience: resource},
		"no audience":           {Issuer: is.URL},
		"negative skew":         {Issuer: is.URL, Audience: resource, ClockSkew: -time.Second},
		"introspect, no client": {Issuer: is.URL, Audience: resource, Introspect: is.URL + "/introspect"},
		"client, no introspect": {Issuer: is.URL, Audience: resource, Client: oidc.Client{ID: "a", Secret: "b"}},
		"introspect not a URL":  {Issuer: is.URL, Audience: resource, Introspect: "introspect", Client: oidc.Client{ID: "a", Secret: "b"}},
		"unreachable issuer":    {Issuer: "http://127.0.0.1:1", Audience: resource},
	} {
		if _, err := oidc.New(context.Background(), cfg); err == nil {
			t.Errorf("%s: New accepted %+v", name, cfg)
		}
	}
}

func TestNewNeedsJWKSURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + "http://" + r.Host + `"}`))
	}))
	defer srv.Close()
	if _, err := oidc.New(context.Background(), oidc.Config{Issuer: srv.URL, Audience: resource}); err == nil || !strings.Contains(err.Error(), "jwks_uri") {
		t.Fatalf("New without jwks_uri: %v", err)
	}
}

func TestVerifyAcceptsTheProvidersToken(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	c := is.standard(resource).with("scope", "sysml:read sysml:write")
	p, refusal := a.Verify(context.Background(), bearer(is.token(t, c)), nil)
	if refusal != guard.Accepted {
		t.Fatalf("refusal = %v", refusal)
	}
	if p.Subject != "alice" || p.Name != "alice@example.org" || p.Source != oidc.Source {
		t.Errorf("principal = %+v", p)
	}
	if !slices.Equal(p.Scopes, []string{"sysml:read", "sysml:write"}) {
		t.Errorf("scopes = %q", p.Scopes)
	}
	if want := time.Unix(c["exp"].(int64), 0); !p.Expires.Equal(want) {
		t.Errorf("expires = %v, want %v", p.Expires, want)
	}
	if hits := is.jwksHits.Load(); hits != 1 {
		t.Errorf("JWKS fetched %d times", hits)
	}
	// The second token uses the cached keys.
	if _, refusal := a.Verify(context.Background(), bearer(is.token(t, c)), nil); refusal != guard.Accepted {
		t.Fatalf("second refusal = %v", refusal)
	}
	if hits := is.jwksHits.Load(); hits != 1 {
		t.Errorf("JWKS fetched %d times after a second token", hits)
	}
}

func TestVerifyNamesThePrincipal(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	base := is.standard(resource).with("preferred_username", nil)
	for _, tc := range []struct {
		name string
		c    claims
		want string
	}{
		{"preferred_username first", base.with("preferred_username", "ali").with("email", "a@x").with("name", "Alice"), "ali"},
		{"email second", base.with("email", "a@x").with("name", "Alice"), "a@x"},
		{"name third", base.with("name", "Alice"), "Alice"},
		{"subject last", base, "alice"},
	} {
		p, refusal := a.Verify(context.Background(), bearer(is.token(t, tc.c)), nil)
		if refusal != guard.Accepted || p.Name != tc.want {
			t.Errorf("%s: name = %q (%v), want %q", tc.name, p.Name, refusal, tc.want)
		}
	}
}

func TestVerifyRefusesForeignTokens(t *testing.T) {
	is := newIssuer(t)
	other := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	good := is.standard(resource)
	for _, tc := range []struct {
		name  string
		token string
		want  guard.Refusal
	}{
		{"wrong audience", is.token(t, good.with("aud", "https://other.example.org/")), guard.Mismatch},
		{"audience list without ours", is.token(t, good.with("aud", []string{"a", "b"})), guard.Mismatch},
		{"wrong issuer", is.token(t, good.with("iss", other.URL)), guard.Mismatch},
		{"signed by another key", sign(t, other.key, is.kid, good), guard.Mismatch},
		{"expired", is.token(t, good.with("exp", time.Now().Add(-time.Minute).Unix())), guard.Expired},
		{"not yet valid", is.token(t, good.with("nbf", time.Now().Add(time.Minute).Unix())), guard.Mismatch},
		{"no subject", is.token(t, good.with("sub", nil)), guard.Mismatch},
		{"unsigned", strings.Join(strings.Split(is.token(t, good), ".")[:2], ".") + ".", guard.Mismatch},
		{"garbage payload", strings.Split(is.token(t, good), ".")[0] + ".bm90anNvbg.c2ln", guard.Mismatch},
	} {
		p, refusal := a.Verify(context.Background(), bearer(tc.token), nil)
		if refusal != tc.want {
			t.Errorf("%s: refusal = %v, want %v", tc.name, refusal, tc.want)
		}
		if !zero(p) {
			t.Errorf("%s: principal = %+v", tc.name, p)
		}
	}
	// The audience may be a list that holds the resource (RFC 7519 § 4.1.3).
	if _, refusal := a.Verify(context.Background(), bearer(is.token(t, good.with("aud", []string{"other", resource}))), nil); refusal != guard.Accepted {
		t.Errorf("audience list with ours: %v", refusal)
	}
}

func TestVerifyToleratesClockSkew(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, func(c *oidc.Config) { c.ClockSkew = time.Minute })
	good := is.standard(resource)
	for _, tc := range []struct {
		name string
		c    claims
		want guard.Refusal
	}{
		{"expired within skew", good.with("exp", time.Now().Add(-30*time.Second).Unix()), guard.Accepted},
		{"expired beyond skew", good.with("exp", time.Now().Add(-90*time.Second).Unix()), guard.Expired},
		{"nbf within skew", good.with("nbf", time.Now().Add(30*time.Second).Unix()), guard.Accepted},
		{"nbf beyond skew", good.with("nbf", time.Now().Add(90*time.Second).Unix()), guard.Mismatch},
	} {
		if _, refusal := a.Verify(context.Background(), bearer(is.token(t, tc.c)), nil); refusal != tc.want {
			t.Errorf("%s: refusal = %v, want %v", tc.name, refusal, tc.want)
		}
	}
	// The default skew holds the expiry check at least as close as a few seconds.
	strict := newAuthenticator(t, is, nil)
	if _, refusal := strict.Verify(context.Background(), bearer(is.token(t, good.with("exp", time.Now().Add(-2*oidc.DefaultClockSkew).Unix()))), nil); refusal != guard.Expired {
		t.Errorf("default skew: refusal = %v", refusal)
	}
}

func TestVerifyExtractsScopes(t *testing.T) {
	is := newIssuer(t)
	good := is.standard(resource)
	for _, tc := range []struct {
		name   string
		mutate func(*oidc.Config)
		c      claims
		want   []string
	}{
		{"scope claim", nil, good.with("scope", "openid sysml:read  sysml:read"), []string{"openid", "sysml:read"}},
		{"scp array", nil, good.with("scp", []string{"sysml:read", "sysml:write"}), []string{"sysml:read", "sysml:write"}},
		{"scope wins over scp", nil, good.with("scope", "a").with("scp", []string{"b"}), []string{"a"}},
		{"no scopes", nil, good, nil},
		{"configured claim", func(c *oidc.Config) { c.ScopeClaim = "permissions" }, good.with("permissions", []string{"x"}).with("scope", "ignored"), []string{"x"}},
		{"configured claim absent", func(c *oidc.Config) { c.ScopeClaim = "permissions" }, good.with("scope", "ignored"), nil},
		{"roles map to scopes", func(c *oidc.Config) { c.Roles = map[string][]string{"editor": {"sysml:read", "sysml:write"}} },
			good.with("scope", "sysml:read").with("roles", []string{"editor", "unknown"}), []string{"sysml:read", "sysml:write"}},
		{"scope of another type", nil, good.with("scope", 42), nil},
	} {
		a := newAuthenticator(t, is, tc.mutate)
		p, refusal := a.Verify(context.Background(), bearer(is.token(t, tc.c)), nil)
		if refusal != guard.Accepted {
			t.Errorf("%s: refusal = %v", tc.name, refusal)
			continue
		}
		if !slices.Equal(p.Scopes, tc.want) {
			t.Errorf("%s: scopes = %q, want %q", tc.name, p.Scopes, tc.want)
		}
	}
}

func TestVerifyPassesOnWhatIsNotAJWT(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	for _, tc := range []struct {
		name          string
		authorization string
		want          guard.Refusal
	}{
		{"no header", "", guard.NothingPresented},
		{"basic", "Basic dXNlcjpwYXNz", guard.NotBearer},
		{"malformed", "Bearer a b", guard.Malformed},
		{"api key", "Bearer osk_abc.def", guard.NothingPresented},
		{"static token", "Bearer 0123456789abcdef0123456789abcdef", guard.NothingPresented},
		{"three segments, no JSON header", "Bearer abc.def.ghi", guard.NothingPresented},
	} {
		p, refusal := a.Verify(context.Background(), tc.authorization, nil)
		if refusal != tc.want || !zero(p) {
			t.Errorf("%s: %+v, %v; want %v", tc.name, p, refusal, tc.want)
		}
	}
}

func TestChainTriesTheStaticTokenAfterOIDC(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	static, err := guard.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	chain := guard.Chain{a, guard.NewStaticToken(static, "sysml:read")}
	p, refusal := chain.Verify(context.Background(), "Bearer 0123456789abcdef0123456789abcdef", nil)
	if refusal != guard.Accepted || p.Source != guard.SourceStaticToken {
		t.Errorf("static token through the chain: %+v, %v", p, refusal)
	}
	p, refusal = chain.Verify(context.Background(), bearer(is.token(t, is.standard(resource))), nil)
	if refusal != guard.Accepted || p.Source != oidc.Source {
		t.Errorf("JWT through the chain: %+v, %v", p, refusal)
	}
	if _, refusal = chain.Verify(context.Background(), bearer(is.token(t, is.standard("other"))), nil); refusal != guard.Mismatch {
		t.Errorf("foreign JWT through the chain: %v", refusal)
	}
	if _, refusal = chain.Verify(context.Background(), "Bearer fedcba9876543210fedcba9876543210", nil); refusal != guard.Mismatch {
		t.Errorf("wrong static token through the chain: %v", refusal)
	}
}

func TestMetadataAdvertisesTheIssuer(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	md := a.Metadata()
	if !slices.Equal(md.AuthorizationServers, []string{is.URL}) {
		t.Errorf("authorization_servers = %q", md.AuthorizationServers)
	}
	if !slices.Equal(md.ScopesSupported, []string{"sysml:read", "sysml:write"}) {
		t.Errorf("scopes_supported = %q", md.ScopesSupported)
	}
	md.ScopesSupported[0] = "changed"
	if a.Metadata().ScopesSupported[0] != "sysml:read" {
		t.Error("Metadata shares its slice with the caller")
	}
	static, _ := guard.New("0123456789abcdef0123456789abcdef")
	chained := guard.Chain{a, guard.NewStaticToken(static)}.Metadata()
	if !slices.Equal(chained.AuthorizationServers, []string{is.URL}) {
		t.Errorf("chain authorization_servers = %q", chained.AuthorizationServers)
	}
}

func TestVerifyFollowsKeyRotation(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	if _, refusal := a.Verify(context.Background(), bearer(is.token(t, is.standard(resource))), nil); refusal != guard.Accepted {
		t.Fatalf("before rotation: %v", refusal)
	}
	is.rotate(t)
	if _, refusal := a.Verify(context.Background(), bearer(is.token(t, is.standard(resource))), nil); refusal != guard.Accepted {
		t.Fatalf("after rotation: %v", refusal)
	}
	if hits := is.jwksHits.Load(); hits != 2 {
		t.Errorf("JWKS fetched %d times, want 2", hits)
	}
}

func TestVerifyReportsTheProviderUnavailable(t *testing.T) {
	is := newIssuer(t)
	a := newAuthenticator(t, is, nil)
	token := bearer(is.token(t, is.standard(resource)))
	is.Close()
	p, refusal := a.Verify(context.Background(), token, nil)
	if refusal != guard.Unavailable || !zero(p) {
		t.Fatalf("with the issuer down: %+v, %v", p, refusal)
	}
	if status := refusal.HTTPStatus(); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d", status)
	}
}
