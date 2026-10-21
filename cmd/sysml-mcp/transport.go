package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
)

// httpOptions is what the -http, -allow-origin, -tls-cert and -tls-key flags
// resolve to once read together; nil when -http was not given.
type httpOptions struct {
	addr     string
	origins  mcp.OriginPolicy
	certFile string
	keyFile  string
}

// readHTTPOptions validates the transport flags together (qualifiers without
// -http, -stdio beside it, a half TLS pair, a bad origin); loadTLS loads the pair.
func readHTTPOptions(o *options) (*httpOptions, error) {
	if o.http == "" {
		for _, f := range []struct{ name, value string }{{"-allow-origin", o.allowOrigin}, {"-tls-cert", o.tlsCert}, {"-tls-key", o.tlsKey}} {
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
	if (o.tlsCert == "") != (o.tlsKey == "") {
		return nil, errors.New("-tls-cert and -tls-key must be supplied together")
	}
	return &httpOptions{addr: addr, origins: origins, certFile: o.tlsCert, keyFile: o.tlsKey}, nil
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
