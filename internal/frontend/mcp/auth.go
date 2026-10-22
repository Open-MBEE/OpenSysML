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

// wellKnownPrefix heads the protected resource metadata path (RFC 9728 § 3):
// the endpoint's path is appended, the form the protocol names first.
const wellKnownPrefix = "/.well-known/oauth-protected-resource"

// ProtectedResourceMetadataPath is where the MCP endpoint's metadata is served.
const ProtectedResourceMetadataPath = wellKnownPrefix + HTTPPath

// TokenEnv is the environment variable a bearer token is read from when no
// flag names one.
const TokenEnv = "OPENSYSML_MCP_TOKEN" // #nosec G101 -- a variable name, not a credential

// BearerToken is the one access token the HTTP transport accepts (RFC 6750),
// issued out of band by whoever runs the server; the zero value is no token.
type BearerToken struct {
	secret []byte
	// public is the endpoint's URL as clients reach it; nil reads each request.
	public *url.URL
}

// NewBearerToken reads a token as RFC 6750 § 2.1 spells one (b64token), so a
// client can send what the server was given; anything else is refused.
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

// At names the URL clients reach the endpoint at behind a proxy, for the
// resource identifier and the challenge; empty derives both from each request.
func (t BearerToken) At(publicURL string) (BearerToken, error) {
	if publicURL == "" {
		t.public = nil
		return t, nil
	}
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return t, fmt.Errorf("%q is not the URL of the endpoint; it is http(s)://host[:port]/path with no user, query or fragment, as https://models.example.org/mcp", publicURL)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = HTTPPath
	}
	if !isPlainPath(u.Path) || u.EscapedPath() != u.Path || strings.HasSuffix(u.Path, "/") {
		return t, fmt.Errorf("%q has a path that is not a plain one; it is segments of letters, digits and -._~ with no trailing slash, as /agents/mcp", publicURL)
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

// Handler requires Authorization: Bearer TOKEN of every request (RFC 6750
// § 2.1), answering 401 with a challenge or 400 otherwise; no token serves next.
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

// challenge writes the refusal with the WWW-Authenticate header naming the
// metadata and the RFC 6750 § 3.1 error code (none when nothing was presented).
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

// ServeMetadata answers the endpoint's protected resource metadata (RFC 9728
// § 3.2); there is no authorization server to list, the token being out of band.
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
	u.Path = wellKnownPrefix + u.Path
	return u.String()
}

// MetadataPaths lists where the metadata is served: ProtectedResourceMetadataPath,
// and the public path's well-known URI when At named another path.
func (t BearerToken) MetadataPaths() []string {
	paths := []string{ProtectedResourceMetadataPath}
	if t.public != nil && t.public.Path != HTTPPath {
		paths = append(paths, wellKnownPrefix+t.public.Path)
	}
	return paths
}
