package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// tokens authenticates the bearer tokens it knows the principals of; another
// token is not of its kind, as a sibling of a guard.Chain answers.
type tokens map[string]guard.Principal

func (ts tokens) Verify(_ context.Context, authorization string, _ *http.Request) (guard.Principal, guard.Refusal) {
	token, refusal := guard.Bearer(authorization)
	if refusal != guard.Accepted {
		return guard.Principal{}, refusal
	}
	if p, ok := ts[token]; ok {
		return p, guard.Accepted
	}
	return guard.Principal{}, guard.NothingPresented
}

func (tokens) Metadata() guard.ResourceMetadata { return guard.ResourceMetadata{} }

// verdict answers every credential alike, and advertises what it is given.
type verdict struct {
	principal guard.Principal
	refusal   guard.Refusal
	metadata  guard.ResourceMetadata
}

func (v verdict) Verify(context.Context, string, *http.Request) (guard.Principal, guard.Refusal) {
	return v.principal, v.refusal
}

func (v verdict) Metadata() guard.ResourceMetadata { return v.metadata }

// TestTokenInfoFor checks a principal reaches the SDK as the token it binds a
// session to: its ID as the user, its scopes and expiry, and itself in Extra.
func TestTokenInfoFor(t *testing.T) {
	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	p := guard.Principal{Subject: "alice", Name: "Alice", Scopes: []string{"sysml:read", "sysml:write"}, Expires: expires, Source: "oidc"}
	info := TokenInfoFor(p)
	if info.UserID != "oidc:alice" || info.UserID != p.ID() {
		t.Errorf("UserID = %q, want the principal's ID %q", info.UserID, p.ID())
	}
	if !slices.Equal(info.Scopes, p.Scopes) || !info.Expiration.Equal(expires) {
		t.Errorf("scopes %v, expiration %v; want %v, %v", info.Scopes, info.Expiration, p.Scopes, expires)
	}
	if got, ok := info.Extra[principalKey].(guard.Principal); !ok || got.ID() != p.ID() || got.Name != "Alice" {
		t.Errorf("Extra = %v, want the principal", info.Extra)
	}
	// One subject under two sources is two users to the SDK.
	if other := TokenInfoFor(guard.Principal{Subject: "alice", Source: "api-key"}); other.UserID == info.UserID {
		t.Errorf("api-key and oidc alice share the user %q", other.UserID)
	}
	// The static token never expires, which the middleware is told to allow.
	static := TokenInfoFor(guard.Principal{Subject: guard.StaticTokenSubject, Source: guard.SourceStaticToken})
	if static.UserID != "static-token:token" || !static.Expiration.IsZero() {
		t.Errorf("static token: user %q, expiration %v", static.UserID, static.Expiration)
	}
}

// TestBearerTokenSetWithAnAuthenticator checks an authenticator alone is what a
// listener other machines reach needs, and that it is asked before the token.
func TestBearerTokenSetWithAnAuthenticator(t *testing.T) {
	if (BearerToken{}).Set() || len(BearerToken{}.authenticators()) != 0 {
		t.Error("the zero BearerToken is set")
	}
	// A chain of none is what the flags resolve to with no credential given.
	for _, none := range []guard.Authenticator{guard.Chain(nil), guard.Chain{}, (*guard.Chain)(nil), &guard.Chain{}} {
		empty := BearerToken{Authenticator: none}
		if served := httptest.NewRecorder(); empty.Set() {
			t.Errorf("a %T of no authenticators is set", none)
		} else if empty.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })).
			ServeHTTP(served, httptest.NewRequest(http.MethodPost, HTTPPath, nil)); served.Code != http.StatusAccepted {
			t.Errorf("a chain of none refuses a request: %d", served.Code)
		}
	}
	key := tokens{"key-alice": {Subject: "alice", Source: "api-key"}}
	for _, one := range []guard.Authenticator{key, guard.Chain{key}, &guard.Chain{key}} {
		if alone := (BearerToken{Authenticator: one}); !alone.Set() || len(alone.authenticators()) != 1 {
			t.Errorf("%T alone: set %v, %d asked", one, alone.Set(), len(alone.authenticators()))
		}
	}
	static, err := NewBearerToken("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !static.Set() || len(static.authenticators()) != 1 {
		t.Errorf("a token alone: set %v, %d asked", static.Set(), len(static.authenticators()))
	}
	static.Authenticator = key
	for authorization, want := range map[string]string{"Bearer key-alice": "api-key:alice", "Bearer s3cret": "static-token:token"} {
		p, refusal := static.authenticators().Verify(context.Background(), authorization, nil)
		if refusal != guard.Accepted || p.ID() != want {
			t.Errorf("%s: %v as %q, want %q", authorization, refusal, p.ID(), want)
		}
	}
	if _, refusal := static.authenticators().Verify(context.Background(), "Bearer other", nil); refusal != guard.Mismatch {
		t.Errorf("a token nobody issued: %v, want a mismatch", refusal)
	}
}

