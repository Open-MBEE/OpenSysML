package guard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// claimant is an authenticator of one kind of bearer token: those with its
// prefix. It accepts the one named good, refuses its others and passes the rest.
type claimant struct {
	prefix, good string
	principal    Principal
	refusal      Refusal
	metadata     ResourceMetadata
	asked        *[]string
}

func (c claimant) Verify(_ context.Context, authorization string, _ *http.Request) (Principal, Refusal) {
	*c.asked = append(*c.asked, c.prefix)
	token, refusal := Bearer(authorization)
	switch {
	case refusal != Accepted:
		return Principal{}, refusal
	case !strings.HasPrefix(token, c.prefix):
		return Principal{}, NothingPresented
	case token != c.good:
		// A refusing authenticator's principal must not leave the chain.
		return c.principal, c.refusal
	}
	return c.principal, Accepted
}

func (c claimant) Metadata() ResourceMetadata { return c.metadata }

func TestRefusalOfAnAuthenticator(t *testing.T) {
	cases := []struct {
		refusal              Refusal
		status               int
		code, message, label string
	}{
		{Expired, http.StatusUnauthorized, "invalid_token", "Unauthorized: the bearer token has expired", "expired token"},
		{InsufficientScope, http.StatusForbidden, "insufficient_scope", "Forbidden: the bearer token lacks a scope this server requires", "insufficient scope"},
	}
	for _, c := range cases {
		if c.refusal.HTTPStatus() != c.status || c.refusal.Code() != c.code || c.refusal.Message() != c.message || c.refusal.String() != c.label {
			t.Errorf("%v: status %d code %q message %q, want %d %q %q", c.refusal, c.refusal.HTTPStatus(), c.refusal.Code(), c.refusal.Message(), c.status, c.code, c.message)
		}
		want := `Bearer realm="svc", error="` + c.code + `", error_description="` + c.refusal.Description() + `"`
		if got := c.refusal.Challenge("svc"); got != want {
			t.Errorf("%v.Challenge = %q, want %q", c.refusal, got, want)
		}
	}
}

func TestBearer(t *testing.T) {
	cases := []struct {
		header, token string
		want          Refusal
	}{
		{"", "", NothingPresented},
		{"   ", "", NothingPresented},
		{"Basic cmlnaHQ=", "", NotBearer},
		{"Bearer", "", Malformed},
		{"Bearer has space", "", Malformed},
		{"Bearer\tabc", "", NotBearer},
		{"Bearer abc", "abc", Accepted},
		{"bEARER a-b.c_d~e+f/g==", "a-b.c_d~e+f/g==", Accepted},
		{"  Bearer   abc  ", "abc", Accepted},
	}
	for _, c := range cases {
		if token, refusal := Bearer(c.header); token != c.token || refusal != c.want {
			t.Errorf("Bearer(%q) = %q, %v; want %q, %v", c.header, token, refusal, c.token, c.want)
		}
	}
}

