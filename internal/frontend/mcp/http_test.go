package mcp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func TestParseOrigins(t *testing.T) {
	p, err := ParseOrigins(" https://agents.example.org, HTTP://Agents.Example.org:8080 ,,http://[::1]:3000")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Allowed(); !slices.Equal(got, []string{"http://[::1]:3000", "http://agents.example.org:8080", "https://agents.example.org"}) {
		t.Errorf("Allowed() = %v", got)
	}
	for _, bad := range []string{"*", "https://a.example.org, *", "agents.example.org", "https://agents.example.org/mcp", "https://user@agents.example.org", "https://agents.example.org?x=1", "https://", "https:///path"} {
		if _, err := ParseOrigins(bad); err == nil {
			t.Errorf("ParseOrigins(%q) accepted it", bad)
		}
	}
	empty, err := ParseOrigins("")
	if err != nil || len(empty.Allowed()) != 0 {
		t.Errorf("ParseOrigins(\"\") = %v, %v", empty.Allowed(), err)
	}
}

func TestOriginPolicyAllows(t *testing.T) {
	p, err := ParseOrigins("https://agents.example.org")
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{
		"",
		"http://localhost", "http://localhost:3000", "https://LOCALHOST:8443",
		"http://127.0.0.1", "http://127.0.0.1:8765", "http://127.1.2.3:80",
		"http://[::1]", "http://[::1]:3000",
		"https://agents.example.org", "HTTPS://AGENTS.EXAMPLE.ORG",
	}
	for _, origin := range allowed {
		if !p.Allows(origin) {
			t.Errorf("Allows(%q) = false", origin)
		}
	}
	refused := []string{
		"null",
		"https://evil.example.org",
		"http://agents.example.org",
		"https://agents.example.org:8443",
		"https://agents.example.org/",
		"http://localhost.example.org",
		"http://10.0.0.5:8765",
		"http://[fe80::1]:8765",
		"localhost",
		"file://",
	}
	for _, origin := range refused {
		if p.Allows(origin) {
			t.Errorf("Allows(%q) = true", origin)
		}
	}
	var none OriginPolicy
	if !none.Allows("") || !none.Allows("http://localhost:3000") || none.Allows("https://agents.example.org") {
		t.Error("the zero policy should admit no origin and loopback only")
	}
}

func TestOriginPolicyHandler(t *testing.T) {
	p, _ := ParseOrigins("https://agents.example.org")
	var served []string
	h := p.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.Header.Get("Origin"))
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		origin string
		status int
	}{
		{"", http.StatusNoContent},
		{"http://localhost:3000", http.StatusNoContent},
		{"https://agents.example.org", http.StatusNoContent},
		{"https://evil.example.org", http.StatusForbidden},
		{"null", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("origin %q: status %d, want %d: %s", tc.origin, rec.Code, tc.status, rec.Body)
		}
		if tc.status == http.StatusForbidden && !strings.Contains(rec.Body.String(), tc.origin) {
			t.Errorf("origin %q: the refusal does not name it: %s", tc.origin, rec.Body)
		}
	}
	if !slices.Equal(served, []string{"", "http://localhost:3000", "https://agents.example.org"}) {
		t.Errorf("served origins = %q", served)
	}
}

// TestOriginPolicyHandlerGrantsCORSByName checks that only a named origin gets
// the CORS headers and a 204 preflight; loopback is served without a grant.
func TestOriginPolicyHandlerGrantsCORSByName(t *testing.T) {
	p, _ := ParseOrigins("https://agents.example.org")
	h := p.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	do := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, HTTPPath, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodOptions, "https://agents.example.org")
	if rec.Code != http.StatusNoContent {
		t.Errorf("named preflight: status %d, want 204", rec.Code)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":  "https://agents.example.org",
		"Access-Control-Allow-Methods": corsAllowMethods,
		"Access-Control-Allow-Headers": corsAllowHeaders,
		"Vary":                         "Origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("named preflight: %s = %q, want %q", header, got, want)
		}
	}
	for _, header := range []string{"Mcp-Session-Id", "MCP-Protocol-Version"} {
		if !strings.Contains(corsAllowHeaders, header) || !strings.Contains(corsExposeHeaders, header) {
			t.Errorf("%s is not both allowed and exposed", header)
		}
	}

	rec = do(http.MethodPost, "https://AGENTS.example.org")
	if rec.Code != http.StatusAccepted || rec.Header().Get("Access-Control-Allow-Origin") != "https://AGENTS.example.org" || rec.Header().Get("Access-Control-Expose-Headers") != corsExposeHeaders {
		t.Errorf("named request: status %d, headers %v", rec.Code, rec.Header())
	}

	rec = do(http.MethodPost, "http://localhost:3000")
	if rec.Code != http.StatusAccepted || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("loopback request: status %d, headers %v; want served without a grant", rec.Code, rec.Header())
	}
	rec = do(http.MethodOptions, "http://localhost:3000")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "-allow-origin") {
		t.Errorf("loopback preflight: status %d, body %q; want 403 naming -allow-origin", rec.Code, rec.Body)
	}

	rec = do(http.MethodOptions, "")
	if rec.Code != http.StatusAccepted || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("no origin: status %d, headers %v; want passed through", rec.Code, rec.Header())
	}
}

