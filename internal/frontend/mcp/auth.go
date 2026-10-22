package mcp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// ProtectedResourceMetadataPath is where the OAuth 2.0 protected resource
// metadata (RFC 9728 § 3) of the MCP endpoint is served: the well-known path
// with the endpoint's path appended, the form the protocol names first.
const ProtectedResourceMetadataPath = "/.well-known/oauth-protected-resource" + HTTPPath

// TokenEnv is the environment variable a bearer token is read from when no
// flag names one.
const TokenEnv = "OPENSYSML_MCP_TOKEN" // #nosec G101 -- a variable name, not a credential

// BearerToken is the one access token the HTTP transport accepts: the server
// is an OAuth 2.1 resource server (RFC 6750) whose token is issued out of band
// by whoever runs it, rather than by an authorization server. The zero value
// is no token, which only a loopback listener may serve without.
type BearerToken struct {
	secret []byte
	// public is the endpoint's URL as clients reach it, when configured;
	// otherwise each request's scheme and Host name it.
	public *url.URL
}

// NewBearerToken reads a token the way RFC 6750 § 2.1 spells one (b64token:
// letters, digits, -._~+/ and trailing =), so a client can send what the
// server was given; an empty token or one with other characters is refused.
func NewBearerToken(s string) (BearerToken, error) {
	if s == "" {
		return BearerToken{}, errors.New("the token is empty")
	}
	if !isB64Token(s) {
		return BearerToken{}, errors.New("a token is letters, digits and -._~+/ with trailing = only (RFC 6750 b64token), with no spaces or line breaks")
	}
	return BearerToken{secret: []byte(s)}, nil
}

// Set reports whether a token is configured.
func (t BearerToken) Set() bool { return len(t.secret) > 0 }

// At names the URL clients reach the endpoint at — behind a proxy that
// terminates TLS or rewrites the path, where a request no longer shows it —
// for the resource identifier and the challenge's metadata URL. Empty keeps
// deriving both from each request.
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
	t.public = u
	return t, nil
}

// PublicURL is the endpoint's URL as At named it, or empty.
func (t BearerToken) PublicURL() string {
	if t.public == nil {
		return ""
	}
	return t.public.String()
}

// matches compares a presented token in constant time.
func (t BearerToken) matches(presented string) bool {
	return t.Set() && subtle.ConstantTimeCompare(t.secret, []byte(presented)) == 1
}

// isB64Token reports whether s is a b64token of RFC 6750 § 2.1.
func isB64Token(s string) bool {
	body := strings.TrimRight(s, "=")
	if body == "" {
		return false
	}
	for _, r := range body {
		switch {
		case 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z', '0' <= r && r <= '9':
		case r == '-', r == '.', r == '_', r == '~', r == '+', r == '/':
		default:
			return false
		}
	}
	return true
}

// Handler requires every request to carry the token as RFC 6750 § 2.1
// spells it (Authorization: Bearer TOKEN) and refuses any other with 401 and
// a WWW-Authenticate challenge naming the resource metadata (RFC 6750 § 3,
// RFC 9728 § 5.1), or 400 when the header is malformed; with no token
// configured, next is served as is.
func (t BearerToken) Handler(next http.Handler) http.Handler {
	if !t.Set() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, credentials, _ := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
		credentials = strings.TrimSpace(credentials)
		switch {
		case scheme == "":
			t.challenge(w, r, http.StatusUnauthorized, "", "")
		case !strings.EqualFold(scheme, "Bearer"):
			t.challenge(w, r, http.StatusUnauthorized, "invalid_request", "the Authorization scheme must be Bearer")
		case !isB64Token(credentials):
			t.challenge(w, r, http.StatusBadRequest, "invalid_request", "the bearer token is malformed")
		case !t.matches(credentials):
			t.challenge(w, r, http.StatusUnauthorized, "invalid_token", "the bearer token is not this server's")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// challenge writes the refusal: the WWW-Authenticate header a client reads
// to find the resource metadata, with the error code RFC 6750 § 3.1 gives
// the failure, and none when no credentials were presented.
func (t BearerToken) challenge(w http.ResponseWriter, r *http.Request, status int, code, description string) {
	params := []string{`realm="sysml-mcp"`, fmt.Sprintf("resource_metadata=%q", t.resourceMetadataURL(r))}
	if code != "" {
		params = append(params, fmt.Sprintf("error=%q", code), fmt.Sprintf("error_description=%q", description))
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
	w.Header().Set("Cache-Control", "no-store")
	message := "Unauthorized: this server requires a bearer token; send Authorization: Bearer TOKEN"
	if description != "" {
		message = http.StatusText(status) + ": " + description
	}
	http.Error(w, message, status)
}

// ServeMetadata answers the protected resource metadata (RFC 9728 § 3.2) of
// the MCP endpoint: its identifier and that the token is sent in the header.
// There is no authorization server to list; the token is issued out of band.
func (t BearerToken) ServeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(&oauthex.ProtectedResourceMetadata{
		Resource:               t.resourceURL(r),
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "sysml-mcp",
		ResourceDocumentation:  "https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/sysml-mcp.md",
	})
}

// endpoint is the URL of the MCP endpoint as the client reached it: the one
// At named, or the scheme the connection used and the Host header it sent.
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

// resourceMetadataURL is where the client finds the resource metadata of the
// endpoint: the well-known path with the endpoint's path appended (RFC 9728 § 3.1).
func (t BearerToken) resourceMetadataURL(r *http.Request) string {
	u := *t.endpoint(r)
	u.Path = "/.well-known/oauth-protected-resource" + u.Path
	return u.String()
}
