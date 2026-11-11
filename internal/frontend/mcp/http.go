package mcp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPPath is the path the Streamable HTTP transport is served at.
const HTTPPath = "/mcp"

// DefaultShutdownTimeout bounds how long a stopping server waits for the
// connections it is closing.
const DefaultShutdownTimeout = 30 * time.Second

// HTTPConfig is how a server is served over the Streamable HTTP transport.
type HTTPConfig struct {
	// Listener is the bound address connections arrive on.
	Listener net.Listener
	// Origins decides which Origin headers are admitted.
	Origins OriginPolicy
	// Token is whose bearer credentials a request must carry, a token's or an authenticator's;
	// the zero value requires none, which only a loopback listener may serve.
	Token BearerToken
	// CertFile and KeyFile, both set, serve TLS; neither serves cleartext.
	CertFile, KeyFile string
	// ShutdownTimeout bounds the graceful shutdown; 0 is DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
	// MaxBodyBytes bounds a request body on the protocol path, answered 413
	// past it; 0 is the SDK's DefaultMaxBodyBytes, and a negative value no bound.
	MaxBodyBytes int64
	// Logger, if set, receives the transport's log; nil logs to the default.
	Logger *slog.Logger
}

// DefaultMaxBodyBytes is the request body bound when none is configured.
const DefaultMaxBodyBytes = sdk.DefaultMaxRequestBodyBytes

// OriginPolicy admits a request with no Origin, a loopback origin or one allowed
// by name, and refuses any other, so a rebound DNS name is not a way in.
type OriginPolicy struct {
	allowed map[string]struct{}
}

// ParseOrigins reads a comma-separated list of exact origins, each
// scheme://host[:port], into a policy; the wildcard is refused.
func ParseOrigins(spec string) (OriginPolicy, error) {
	p := OriginPolicy{allowed: map[string]struct{}{}}
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == "*" {
			return OriginPolicy{}, errors.New("the wildcard origin is not allowed; name each origin")
		}
		origin, ok := normalizeOrigin(item)
		if !ok {
			return OriginPolicy{}, fmt.Errorf("%q is not an origin; an origin is scheme://host[:port], as https://agents.example.org", item)
		}
		p.allowed[origin] = struct{}{}
	}
	return p, nil
}

// Allowed lists the origins admitted by name, sorted, for the log.
func (p OriginPolicy) Allowed() []string {
	names := make([]string, 0, len(p.allowed))
	for o := range p.allowed {
		names = append(names, o)
	}
	slices.Sort(names)
	return names
}

// Allows reports whether a request bearing origin (the Origin header's value,
// empty when absent) is admitted.
func (p OriginPolicy) Allows(origin string) bool {
	if origin == "" {
		return true
	}
	normalized, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}
	if _, ok := p.allowed[normalized]; ok {
		return true
	}
	u, _ := url.Parse(normalized)
	return guard.IsLoopbackHost(u.Hostname())
}

// Named reports whether origin is one the policy admits by name, as opposed
// to admitting it for being loopback.
func (p OriginPolicy) Named(origin string) bool {
	normalized, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}
	_, named := p.allowed[normalized]
	return named
}

// The CORS grant a named origin's browser is given: the transport's methods,
// request headers and the response headers it reads.
const (
	corsAllowMethods  = "GET, POST, DELETE, OPTIONS"
	corsAllowHeaders  = "Content-Type, Accept, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID, Authorization"
	corsExposeHeaders = "Mcp-Session-Id, MCP-Protocol-Version"
)

// Handler refuses origins the policy does not admit with 403 and grants CORS
// only to origins admitted by name; a loopback origin is served without a grant.
func (p OriginPolicy) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !p.Allows(origin) {
			http.Error(w, fmt.Sprintf("Forbidden: origin %q is not allowed; this server answers loopback origins and those named by -allow-origin", origin), http.StatusForbidden)
			return
		}
		w.Header().Add("Vary", "Origin")
		switch {
		case p.Named(origin):
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case origin != "" && r.Method == http.MethodOptions:
			http.Error(w, fmt.Sprintf("Forbidden: origin %q is admitted but not granted cross-origin access; name it with -allow-origin", origin), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// normalizeOrigin reduces an origin to scheme://host[:port] with the scheme
// and host in lower case; anything that is not just that is not an origin.
func normalizeOrigin(s string) (string, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Opaque != "" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" {
		host += ":" + port
	}
	return strings.ToLower(u.Scheme) + "://" + host, true
}

// IsLoopbackAddr reports whether a bound address can be reached from this
// machine only; an address on every interface cannot.
func IsLoopbackAddr(addr net.Addr) bool { return guard.IsLoopbackAddr(addr) }

// protocolHandler is the Streamable HTTP transport of this server behind the
// token, with the request body bounded as over asks.
func (s *Server) protocolHandler(over HTTPConfig) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.sdk },
		&sdk.StreamableHTTPOptions{Logger: s.logger, MaxRequestBodyBytes: over.MaxBodyBytes})
	return over.Token.Handler(stampRemoteAddr(streamable))
}

// ErrNoToken is why a listener other machines reach is refused with no token.
var ErrNoToken = guard.NoToken(TokenEnv)

// describe names the transport's security and authentication for the log,
// warning when a token is about to cross the network in the clear.
func (c HTTPConfig) describe(addr net.Addr, logger *slog.Logger) (security, auth string) {
	security, auth = "cleartext", "none"
	if c.CertFile != "" {
		security = "TLS"
	}
	if c.Token.Set() {
		auth = "bearer"
		if c.CertFile == "" && !IsLoopbackAddr(addr) {
			logger.Warn("serving the bearer token over cleartext on an address other machines reach: a network observer can read and reuse it; serve TLS with -tls-cert and -tls-key, or terminate TLS in front",
				"addr", addr.String())
		}
	}
	return security, auth
}

// tlsConfig is the TLS the transport serves: 1.2 or later, with HTTP/2.
func tlsConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}
}