// TestBearerTokenHandlerServesAsThePrincipal checks an accepted request reaches the
// transport with its principal's token, where the SDK binds the session to it.
func TestBearerTokenHandlerServesAsThePrincipal(t *testing.T) {
	alice := guard.Principal{Subject: "alice", Scopes: []string{"sysml:read"}, Expires: time.Now().Add(time.Hour), Source: "api-key"}
	var seen *auth.TokenInfo
	h := BearerToken{Authenticator: tokens{"key-alice": alice}}.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = auth.TokenInfoFromContext(r.Context())
		w.WriteHeader(http.StatusAccepted)
	}))
	req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
	req.Header.Set("Authorization", "Bearer key-alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || seen == nil {
		t.Fatalf("status %d, token %v: %s", rec.Code, seen, rec.Body)
	}
	if seen.UserID != "api-key:alice" || !slices.Equal(seen.Scopes, alice.Scopes) || !seen.Expiration.Equal(alice.Expires) {
		t.Errorf("the transport sees %+v, want alice's token", seen)
	}
}

// TestBearerTokenHandlerAnswersAnAuthenticatorsRefusals checks the refusals an authenticator
// of issued tokens adds are answered as RFC 6750 § 3.1 has them, an outage as 503.
func TestBearerTokenHandlerAnswersAnAuthenticatorsRefusals(t *testing.T) {
	past := guard.Principal{Subject: "alice", Source: "oidc", Expires: time.Now().Add(-time.Second)}
	for _, tc := range []struct {
		name   string
		auth   verdict
		status int
		code   string
	}{
		{"expired", verdict{refusal: guard.Expired}, http.StatusUnauthorized, "invalid_token"},
		{"accepted past its expiry", verdict{principal: past, refusal: guard.Accepted}, http.StatusUnauthorized, "invalid_token"},
		{"insufficient scope", verdict{refusal: guard.InsufficientScope}, http.StatusForbidden, "insufficient_scope"},
		{"unavailable", verdict{refusal: guard.Unavailable}, http.StatusServiceUnavailable, ""},
	} {
		served := false
		h := BearerToken{Authenticator: tc.auth}.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }))
		req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
		req.Host = "models.example.org"
		req.Header.Set("Authorization", "Bearer s3cret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if served || rec.Code != tc.status {
			t.Errorf("%s: served %v, status %d, want %d: %s", tc.name, served, rec.Code, tc.status, rec.Body)
		}
		scheme, params := challengeOf(t, rec)
		if scheme != "Bearer" || params["realm"] != "sysml-mcp" || params["resource_metadata"] != "http://models.example.org"+ProtectedResourceMetadataPath {
			t.Errorf("%s: challenge %s %v", tc.name, scheme, params)
		}
		if params["error"] != tc.code || (tc.code != "") != (params["error_description"] != "") {
			t.Errorf("%s: error %q (%q), want %q", tc.name, params["error"], params["error_description"], tc.code)
		}
		if body := rec.Body.String() + rec.Header().Get("WWW-Authenticate"); strings.Contains(body, "s3cret") {
			t.Errorf("%s: the refusal leaks the token: %s", tc.name, body)
		}
	}
}

