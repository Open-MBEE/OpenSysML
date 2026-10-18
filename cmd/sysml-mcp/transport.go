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

// readHTTPOptions reads the transport flags as a whole: the ones that qualify
// -http are a usage error without it, as is a TLS pair with one half missing,
// a certificate and key that do not load, or an origin that is not one.
func readHTTPOptions(o *options) (*httpOptions, error) {
	if o.http == "" {
		for _, f := range []struct{ name, value string }{{"-allow-origin", o.allowOrigin}, {"-tls-cert", o.tlsCert}, {"-tls-key", o.tlsKey}} {
			if f.value != "" {
				return nil, fmt.Errorf("%s applies to the HTTP transport; name its address with -http", f.name)
			}
		}
		return nil, nil
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
	if o.tlsCert != "" {
		if _, err := tls.LoadX509KeyPair(o.tlsCert, o.tlsKey); err != nil {
			return nil, fmt.Errorf("-tls-cert %q with -tls-key %q: %w", o.tlsCert, o.tlsKey, err)
		}
	}
	return &httpOptions{addr: addr, origins: origins, certFile: o.tlsCert, keyFile: o.tlsKey}, nil
}

// httpAddress reads the -http flag into the address to bind: host:port as
// given, or loopback for a port alone or an address with no host, so that a
// server is reached from other machines only when its address says so.
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
