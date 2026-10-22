package mcp

import (
	"context"
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
	// Token is the bearer token every request must carry; the zero value
	// requires none, which only a loopback listener may serve.
	Token BearerToken
	// CertFile and KeyFile, both set, serve TLS; neither serves cleartext.
	CertFile, KeyFile string
	// ShutdownTimeout bounds the graceful shutdown; 0 is DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
}

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
	return isLoopbackHost(u.Hostname())
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

// isLoopbackHost reports whether a host name or address is this machine's
// loopback: localhost or a loopback IP.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsLoopbackAddr reports whether a bound address can be reached from this
// machine only; an address on every interface cannot.
func IsLoopbackAddr(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	return ok && tcp.IP != nil && tcp.IP.IsLoopback()
}

// Handler is the Streamable HTTP transport of this server, mounted at HTTPPath
// behind origins and token; every request is served by the one SDK server.
// With a token, the endpoint's resource metadata is served beside it, open.
func (s *Server) Handler(origins OriginPolicy, token BearerToken) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.sdk }, &sdk.StreamableHTTPOptions{Logger: s.logger})
	mux := http.NewServeMux()
	mux.Handle(HTTPPath, origins.Handler(token.Handler(streamable)))
	if token.Set() {
		mux.Handle(ProtectedResourceMetadataPath, origins.Handler(http.HandlerFunc(token.ServeMetadata)))
	}
	return mux
}

// ErrNoToken is why a listener other machines reach is refused with no token.
var ErrNoToken = errors.New("no bearer token is configured; name one with -token-file, -token or " + TokenEnv + ", or bind loopback")

// ServeStreamableHTTP serves clients over cfg.Listener until ctx ends, then
// closes the live sessions and drains the connections within the timeout.
func (s *Server) ServeStreamableHTTP(ctx context.Context, cfg HTTPConfig) error {
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("a certificate file and a key file are both needed for TLS")
	}
	addr := cfg.Listener.Addr()
	if !IsLoopbackAddr(addr) && !cfg.Token.Set() {
		return fmt.Errorf("%s is an address other machines reach: %w", addr, ErrNoToken)
	}
	srv := &http.Server{Handler: s.Handler(cfg.Origins, cfg.Token), ReadHeaderTimeout: 10 * time.Second}
	security := "cleartext"
	if cfg.CertFile != "" {
		security = "TLS"
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}
	}
	auth := "none"
	if cfg.Token.Set() {
		auth = "bearer"
	}
	s.log().Info("serving the Model Context Protocol over Streamable HTTP",
		"addr", addr.String(), "path", HTTPPath, "security", security, "auth", auth, "allowed_origins", cfg.Origins.Allowed())

	errs := make(chan error, 1)
	go func() {
		if cfg.CertFile != "" {
			errs <- srv.ServeTLS(cfg.Listener, cfg.CertFile, cfg.KeyFile)
			return
		}
		errs <- srv.Serve(cfg.Listener)
	}()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	timeout := cfg.ShutdownTimeout
	if timeout == 0 {
		timeout = DefaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Shutdown(shutdownCtx) }()
	// A client's event stream stays open until its session ends, so the
	// sessions are closed for the shutdown to see the connections idle.
	for sess := range s.sdk.Sessions() {
		_ = sess.Close()
	}
	if err := <-done; err != nil {
		_ = srv.Close()
		return fmt.Errorf("stopping the HTTP server: %w", err)
	}
	s.log().Info("stopped serving over Streamable HTTP")
	return nil
}

// log is the server's logger, or the default when the config set none.
func (s *Server) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}
