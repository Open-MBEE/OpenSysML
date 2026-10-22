package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
)

// httpOptions is what the -http, -allow-origin, -token, -token-file, -tls-cert
// and -tls-key flags resolve to once read together; nil when -http was not given.
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
		for _, f := range []struct{ name, value string }{{"-allow-origin", o.allowOrigin}, {"-token", o.token}, {"-token-file", o.tokenFile}, {"-tls-cert", o.tlsCert}, {"-tls-key", o.tlsKey}} {
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
	if (o.tlsCert == "") != (o.tlsKey == "") {
		return nil, errors.New("-tls-cert and -tls-key must be supplied together")
	}
	return &httpOptions{addr: addr, origins: origins, token: token, tokenText: text, certFile: o.tlsCert, keyFile: o.tlsKey}, nil
}

// tokenValue is the token a -print-config block carries; empty without one.
func (h *httpOptions) tokenValue() string { return h.tokenText }

// readToken reads the bearer token from -token-file, -token or the
// environment, in that order of preference; the two flags together are
// refused, a file's trailing line break is not part of the token, and an
// empty flag or variable names none.
func readToken(o *options) (mcp.BearerToken, string, error) {
	if o.token != "" && o.tokenFile != "" {
		return mcp.BearerToken{}, "", errors.New("-token and -token-file both name the bearer token; give one")
	}
	source, value := "-token", o.token
	switch {
	case o.tokenFile != "":
		raw, err := os.ReadFile(o.tokenFile)
		if err != nil {
			return mcp.BearerToken{}, "", fmt.Errorf("-token-file: %w", err)
		}
		source, value = fmt.Sprintf("-token-file %q", o.tokenFile), strings.TrimRight(string(raw), "\r\n")
	case o.token == "":
		env := os.Getenv(mcp.TokenEnv)
		if env == "" {
			return mcp.BearerToken{}, "", nil
		}
		source, value = mcp.TokenEnv, env
	}
	token, err := mcp.NewBearerToken(value)
	if err != nil {
		return mcp.BearerToken{}, "", fmt.Errorf("%s: %w", source, err)
	}
	return token, value, nil
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

// url is where a client reaches the server: https with a TLS pair, and
// localhost where the address binds every interface or loopback alone.
func (h *httpOptions) url() string {
	return serverURL(h.addr, h.certFile != "")
}

// httpAddress reads -http into the address to bind: host:port as given, or
// loopback for a port alone or an address with no host.
func httpAddress(spec string) (string, error) {
	if _, err := strconv.ParseUint(spec, 10, 16); err == nil {
		return net.JoinHostPort("localhost", spec), nil
	}
	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		return "", fmt.Errorf("%q is not a port or a host:port address", spec)
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", fmt.Errorf("%q is not a port", port)
	}
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port), nil
}
