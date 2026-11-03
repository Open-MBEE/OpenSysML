package guard

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// IsLoopbackHost reports whether a host name or address is this machine's
// loopback interface.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsLoopbackAddr reports whether a bound address can be reached from this
// machine only.
func IsLoopbackAddr(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	return ok && tcp.IP != nil && tcp.IP.IsLoopback()
}

// ListenAddress reads a listening address as every service spells one: a port
// alone, or host:port whose host is empty, binds loopback (localhost).
func ListenAddress(spec string) (string, error) {
	if _, err := strconv.ParseUint(spec, 10, 16); err == nil {
		return net.JoinHostPort("localhost", spec), nil
	}
	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		return "", errors.New(strconv.Quote(spec) + " is not a port or a host:port address")
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", errors.New(strconv.Quote(port) + " is not a port")
	}
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port), nil
}

// NoToken is why a listener other machines reach is refused with no token;
// env is the variable the command reads one from.
func NoToken(env string) error {
	return errors.New("no bearer token is configured; name one with -token-file, -token or " + env + ", or bind loopback")
}
