package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestServeHTTPAnswersProbesWhileTheServerBuilds checks the listener answers
// /health from the start, /ready and /mcp with 503 until the server is built,
// and the protocol from then on, each probe open of the token.
func TestServeHTTPAnswersProbesWhileTheServerBuilds(t *testing.T) {
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tok, _ := NewBearerToken("s3cret")
	cfg := sessionConfig(t)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serveGated(ctx, HTTPConfig{Listener: lis, Token: tok, Logger: slog.New(slog.NewTextHandler(&logs, nil))}, "1.2.3",
			func(ctx context.Context) (*Server, error) {
				<-release
				return New(ctx, cfg)
			})
	}()
	base := "http://" + lis.Addr().String()
	get := func(path string) (int, map[string]string, http.Header) {
		t.Helper()
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body, resp.Header
	}
	if code, body, _ := get(HealthPath); code != http.StatusOK || body["status"] != "ok" || body["service"] != "sysml-mcp" || body["version"] != "1.2.3" {
		t.Errorf("GET /health while building: %d %v", code, body)
	}
	if code, body, h := get(ReadyPath); code != http.StatusServiceUnavailable || body["status"] != "starting" || h.Get("Retry-After") != "1" {
		t.Errorf("GET /ready while building: %d %v %v", code, body, h)
	}
	resp, err := http.Post(base+HTTPPath, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") != "1" {
		t.Errorf("POST /mcp while building: %d %v", resp.StatusCode, resp.Header)
	}

	close(release)
	waitReady(t, base)
	if code, body, _ := get(ReadyPath); code != http.StatusOK || body["status"] != "ready" {
		t.Errorf("GET /ready once built: %d %v", code, body)
	}
	if code, _, _ := get(HTTPPath); code != http.StatusUnauthorized {
		t.Errorf("GET /mcp with no token once built: %d, want 401", code)
	}
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: base + HTTPPath, HTTPClient: &http.Client{Transport: headerTransport{"Authorization", "Bearer s3cret"}}}, nil)
	if err != nil {
		t.Fatalf("Connect once built: %v", err)
	}
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools: %v", err)
	}
	_ = cs.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ServeHTTP: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("ServeHTTP did not return after its context ended")
	}
	for _, want := range []string{"listening for Streamable HTTP", "health=/health", "serving the Model Context Protocol", "stopped serving"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, logs.String())
		}
	}
}

// TestReadinessDrains checks a stopping server reports 503 draining, and that
// the probes answer GET and HEAD alone.
func TestReadinessDrains(t *testing.T) {
	gate := &readiness{version: "test"}
	gate.serve(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	gate.ServeReady(rec, httptest.NewRequest(http.MethodGet, ReadyPath, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("ready: %d", rec.Code)
	}
	gate.drain()
	rec = httptest.NewRecorder()
	gate.ServeReady(rec, httptest.NewRequest(http.MethodGet, ReadyPath, nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"draining"`) {
		t.Errorf("draining: %d %s", rec.Code, rec.Body)
	}
	for _, serve := range []http.HandlerFunc{gate.ServeHealth, gate.ServeReady} {
		rec = httptest.NewRecorder()
		serve(rec, httptest.NewRequest(http.MethodPost, HealthPath, nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("POST to a probe: %d %v", rec.Code, rec.Header())
		}
	}
}

// TestServeHTTPBoundsTheRequestBody checks a body over MaxBodyBytes is refused
// with 413 and one under it is read.
func TestServeHTTPBoundsTheRequestBody(t *testing.T) {
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := sessionConfig(t)
	cfg.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	done := make(chan error, 1)
	go func() { done <- ServeHTTP(ctx, cfg, HTTPConfig{Listener: lis, MaxBodyBytes: 256, Logger: cfg.Logger}) }()
	base := "http://" + lis.Addr().String()
	waitReady(t, base)
	post := func(body string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, base+HTTPPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	if code := post(initialize); code != http.StatusOK {
		t.Errorf("a %d-byte initialize: %d", len(initialize), code)
	}
	if code := post(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"` + strings.Repeat("x", 300) + `","version":"0"}}}`); code != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over the bound: %d, want 413", code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("ServeHTTP: %v", err)
	}
}
