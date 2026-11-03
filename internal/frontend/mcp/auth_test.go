package mcp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func TestNewBearerToken(t *testing.T) {
	for _, ok := range []string{"a", "s3cret-token_~.+/", "YWJj==", "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"} {
		if tok, err := NewBearerToken(ok); err != nil || !tok.Set() {
			t.Errorf("NewBearerToken(%q) = %v, %v", ok, tok.Set(), err)
		}
	}
	for _, bad := range []string{"", "=", "has space", "line\nbreak", "Bearer x", "tab\tx", "café", "quote\"x", "a=b"} {
		if tok, err := NewBearerToken(bad); err == nil || tok.Set() {
			t.Errorf("NewBearerToken(%q) accepted it", bad)
		}
	}
	var none BearerToken
	if none.Set() || none.Matches("") {
		t.Error("the zero token is set or matches the empty string")
	}
}

// challengeOf parses the WWW-Authenticate header's scheme and parameters.
func challengeOf(t *testing.T, rec *httptest.ResponseRecorder) (scheme string, params map[string]string) {
	t.Helper()
	header := rec.Header().Get("WWW-Authenticate")
	scheme, rest, _ := strings.Cut(header, " ")
	params = map[string]string{}
	for _, p := range strings.Split(rest, ", ") {
		k, v, ok := strings.Cut(p, "=")
		if !ok || !strings.HasPrefix(v, `"`) || !strings.HasSuffix(v, `"`) {
			t.Fatalf("WWW-Authenticate %q: parameter %q is not key=\"value\"", header, p)
		}
		params[k] = strings.Trim(v, `"`)
	}
	return scheme, params
}

func TestBearerTokenHandler(t *testing.T) {
	tok, err := NewBearerToken("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	var served int
	h := tok.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served++
		w.WriteHeader(http.StatusAccepted)
	}))
	do := func(authorization string, tlsConn bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
		req.Host = "models.example.org:8443"
		if tlsConn {
			req.TLS = &tls.ConnectionState{}
		}
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for _, good := range []string{"Bearer s3cret", "bearer s3cret", "BEARER  s3cret"} {
		if rec := do(good, false); rec.Code != http.StatusAccepted || rec.Header().Get("WWW-Authenticate") != "" {
			t.Errorf("%q: status %d, headers %v", good, rec.Code, rec.Header())
		}
	}
	if served != 3 {
		t.Errorf("served %d requests, want 3", served)
	}

	for _, tc := range []struct {
		authorization string
		status        int
		code          string
	}{
		{"", http.StatusUnauthorized, ""},
		{"Bearer wrong", http.StatusUnauthorized, "invalid_token"},
		{"Bearer s3cre", http.StatusUnauthorized, "invalid_token"},
		{"Bearer s3cret2", http.StatusUnauthorized, "invalid_token"},
		{"Bearer S3CRET", http.StatusUnauthorized, "invalid_token"},
		{"Basic czNjcmV0", http.StatusUnauthorized, "invalid_request"},
		{"Bearer", http.StatusBadRequest, "invalid_request"},
		{"Bearer s3cret extra", http.StatusBadRequest, "invalid_request"},
		{"Bearer s3c ret", http.StatusBadRequest, "invalid_request"},
	} {
		rec := do(tc.authorization, true)
		if rec.Code != tc.status {
			t.Errorf("%q: status %d, want %d: %s", tc.authorization, rec.Code, tc.status, rec.Body)
		}
		scheme, params := challengeOf(t, rec)
		if scheme != "Bearer" {
			t.Errorf("%q: challenge scheme %q, want Bearer", tc.authorization, scheme)
		}
		if params["resource_metadata"] != "https://models.example.org:8443"+ProtectedResourceMetadataPath {
			t.Errorf("%q: resource_metadata = %q", tc.authorization, params["resource_metadata"])
		}
		if params["error"] != tc.code {
			t.Errorf("%q: error = %q, want %q", tc.authorization, params["error"], tc.code)
		}
		if (tc.code != "") != (params["error_description"] != "") {
			t.Errorf("%q: error_description = %q beside error %q", tc.authorization, params["error_description"], tc.code)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%q: Cache-Control = %q", tc.authorization, rec.Header().Get("Cache-Control"))
		}
		if body := rec.Body.String(); strings.Contains(body, "s3cret") || strings.Contains(rec.Header().Get("WWW-Authenticate"), "s3cret") {
			t.Errorf("%q: the refusal leaks the token: %s %v", tc.authorization, body, rec.Header())
		}
	}
	if served != 3 {
		t.Errorf("a refused request was served: %d served", served)
	}

	// A cleartext request is challenged with its own scheme and host.
	if _, params := challengeOf(t, do("", false)); params["resource_metadata"] != "http://models.example.org:8443"+ProtectedResourceMetadataPath {
		t.Errorf("cleartext resource_metadata = %q", params["resource_metadata"])
	}

	// With no token the handler is the one it wraps.
	var none BearerToken
	passed := none.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	rec := httptest.NewRecorder()
	passed.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, HTTPPath, nil))
	if rec.Code != http.StatusAccepted || rec.Header().Get("WWW-Authenticate") != "" {
		t.Errorf("no token: status %d, headers %v", rec.Code, rec.Header())
	}
}

