package main

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
)

// httpOptions is what -http and its qualifiers (-allow-origin, -token,
// -token-file, -public-url, -tls-cert, -tls-key) resolve to; nil without -http.
type httpOptions struct {
	addr    string
	origins mcp.OriginPolicy
	token   mcp.BearerToken
	// tokenText is the token as given, for the -print-config block alone.
	tokenText string
	certFile  string
	keyFile   string
}

// readHTTPOptions validates the transport flags together (qualifiers without
// -http, -stdio beside it, a half TLS pair, a bad origin, a bad token);
// loadTLS loads the pair.
func readHTTPOptions(o *options) (*httpOptions, error) {
	if o.http == "" {
		for _, f := range []struct{ name, value string }{{"-allow-origin", o.allowOrigin}, {"-token", o.token}, {"-token-file", o.tokenFile}, {"-public-url", o.publicURL}, {"-tls-cert", o.tlsCert}, {"-tls-key", o.tlsKey}} {
			if f.value != "" {
				return nil, fmt.Errorf("%s applies to the HTTP transport; name its address with -http", f.name)
			}
		}
		return nil, nil
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
	token, text, err := readToken(o)
	if err != nil {
		return nil, err
	}
	if token, err = token.At(o.publicURL); err != nil {
		return nil, fmt.Errorf("-public-url: %w", err)
	}
	if (o.tlsCert == "") != (o.tlsKey == "") {
		return nil, errors.New("-tls-cert and -tls-key must be supplied together")
	}
	return &httpOptions{addr: addr, origins: origins, token: token, tokenText: text, certFile: o.tlsCert, keyFile: o.tlsKey}, nil
}

// tokenValue is the token a -print-config block carries; empty without one.
func (h *httpOptions) tokenValue() string { return h.tokenText }

// readToken reads the token from -token-file (one trailing line break dropped),
// -token or the environment, in that order; both flags together are refused.
func readToken(o *options) (mcp.BearerToken, string, error) {
	token, text, err := guard.Read(o.tokenFile, o.token, mcp.TokenEnv)
	if err != nil {
		return mcp.BearerToken{}, "", err
	}
	return mcp.BearerToken{Token: token}, text, nil
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