// serveHTTP serves a new server over a loopback listener until the returned
// stop function is called, which also waits for ServeStreamableHTTP to return.
func serveHTTP(t *testing.T, cfg Config, logs *bytes.Buffer, addr string) (url string, stop func() error) {
	t.Helper()
	return serveHTTPWith(t, cfg, logs, addr, BearerToken{})
}

// serveHTTPWith is serveHTTP requiring token of every request.
func serveHTTPWith(t *testing.T, cfg Config, logs *bytes.Buffer, addr string, token BearerToken) (url string, stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cfg.NewSession = func() (*repl.Session, error) { return repl.NewSession(), nil }
	cfg.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { srv.Close(context.Background()) })
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- srv.ServeStreamableHTTP(ctx, HTTPConfig{Listener: lis, Token: token, ShutdownTimeout: 20 * time.Second})
	}()
	return "http://" + lis.Addr().String() + HTTPPath, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(30 * time.Second):
			t.Fatal("ServeStreamableHTTP did not return after its context ended")
			return nil
		}
	}
}

func TestServeStreamableHTTPSharesTheServerAndStopsWithLiveSessions(t *testing.T) {
	var logs bytes.Buffer
	url, stop := serveHTTP(t, Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test"}, &logs, "127.0.0.1:0")
	ctx := context.Background()
	var clients []*sdk.ClientSession
	for range 2 {
		c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
		cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: url}, nil)
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		defer cs.Close()
		clients = append(clients, cs)
	}
	var opened struct {
		Session string `json:"session"`
	}
	call(t, clients[0], "session_open", map[string]any{"session": "shared"}, &opened)
	var list struct {
		Sessions []string `json:"sessions"`
	}
	call(t, clients[1], "session_list", map[string]any{}, &list)
	if !slices.Equal(list.Sessions, []string{"shared"}) {
		t.Errorf("the second client sees sessions %v, want [shared]", list.Sessions)
	}

	// The event streams of both clients are open; stopping must not wait for
	// them to end on their own.
	started := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("ServeStreamableHTTP: %v", err)
	}
	if took := time.Since(started); took > 15*time.Second {
		t.Errorf("stopping took %v with live sessions", took)
	}
	for i, cs := range clients {
		if _, err := cs.ListTools(ctx, nil); err == nil {
			t.Errorf("client %d was served after the server stopped", i)
		}
	}
	if log := logs.String(); !strings.Contains(log, "path=/mcp") || !strings.Contains(log, "security=cleartext") || !strings.Contains(log, "auth=none") || !strings.Contains(log, "stopped serving") {
		t.Errorf("log:\n%s", log)
	}
}

func TestServeStreamableHTTPRefusesOffLoopbackWithoutAToken(t *testing.T) {
	srv, err := New(context.Background(), Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test",
		NewSession: func() (*repl.Session, error) { return repl.NewSession(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close(context.Background())
	lis, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	err = srv.ServeStreamableHTTP(context.Background(), HTTPConfig{Listener: lis})
	if !errors.Is(err, ErrNoToken) || !strings.Contains(err.Error(), "-token-file") || !strings.Contains(err.Error(), TokenEnv) {
		t.Errorf("ServeStreamableHTTP on every interface with no token: %v", err)
	}
}

func TestServeStreamableHTTPRefusesHalfATLSPair(t *testing.T) {
	srv, err := New(context.Background(), Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test",
		NewSession: func() (*repl.Session, error) { return repl.NewSession(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close(context.Background())
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	if err := srv.ServeStreamableHTTP(context.Background(), HTTPConfig{Listener: lis, CertFile: "server.crt"}); err == nil || !strings.Contains(err.Error(), "both") {
		t.Errorf("ServeStreamableHTTP with a certificate alone: %v", err)
	}
}