func TestStaticToken(t *testing.T) {
	token, _ := New("right")
	scopes := []string{"model:read", "model:write"}
	var auth Authenticator = NewStaticToken(token, scopes...)
	request := httptest.NewRequest(http.MethodPost, "/", nil)

	for _, header := range []string{"", "Basic cmlnaHQ=", "Bearer", "Bearer has space", "Bearer wrong", "Bearer right", "bearer right", "  Bearer   right  "} {
		principal, refusal := auth.Verify(context.Background(), header, request)
		if want := token.Check(header); refusal != want {
			t.Errorf("Verify(%q) = %v, want Token.Check's %v", header, refusal, want)
		}
		if refusal != Accepted && (principal.Subject != "" || principal.Scopes != nil || principal.Source != "") {
			t.Errorf("Verify(%q) refused as %v yet named %+v", header, refusal, principal)
		}
	}

	principal, refusal := auth.Verify(context.Background(), "Bearer right", nil)
	if refusal != Accepted {
		t.Fatalf("Verify with no HTTP request = %v, want Accepted", refusal)
	}
	if principal.Subject != "token" || principal.Source != "static-token" || principal.Name == "" {
		t.Errorf("principal = %+v; want subject %q of source %q with a name", principal, StaticTokenSubject, SourceStaticToken)
	}
	if !principal.Expires.IsZero() {
		t.Errorf("principal expires %v; a static token has no expiry", principal.Expires)
	}
	if !slices.Equal(principal.Scopes, scopes) {
		t.Errorf("principal scopes = %v, want every scope given %v", principal.Scopes, scopes)
	}

	// Neither the caller's slice nor one principal's reaches the next principal.
	scopes[0], principal.Scopes[1] = "changed", "changed"
	again, _ := auth.Verify(context.Background(), "Bearer right", nil)
	if !slices.Equal(again.Scopes, []string{"model:read", "model:write"}) {
		t.Errorf("a later principal's scopes = %v; they were changed from outside", again.Scopes)
	}

	if m := auth.Metadata(); m.AuthorizationServers != nil || m.ScopesSupported != nil {
		t.Errorf("Metadata() = %+v, want nothing advertised", m)
	}
	if p, refusal := NewStaticToken(token).Verify(context.Background(), "Bearer right", nil); refusal != Accepted || p.Scopes != nil {
		t.Errorf("a token given no scopes: %v with scopes %v", refusal, p.Scopes)
	}
	for _, header := range []string{"Bearer right", "Bearer", ""} {
		if _, refusal := (StaticToken{}).Verify(context.Background(), header, nil); refusal == Accepted {
			t.Errorf("an unset token accepted %q", header)
		}
	}
}

func TestChainOrder(t *testing.T) {
	var asked []string
	key := claimant{prefix: "key_", good: "key_alice", principal: Principal{Subject: "alice", Source: "key", Expires: time.Unix(1, 0)}, refusal: Mismatch, asked: &asked}
	jwt := claimant{prefix: "jwt_", good: "jwt_bob", principal: Principal{Subject: "bob", Source: "jwt"}, refusal: Expired, asked: &asked}
	scoped := claimant{prefix: "jwt_", good: "jwt_none", principal: Principal{Subject: "shadowed"}, refusal: InsufficientScope, asked: &asked}
	static, _ := New("jwt_carol")
	chain := Chain{key, jwt, NewStaticToken(static)}

	cases := []struct {
		header  string
		want    Refusal
		subject string
		asked   int
	}{
		{"Bearer key_alice", Accepted, "alice", 1},
		{"Bearer key_mallory", Mismatch, "", 1},
		{"Bearer jwt_bob", Accepted, "bob", 2},
		// The first authenticator to claim a credential decides, though a later one accepts it.
		{"Bearer jwt_carol", Expired, "", 2},
		{"Bearer other", Mismatch, "", 2},
		{"Basic cmlnaHQ=", NotBearer, "", 1},
		{"Bearer has space", Malformed, "", 1},
		{"", NothingPresented, "", 2},
	}
	for _, c := range cases {
		asked = nil
		principal, refusal := chain.Verify(context.Background(), c.header, nil)
		if refusal != c.want || principal.Subject != c.subject || len(asked) != c.asked {
			t.Errorf("Verify(%q) = %q, %v after asking %v; want %q, %v after %d", c.header, principal.Subject, refusal, asked, c.subject, c.want, c.asked)
		}
		if refusal != Accepted && (principal.Source != "" || !principal.Expires.IsZero()) {
			t.Errorf("Verify(%q) refused as %v yet named %+v", c.header, refusal, principal)
		}
	}

	// The static token reads every bearer token: ahead of the others it decides them all.
	asked = nil
	if _, refusal := (Chain{NewStaticToken(static), key}).Verify(context.Background(), "Bearer key_alice", nil); refusal != Mismatch || len(asked) != 0 {
		t.Errorf("a static token first: %v after asking %v; want Mismatch and nobody asked", refusal, asked)
	}
	// An insufficient scope is a verdict too.
	if _, refusal := (Chain{key, scoped}).Verify(context.Background(), "Bearer jwt_bob", nil); refusal != InsufficientScope {
		t.Errorf("a claimed token lacking a scope: %v, want InsufficientScope", refusal)
	}
	// A chain within a chain passes what it does not claim to the next.
	nested := Chain{Chain{key}, Chain{Chain{}, jwt}}
	for header, want := range map[string]Refusal{"Bearer jwt_bob": Accepted, "Bearer key_mallory": Mismatch, "Bearer other": Mismatch, "": NothingPresented} {
		asked = nil
		p, refusal := nested.Verify(context.Background(), header, nil)
		if refusal != want || (refusal == Accepted) != (p.Subject == "bob") || (header != "Bearer key_mallory") != (len(asked) == 2) {
			t.Errorf("a chain of chains, Verify(%q) = %q, %v after asking %v; want %v", header, p.Subject, refusal, asked, want)
		}
	}
}