// TestBearerTokenAt checks a public URL stands in for the request's scheme and
// host in the challenge and the metadata, and that only an endpoint URL is taken.
func TestBearerTokenAt(t *testing.T) {
	tok, _ := NewBearerToken("s3cret")
	for _, bad := range []string{"models.example.org", "ftp://models.example.org/mcp", "https://", "https://user@models.example.org/mcp", "https://models.example.org/mcp?x=1", "https://models.example.org/mcp#f", "mailto:a@b", "https://models.example.org/team%25a/mcp", "https://models.example.org/{team}/mcp", "https://models.example.org/a b/mcp", "https://models.example.org/agents/mcp/", "https://models.example.org//mcp"} {
		if _, err := tok.At(bad); err == nil {
			t.Errorf("At(%q) accepted it", bad)
		}
	}
	none, err := tok.At("")
	if err != nil || none.PublicURL() != "" || !none.Set() {
		t.Errorf("At(\"\") = %q, %v", none.PublicURL(), err)
	}
	bare, err := tok.At("https://models.example.org")
	if err != nil || bare.PublicURL() != "https://models.example.org/mcp" {
		t.Errorf("At with no path = %q, %v; want the endpoint path appended", bare.PublicURL(), err)
	}
	// A proxy may expose the endpoint at the root, whose metadata is at the
	// bare well-known URI.
	root, err := tok.At("https://models.example.org/")
	if err != nil || root.PublicURL() != "https://models.example.org/" {
		t.Errorf("At(root) = %q, %v", root.PublicURL(), err)
	}
	if got := root.MetadataPaths(); !slices.Equal(got, []string{ProtectedResourceMetadataPath, "/.well-known/oauth-protected-resource"}) {
		t.Errorf("root MetadataPaths() = %v", got)
	}
	rec := httptest.NewRecorder()
	root.Handler(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, HTTPPath, nil))
	if _, params := challengeOf(t, rec); params["resource_metadata"] != "https://models.example.org/.well-known/oauth-protected-resource" {
		t.Errorf("root resource_metadata = %q", params["resource_metadata"])
	}
	public, err := tok.At("https://models.example.org/agents/mcp")
	if err != nil {
		t.Fatal(err)
	}
	// The request arrives over cleartext from the proxy, with its own Host.
	req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
	req.Host = "127.0.0.1:8765"
	rec = httptest.NewRecorder()
	public.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if _, params := challengeOf(t, rec); params["resource_metadata"] != "https://models.example.org/.well-known/oauth-protected-resource/agents/mcp" {
		t.Errorf("resource_metadata = %q", params["resource_metadata"])
	}
	rec = httptest.NewRecorder()
	public.ServeMetadata(rec, httptest.NewRequest(http.MethodGet, ProtectedResourceMetadataPath, nil))
	var meta map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta["resource"] != "https://models.example.org/agents/mcp" {
		t.Errorf("resource = %v", meta["resource"])
	}
}

