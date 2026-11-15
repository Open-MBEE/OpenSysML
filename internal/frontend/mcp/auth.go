package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// wellKnownPrefix heads the protected resource metadata path (RFC 9728 § 3):
// the endpoint's path is appended, the form the protocol names first.
const wellKnownPrefix = "/.well-known/oauth-protected-resource"

// ProtectedResourceMetadataPath is where the MCP endpoint's metadata is served.
const ProtectedResourceMetadataPath = wellKnownPrefix + HTTPPath

// TokenEnv is the environment variable a bearer token is read from when no
// flag names one.
const TokenEnv = "OPENSYSML_MCP_TOKEN" // #nosec G101 -- a variable name, not a credential

// BearerToken is whose bearer credentials the HTTP transport accepts (RFC 6750): a token
// issued out of band, an Authenticator's, or both, with the endpoint it protects.
type BearerToken struct {
	guard.Token
	// Authenticator, if set, names the principals of the credentials it reads, ahead of
	// the token; a guard.Chain of none, as flags resolve to with none given, is not set.
	Authenticator guard.Authenticator
	// public is the endpoint's URL as clients reach it; nil reads each request.
	public *url.URL
}

// NewBearerToken reads a token as RFC 6750 § 2.1 spells one (b64token), so a
// client can send what the server was given; anything else is refused.
func NewBearerToken(s string) (BearerToken, error) {
	token, err := guard.New(s)
	if err != nil {
		return BearerToken{}, err
	}
	return BearerToken{Token: token}, nil
}

// At names the URL clients reach the endpoint at behind a proxy (an origin
// alone means its /mcp, / the root); empty derives it from each request.
func (t BearerToken) At(publicURL string) (BearerToken, error) {
	if publicURL == "" {
		t.public = nil
		return t, nil
	}
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return t, fmt.Errorf("%q is not the URL of the endpoint; it is http(s)://host[:port]/path with no user, query or fragment, as https://models.example.org/mcp", publicURL)
	}
	if u.Path == "" {
		u.Path = HTTPPath
	}
	if !isPlainPath(u.Path) || u.EscapedPath() != u.Path || (u.Path != "/" && strings.HasSuffix(u.Path, "/")) {
		return t, fmt.Errorf("%q has a path that is not a plain one; it is / or segments of letters, digits and -._~ with no trailing slash, as /agents/mcp", publicURL)
	}
	t.public = u
	return t, nil
}

// isPlainPath reports whether p is /-separated segments of unreserved
// characters, which a mux pattern and a well-known URI both take as written.
func isPlainPath(p string) bool {
	for _, r := range p {
		switch {
		case 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z', '0' <= r && r <= '9':
		case r == '-', r == '.', r == '_', r == '~', r == '/':
		default:
			return false
		}
	}
	return strings.HasPrefix(p, "/") && !strings.Contains(p, "//")
}

// PublicURL is the endpoint's URL as At named it, or empty.
func (t BearerToken) PublicURL() string {
	if t.public == nil {
		return ""
	}
	return t.public.String()
}

// Set reports whether a request has to authenticate, with the token or to the Authenticator.
func (t BearerToken) Set() bool { return len(t.authenticators()) > 0 }

// authenticators are who a request authenticates to, in the order asked: the Authenticator,
// then the token as an all-scopes guard.StaticToken, which claims every bearer token.
func (t BearerToken) authenticators() guard.Chain {
	var chain guard.Chain
	switch a := t.Authenticator.(type) {
	case nil:
	case guard.Chain:
		chain = append(chain, a...)
	case *guard.Chain:
		if a != nil {
			chain = append(chain, *a...)
		}
	default:
		chain = append(chain, a)
	}
	if t.Token.Set() {
		chain = append(chain, guard.NewStaticToken(t.Token, catalog.ScopeNames()...))
	}
	return chain
}

// Handler serves next the requests an authenticator accepts (RFC 6750 § 2.1), as their
// principal, and answers a refusal as § 3.1 has it; with nothing set it serves every request.
func (t BearerToken) Handler(next http.Handler) http.Handler {
	if !t.Set() {
		return next
	}
	chain := t.authenticators()
	next = carryPrincipal(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, refusal := chain.Verify(r.Context(), r.Header.Get("Authorization"), r)
		if refusal == guard.Accepted && !p.Expires.IsZero() && !p.Expires.After(time.Now()) {
			refusal = guard.Expired
		}
		if refusal != guard.Accepted {
			t.challenge(w, r, refusal)
			return
		}
		next.ServeHTTP(w, r.WithContext(guard.WithPrincipal(r.Context(), p)))
	})
}

// carryPrincipal is the SDK's bearer middleware over a request already verified:
// it lifts the context's principal into the TokenInfo the SDK binds the session to.
var carryPrincipal = auth.RequireBearerToken(func(ctx context.Context, _ string, _ *http.Request) (*auth.TokenInfo, error) {
	p, ok := guard.PrincipalFrom(ctx)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	return TokenInfoFor(p), nil
}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})

// challenge writes the refusal with the WWW-Authenticate header naming the
// metadata and the RFC 6750 § 3.1 error code, when the refusal has one.
func (t BearerToken) challenge(w http.ResponseWriter, r *http.Request, refusal guard.Refusal) {
	params := []string{`realm="sysml-mcp"`, fmt.Sprintf("resource_metadata=%q", t.resourceMetadataURL(r))}
	params = append(params, refusal.ChallengeParams()...)
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, refusal.Message(), refusal.HTTPStatus())
}

// ServeMetadata answers the endpoint's protected resource metadata (RFC 9728 § 3.2), with
// the authorization servers and scopes the Authenticator advertises; the token has none.
func (t BearerToken) ServeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	advertised := t.authenticators().Metadata()
	_ = json.NewEncoder(w).Encode(&oauthex.ProtectedResourceMetadata{
		Resource:               t.resourceURL(r),
		AuthorizationServers:   advertised.AuthorizationServers,
		ScopesSupported:        advertised.ScopesSupported,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "sysml-mcp",
		ResourceDocumentation:  "https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/sysml-mcp.md",
	})
}

// endpoint is the MCP endpoint's URL as the client reached it: the one At
// named, or the connection's scheme and the request's Host.
func (t BearerToken) endpoint(r *http.Request) *url.URL {
	if t.public != nil {
		return t.public
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return &url.URL{Scheme: scheme, Host: r.Host, Path: HTTPPath}
}

// resourceURL is the identifier of the MCP endpoint (RFC 8707 § 2).
func (t BearerToken) resourceURL(r *http.Request) string {
	return t.endpoint(r).String()
}

// resourceMetadataURL is where the client finds the endpoint's metadata:
// the well-known prefix with the endpoint's path appended (RFC 9728 § 3.1).
func (t BearerToken) resourceMetadataURL(r *http.Request) string {
	u := *t.endpoint(r)
	u.Path = wellKnownPrefix + strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// MetadataPaths lists where the metadata is served: ProtectedResourceMetadataPath,
// and the public path's well-known URI when At named another path.
func (t BearerToken) MetadataPaths() []string {
	paths := []string{ProtectedResourceMetadataPath}
	if t.public != nil && t.public.Path != HTTPPath {
		paths = append(paths, wellKnownPrefix+strings.TrimSuffix(t.public.Path, "/"))
	}
	return paths
}