// TestBearerTokenServeMetadataAdvertisesItsAuthenticator checks the metadata lists the
// authorization servers and scopes the authenticator advertises (RFC 9728 § 2), the token none.
func TestBearerTokenServeMetadataAdvertisesItsAuthenticator(t *testing.T) {
	oidc := verdict{refusal: guard.NothingPresented, metadata: guard.ResourceMetadata{
		AuthorizationServers: []string{"https://idp.example.org"}, ScopesSupported: []string{"sysml:read", "sysml:write"}}}
	token, err := NewBearerToken("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	token.Authenticator = oidc
	rec := httptest.NewRecorder()
	token.ServeMetadata(rec, httptest.NewRequest(http.MethodGet, ProtectedResourceMetadataPath, nil))
	var meta struct {
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("%v:\n%s", err, rec.Body)
	}
	if !slices.Equal(meta.AuthorizationServers, []string{"https://idp.example.org"}) || !slices.Equal(meta.ScopesSupported, []string{"sysml:read", "sysml:write"}) {
		t.Errorf("metadata = %+v:\n%s", meta, rec.Body)
	}
}

// WhoOutput is the principal a whoami call was served for.
type WhoOutput struct {
	Known bool   `json:"known"`
	ID    string `json:"id"`
	Name  string `json:"name"`
}

// whoServer is a server with a whoami tool answering guard.PrincipalFrom of its
// context, as a tool handler reads who it is serving.
func whoServer(t *testing.T) *Server {
	t.Helper()
	srv, err := New(context.Background(), Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test",
		NewSession: func() (*repl.Session, error) { return repl.NewSession(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(context.Background()) })
	sdk.AddTool(srv.sdk, &sdk.Tool{Name: "whoami"}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, WhoOutput, error) {
		p, ok := guard.PrincipalFrom(ctx)
		return nil, WhoOutput{Known: ok, ID: p.ID(), Name: p.Name}, nil
	})
	return srv
}

// serveWho serves a whoServer over HTTP behind token, and answers its endpoint.
func serveWho(t *testing.T, token BearerToken) string {
	t.Helper()
	srv := whoServer(t)
	mux := http.NewServeMux()
	mux.Handle(HTTPPath, token.Handler(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv.sdk }, nil)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL + HTTPPath
}

// people are two callers named alice by different authenticators, and bob.
var people = BearerToken{Authenticator: guard.Chain{
	tokens{"key-alice": {Subject: "alice", Name: "Alice's key", Source: "api-key"}, "key-alice-2": {Subject: "alice", Name: "Alice's key", Source: "api-key"}},
	tokens{"jwt-alice": {Subject: "alice", Name: "alice@example.org", Source: "oidc"}, "jwt-bob": {Subject: "bob", Name: "bob@example.org", Source: "oidc"}},
}}

// TestToolHandlersReadThePrincipal checks a tool handler finds the principal an
// authenticator named, whole, with guard.PrincipalFrom.
func TestToolHandlersReadThePrincipal(t *testing.T) {
	endpoint := serveWho(t, people)
	for token, want := range map[string]WhoOutput{
		"key-alice": {Known: true, ID: "api-key:alice", Name: "Alice's key"},
		"jwt-alice": {Known: true, ID: "oidc:alice", Name: "alice@example.org"},
		"jwt-bob":   {Known: true, ID: "oidc:bob", Name: "bob@example.org"},
	} {
		var got WhoOutput
		call(t, connectAs(t, endpoint, token), "whoami", map[string]any{}, &got)
		if got != want {
			t.Errorf("%s: whoami = %+v, want %+v", token, got, want)
		}
	}
}

// TestTwoPrincipalsCannotShareOneSession checks a protocol session of the served transport
// answers only the principal that opened it, by ID: a namesake of another source is refused.
func TestTwoPrincipalsCannotShareOneSession(t *testing.T) {
	var logs bytes.Buffer
	endpoint, stop := serveHTTPWith(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, &logs, "127.0.0.1:0", people)
	alice := connectAs(t, endpoint, "key-alice")
	session := alice.ID()
	if session == "" {
		t.Fatal("the server assigned no session ID")
	}
	post := func(authorization string) (status int, body string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", session)
		req.Header.Set("Mcp-Protocol-Version", alice.InitializeResult().ProtocolVersion)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(raw)
	}

	// The opener is served in its session, with either of its credentials.
	for _, own := range []string{"Bearer key-alice", "Bearer key-alice-2"} {
		if status, body := post(own); status != http.StatusOK || !strings.Contains(body, `"tools"`) {
			t.Errorf("%s in its own session: status %d: %s", own, status, body)
		}
	}
	// Another principal is not, were it the same subject from another source.
	for _, other := range []string{"Bearer jwt-bob", "Bearer jwt-alice"} {
		status, body := post(other)
		if status != http.StatusForbidden || !strings.Contains(body, "session user mismatch") || strings.Contains(body, `"tools"`) {
			t.Errorf("%s in alice's session: status %d, want 403 session user mismatch: %s", other, status, body)
		}
	}
	// No credentials at all are refused before the session is looked up.
	if status, _ := post(""); status != http.StatusUnauthorized {
		t.Errorf("no credentials in alice's session: status %d, want 401", status)
	}
	// The session still answers its opener, and the others their own sessions.
	if _, err := alice.ListTools(context.Background(), nil); err != nil {
		t.Errorf("alice after the refusals: %v", err)
	}
	if _, err := connectAs(t, endpoint, "jwt-alice").ListTools(context.Background(), nil); err != nil {
		t.Errorf("oidc alice in her own session: %v", err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if log := logs.String(); !strings.Contains(log, "auth=bearer") || strings.Contains(log, "key-alice") || strings.Contains(log, "jwt-") {
		t.Errorf("log:\n%s", log)
	}
}
