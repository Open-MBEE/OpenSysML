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
	// CertFile and KeyFile, both set, serve TLS; neither serves cleartext.
	CertFile, KeyFile string
	// ShutdownTimeout bounds the graceful shutdown; 0 is DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
}

// OriginPolicy decides which browser origins the HTTP transport answers, which
// is what keeps a page on another site from reaching a server on this machine
// through a rebound DNS name: a request with no Origin header is admitted, as
// is one from a loopback origin (localhost, 127.0.0.0/8, ::1) and one from an
// origin allowed by name; any other is refused.
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

// Handler refuses requests whose origin the policy does not admit, with 403,
// and passes the rest to next.
func (p OriginPolicy) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); !p.Allows(origin) {
			http.Error(w, fmt.Sprintf("Forbidden: origin %q is not allowed; this server answers loopback origins and those named by -allow-origin", origin), http.StatusForbidden)
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

// isLoopbackAddr reports whether a bound address can be reached from this
// machine only; an address on every interface cannot.
func isLoopbackAddr(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	return ok && tcp.IP != nil && tcp.IP.IsLoopback()
}

// Handler is the Streamable HTTP transport of this server, mounted at HTTPPath
// behind the origin policy; every session it opens shares the server's
// workspace, runtime cache and REPL sessions.
func (s *Server) Handler(origins OriginPolicy) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.sdk }, &sdk.StreamableHTTPOptions{Logger: s.logger})
	mux := http.NewServeMux()
	mux.Handle(HTTPPath, origins.Handler(streamable))
	return mux
}

// ServeStreamableHTTP serves clients over cfg.Listener until ctx ends, then
// stops: the listener closes, the live sessions are closed so their streams
// end, and the connections are given cfg.ShutdownTimeout to drain.
func (s *Server) ServeStreamableHTTP(ctx context.Context, cfg HTTPConfig) error {
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("a certificate file and a key file are both needed for TLS")
	}
	srv := &http.Server{Handler: s.Handler(cfg.Origins), ReadHeaderTimeout: 10 * time.Second}
	security := "cleartext"
	if cfg.CertFile != "" {
		security = "TLS"
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}
	}
	addr := cfg.Listener.Addr()
	if !isLoopbackAddr(addr) {
		s.log().Warn("serving on a non-loopback address with no authentication: every client that reaches it may read and edit the workspace; bind loopback or put an authenticating proxy in front",
			"addr", addr.String())
	}
	s.log().Info("serving the Model Context Protocol over Streamable HTTP",
		"addr", addr.String(), "path", HTTPPath, "security", security, "allowed_origins", cfg.Origins.Allowed())

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