// A credential no authenticator claims is refused as its header reads, never
// answered as though nothing had been presented.
func TestChainUnclaimed(t *testing.T) {
	var asked []string
	key := claimant{prefix: "key_", good: "key_alice", principal: Principal{Subject: "alice"}, refusal: Mismatch, asked: &asked}
	jwt := claimant{prefix: "jwt_", good: "jwt_bob", principal: Principal{Subject: "bob"}, refusal: Mismatch, asked: &asked}
	for _, chain := range []Chain{{key, jwt}, {}, nil} {
		for header, want := range map[string]Refusal{
			"":                 NothingPresented,
			"Basic cmlnaHQ=":   NotBearer,
			"Bearer":           Malformed,
			"Bearer has space": Malformed,
			"Bearer neither":   Mismatch,
		} {
			if principal, refusal := chain.Verify(context.Background(), header, nil); refusal != want || principal.Subject != "" {
				t.Errorf("%d authenticators, Verify(%q) = %q, %v; want %v", len(chain), header, principal.Subject, refusal, want)
			}
		}
	}
	if Mismatch.Code() != "invalid_token" || Mismatch.HTTPStatus() != http.StatusUnauthorized {
		t.Errorf("an unclaimed bearer token is answered %d %q, want 401 invalid_token (RFC 6750 § 3.1)", Mismatch.HTTPStatus(), Mismatch.Code())
	}
}

func TestChainMetadata(t *testing.T) {
	var asked []string
	token, _ := New("right")
	chain := Chain{
		claimant{metadata: ResourceMetadata{ScopesSupported: []string{"read", "write"}}, asked: &asked},
		NewStaticToken(token, "read", "admin"),
		claimant{metadata: ResourceMetadata{AuthorizationServers: []string{"https://b.example", "https://a.example"}, ScopesSupported: []string{"write", "admin", "read"}}, asked: &asked},
		Chain{claimant{metadata: ResourceMetadata{AuthorizationServers: []string{"https://a.example", "https://c.example"}}, asked: &asked}},
	}
	m := chain.Metadata()
	if want := []string{"https://b.example", "https://a.example", "https://c.example"}; !slices.Equal(m.AuthorizationServers, want) {
		t.Errorf("AuthorizationServers = %v, want %v", m.AuthorizationServers, want)
	}
	if want := []string{"read", "write", "admin"}; !slices.Equal(m.ScopesSupported, want) {
		t.Errorf("ScopesSupported = %v, want %v", m.ScopesSupported, want)
	}
	if m := (Chain{}).Metadata(); m.AuthorizationServers != nil || m.ScopesSupported != nil {
		t.Errorf("an empty chain advertises %+v", m)
	}
	if len(asked) != 0 {
		t.Errorf("Metadata verified a credential: %v", asked)
	}
}
