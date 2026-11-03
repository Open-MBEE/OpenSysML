package guard

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	for _, s := range []string{"abc", "a-b.c_d~e+f/g", "abc==", "0"} {
		token, err := New(s)
		if err != nil || !token.Set() || !token.Matches(s) {
			t.Errorf("New(%q) = %v, %v; want a set token matching itself", s, token.Set(), err)
		}
	}
	for _, s := range []string{"", "has space", "line\nbreak", "===", "ab=c", "tab\t"} {
		if token, err := New(s); err == nil || token.Set() {
			t.Errorf("New(%q) accepted; want a refusal", s)
		}
	}
	token, _ := New("secret")
	if token.Matches("secret2") || token.Matches("") || (Token{}).Matches("") {
		t.Error("a token matched what it is not, or an unset token matched")
	}
}

func TestCheck(t *testing.T) {
	token, _ := New("right")
	cases := []struct {
		header string
		want   Refusal
		status int
		code   string
	}{
		{"", NothingPresented, http.StatusUnauthorized, ""},
		{"Basic cmlnaHQ=", NotBearer, http.StatusUnauthorized, "invalid_request"},
		{"Bearer", Malformed, http.StatusBadRequest, "invalid_request"},
		{"Bearer has space", Malformed, http.StatusBadRequest, "invalid_request"},
		{"Bearer wrong", Mismatch, http.StatusUnauthorized, "invalid_token"},
		{"Bearer right", Accepted, 0, ""},
		{"bearer right", Accepted, 0, ""},
		{"  Bearer   right  ", Accepted, 0, ""},
	}
	for _, c := range cases {
		got := token.Check(c.header)
		if got != c.want {
			t.Errorf("Check(%q) = %v, want %v", c.header, got, c.want)
			continue
		}
		if got == Accepted {
			continue
		}
		if got.HTTPStatus() != c.status || got.Code() != c.code {
			t.Errorf("Check(%q): status %d code %q, want %d %q", c.header, got.HTTPStatus(), got.Code(), c.status, c.code)
		}
		challenge := got.Challenge("svc")
		if !strings.HasPrefix(challenge, `Bearer realm="svc"`) || (c.code != "" && !strings.Contains(challenge, `error="`+c.code+`"`)) || (c.code == "" && strings.Contains(challenge, "error=")) {
			t.Errorf("Check(%q).Challenge = %q", c.header, challenge)
		}
		if strings.Contains(challenge, "right") || strings.Contains(got.Message(), "right") || strings.Contains(got.String(), "wrong") && got != Mismatch {
			t.Errorf("a refusal repeated a token: %q %q", challenge, got.Message())
		}
	}
	if NothingPresented.Message() != "Unauthorized: this server requires a bearer token; send Authorization: Bearer TOKEN" {
		t.Errorf("NothingPresented.Message() = %q", NothingPresented.Message())
	}
	if Malformed.Message() != "Bad Request: the bearer token is malformed" {
		t.Errorf("Malformed.Message() = %q", Malformed.Message())
	}
}

func TestRead(t *testing.T) {
	const env = "OPENSYSML_GUARD_TEST_TOKEN"
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte("from-file\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(env, "")

	token, text, err := Read("", "", env)
	if err != nil || token.Set() || text != "" {
		t.Errorf("nothing named: %v %q %v; want no token", token.Set(), text, err)
	}
	token, text, err = Read(file, "", env)
	if err != nil || text != "from-file" || !token.Matches("from-file") {
		t.Errorf("-token-file: %q %v; want from-file with its line break dropped", text, err)
	}
	token, text, err = Read("", "from-flag", env)
	if err != nil || text != "from-flag" || !token.Matches("from-flag") {
		t.Errorf("-token: %q %v", text, err)
	}
	t.Setenv(env, "from-env")
	token, text, err = Read("", "", env)
	if err != nil || text != "from-env" || !token.Matches("from-env") {
		t.Errorf("%s: %q %v", env, text, err)
	}
	if _, _, err := Read("", "from-flag", env); err != nil || !token.Matches("from-env") {
		t.Errorf("-token beside the variable: %v", err)
	}
	if _, _, err := Read(file, "from-flag", env); err == nil || !strings.Contains(err.Error(), "give one") {
		t.Errorf("both flags: %v; want a refusal", err)
	}
	if _, _, err := Read(filepath.Join(dir, "missing"), "", env); err == nil || !strings.HasPrefix(err.Error(), "-token-file:") {
		t.Errorf("missing file: %v", err)
	}
	t.Setenv(env, "has space")
	if _, _, err := Read("", "", env); err == nil || !strings.HasPrefix(err.Error(), env+":") {
		t.Errorf("malformed variable: %v; want the source named", err)
	}
}

func TestListenAddress(t *testing.T) {
	for spec, want := range map[string]string{
		"8765":                   "localhost:8765",
		":8765":                  "localhost:8765",
		"0":                      "localhost:0",
		"0.0.0.0:8765":           "0.0.0.0:8765",
		"[::]:8765":              "[::]:8765",
		"models.example.org:443": "models.example.org:443",
	} {
		got, err := ListenAddress(spec)
		if err != nil || got != want {
			t.Errorf("ListenAddress(%q) = %q, %v; want %q", spec, got, err, want)
		}
	}
	for _, spec := range []string{"", "host", "host:port", "host:70000", "70000"} {
		if got, err := ListenAddress(spec); err == nil {
			t.Errorf("ListenAddress(%q) = %q; want an error", spec, got)
		}
	}
}

func TestLoopback(t *testing.T) {
	for host, want := range map[string]bool{"localhost": true, "LOCALHOST": true, "127.0.0.1": true, "127.5.5.5": true, "::1": true, "0.0.0.0": false, "::": false, "10.0.0.1": false, "example.org": false} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v", host, got)
		}
	}
	for addr, want := range map[string]bool{"127.0.0.1:1": true, "[::1]:1": true, "0.0.0.0:1": false, "[::]:1": false, "10.0.0.1:1": false} {
		tcp, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		if got := IsLoopbackAddr(tcp); got != want {
			t.Errorf("IsLoopbackAddr(%s) = %v", addr, got)
		}
	}
	if IsLoopbackAddr(&net.UnixAddr{Name: "/tmp/x", Net: "unix"}) {
		t.Error("a unix socket counted as loopback")
	}
	err := NoToken("SOME_TOKEN")
	if !strings.Contains(err.Error(), "-token-file, -token or SOME_TOKEN") || !errors.Is(err, err) {
		t.Errorf("NoToken = %v", err)
	}
}