func TestBearerTokenServeMetadata(t *testing.T) {
	tok, _ := NewBearerToken("s3cret")
	req := httptest.NewRequest(http.MethodGet, ProtectedResourceMetadataPath, nil)
	req.Host = "models.example.org:8443"
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	tok.ServeMetadata(rec, req)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, headers %v", rec.Code, rec.Header())
	}
	var meta map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("%v:\n%s", err, rec.Body)
	}
	if meta["resource"] != "https://models.example.org:8443/mcp" {
		t.Errorf("resource = %v", meta["resource"])
	}
	if methods, _ := meta["bearer_methods_supported"].([]any); !slices.Equal(methods, []any{"header"}) {
		t.Errorf("bearer_methods_supported = %v", meta["bearer_methods_supported"])
	}
	if _, has := meta["authorization_servers"]; has {
		t.Errorf("authorization_servers is listed with none to list: %v", meta)
	}
	if strings.Contains(rec.Body.String(), "s3cret") {
		t.Errorf("the metadata leaks the token:\n%s", rec.Body)
	}
	rec = httptest.NewRecorder()
	tok.ServeMetadata(rec, httptest.NewRequest(http.MethodPost, ProtectedResourceMetadataPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d", rec.Code)
	}
}

// TestHandlerMountsTheMetadataOnlyWithAToken checks that the well-known path
// is served, open, beside a protected endpoint, and is absent otherwise.
func TestHandlerMountsTheMetadataOnlyWithAToken(t *testing.T) {
	srv, err := New(context.Background(), Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test",
		NewSession: func() (*repl.Session, error) { return repl.NewSession(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close(context.Background())
	tok, _ := NewBearerToken("s3cret")
	get := func(h http.Handler, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	with := srv.Handler(OriginPolicy{}, tok)
	if rec := get(with, ProtectedResourceMetadataPath); rec.Code != http.StatusOK {
		t.Errorf("metadata with a token: status %d", rec.Code)
	}
	if rec := get(with, HTTPPath); rec.Code != http.StatusUnauthorized {
		t.Errorf("the endpoint with a token and no credentials: status %d", rec.Code)
	}
	without := srv.Handler(OriginPolicy{}, BearerToken{})
	if rec := get(without, ProtectedResourceMetadataPath); rec.Code != http.StatusNotFound {
		t.Errorf("metadata without a token: status %d", rec.Code)
	}

	// Behind a proxy exposing another path, the metadata is served at that
	// path's well-known URI too, whether or not the proxy rewrites it.
	public, _ := tok.At("https://models.example.org/agents/mcp")
	if got := public.MetadataPaths(); !slices.Equal(got, []string{ProtectedResourceMetadataPath, "/.well-known/oauth-protected-resource/agents/mcp"}) {
		t.Errorf("MetadataPaths() = %v", got)
	}
	proxied := srv.Handler(OriginPolicy{}, public)
	for _, path := range public.MetadataPaths() {
		rec := get(proxied, path)
		var meta map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &meta) != nil || meta["resource"] != "https://models.example.org/agents/mcp" {
			t.Errorf("metadata at %s: status %d, body %s", path, rec.Code, rec.Body)
		}
	}
	if got := tok.MetadataPaths(); !slices.Equal(got, []string{ProtectedResourceMetadataPath}) {
		t.Errorf("MetadataPaths() with no public URL = %v", got)
	}
}

// TestServeStreamableHTTPEnforcesTheTokenOnLoopback checks the protocol is
// served only to a client sending the token, on loopback too, and never logs it.
func TestServeStreamableHTTPEnforcesTheTokenOnLoopback(t *testing.T) {
	var logs bytes.Buffer
	tok, _ := NewBearerToken("s3cret-token")
	url, stop := serveHTTPWith(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, &logs, "127.0.0.1:0", tok)
	ctx := context.Background()
	connect := func(authorization string) (*sdk.ClientSession, error) {
		c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
		var client *http.Client
		if authorization != "" {
			client = &http.Client{Transport: headerTransport{"Authorization", authorization}}
		}
		return c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: url, HTTPClient: client}, nil)
	}
	cs, err := connect("Bearer s3cret-token")
	if err != nil {
		t.Fatalf("Connect with the token: %v", err)
	}
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools with the token: %v", err)
	}
	_ = cs.Close()
	for _, bad := range []string{"", "Bearer wrong"} {
		cs, err := connect(bad)
		if err == nil {
			_ = cs.Close()
			t.Errorf("Authorization %q was served", bad)
			continue
		}
		if !strings.Contains(err.Error(), "Unauthorized") {
			t.Errorf("Authorization %q: %v, want Unauthorized", bad, err)
		}
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if log := logs.String(); !strings.Contains(log, "auth=bearer") || strings.Contains(log, "s3cret") {
		t.Errorf("log:\n%s", log)
	}
}

// headerTransport sends one header on every request.
type headerTransport struct{ name, value string }

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(h.name, h.value)
	return http.DefaultTransport.RoundTrip(r)
}
