package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/flags"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

// httpOptions is what -http and its qualifiers (-allow-origin, the token flags, -api-keys,
// -public-url, the TLS pair, -shutdown-timeout, -max-body-bytes) resolve to; nil without -http.
type httpOptions struct {
	addr    string
	origins mcp.OriginPolicy
	token   mcp.BearerToken
	// auth is the chain the token verifies through, for a SIGHUP to reload.
	auth            flags.Authenticators
	certFile        string
	keyFile         string
	shutdownTimeout time.Duration
	maxBodyBytes    int64
}

// readHTTPOptions validates the transport flags together (qualifiers without
// -http, -stdio beside it, a half TLS pair, a bad origin, a bad token);
// loadTLS loads the pair.
func readHTTPOptions(o *options) (*httpOptions, error) {
	if o.http == "" {
		auth := o.auth.Given()
		for _, f := range []struct{ name, value string }{{"-allow-origin", o.allowOrigin}, {auth, auth}, {"-public-url", o.publicURL}, {"-tls-cert", o.tlsCert}, {"-tls-key", o.tlsKey}} {
			if f.value != "" {
				return nil, fmt.Errorf("%s applies to the HTTP transport; name its address with -http", f.name)
			}
		}
		for _, name := range []string{"shutdown-timeout", "max-body-bytes"} {
			if o.given[name] {
				return nil, fmt.Errorf("-%s applies to the HTTP transport; name its address with -http", name)
			}
		}
		return nil, nil
	}
	if o.shutdownTimeout <= 0 {
		return nil, fmt.Errorf("-shutdown-timeout %s: a stopping server needs a positive time to drain in", o.shutdownTimeout)
	}
	if o.maxBodyBytes < 1 {
		return nil, fmt.Errorf("-max-body-bytes %d: a request needs room for at least one byte", o.maxBodyBytes)
	}
	if o.stdio {
		return nil, errors.New("-stdio and -http name different transports; give one")
	}
	addr, err := httpAddress(o.http)
	if err != nil {
		return nil, fmt.Errorf("-http: %w", err)
	}
	origins, err := mcp.ParseOrigins(o.allowOrigin)
	if err != nil {
		return nil, fmt.Errorf("-allow-origin: %w", err)
	}
	token, auth, err := readToken(o)
	if err != nil {
		return nil, err
	}
	if token, err = token.At(o.publicURL); err != nil {
		return nil, fmt.Errorf("-public-url: %w", err)
	}
	if (o.tlsCert == "") != (o.tlsKey == "") {
		return nil, errors.New("-tls-cert and -tls-key must be supplied together")
	}
	return &httpOptions{addr: addr, origins: origins, token: token, auth: auth, certFile: o.tlsCert, keyFile: o.tlsKey,
		shutdownTimeout: o.shutdownTimeout, maxBodyBytes: o.maxBodyBytes}, nil
}

// tokenValue is the token a -print-config block carries; empty without one.
func (h *httpOptions) tokenValue() string { return h.auth.TokenText() }

// tokenFileEnv names the token file when neither token flag is given, as
// mcp.TokenEnv names the token itself; the file wins, as -token-file does.
var tokenFileEnv = usage.EnvName(envPrefix, "token-file")

// readToken resolves the credentials: the API keys of -api-keys ahead of the token of
// -token-file, -token or the environment (the file first), a principal of every scope.
func readToken(o *options) (mcp.BearerToken, flags.Authenticators, error) {
	selected, fromEnv := *o.auth, false
	if selected.TokenFile == "" && selected.Token == "" {
		selected.TokenFile, fromEnv = os.Getenv(tokenFileEnv), true
	}
	resolved, err := selected.Resolve(mcp.TokenEnv, catalog.ScopeNames()...)
	if err != nil {
		if fromEnv && selected.TokenFile != "" {
			err = fmt.Errorf("%s: %w", tokenFileEnv, err)
		}
		return mcp.BearerToken{}, flags.Authenticators{}, err
	}
	return mcp.BearerToken{Authenticator: resolved.Chain()}, resolved, nil
}

// loadTLS proves the TLS pair loads before anything is served; nothing to
// prove without -http or without a pair.
func (h *httpOptions) loadTLS() error {
	if h == nil || h.certFile == "" {
		return nil
	}
	if _, err := tls.LoadX509KeyPair(h.certFile, h.keyFile); err != nil {
		return fmt.Errorf("-tls-cert %q with -tls-key %q: %w", h.certFile, h.keyFile, err)
	}
	return nil
}

// url is where a client reaches the server: -public-url when given, else the
// bound address (localhost for every interface), https with a TLS pair.
func (h *httpOptions) url() string {
	if public := h.token.PublicURL(); public != "" {
		return public
	}
	return serverURL(h.addr, h.certFile != "")
}

// httpAddress reads -http into the address to bind: host:port as given, or
// loopback for a port alone or an address with no host.
func httpAddress(spec string) (string, error) { return guard.ListenAddress(spec) }
