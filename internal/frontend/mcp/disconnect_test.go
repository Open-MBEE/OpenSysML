package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// post sends one JSON-RPC message to endpoint in session, and answers the response.
func post(ctx context.Context, t *testing.T, endpoint, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
		req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// TestHungUpRequestEndsItsToolCall checks a tools/call stops when the client
// drops the request that carried it, with no cancellation notice sent.
func TestHungUpRequestEndsItsToolCall(t *testing.T) {
	srv, err := New(context.Background(), Config{Root: t.TempDir(), Groups: []Group{GroupSession}, Version: "test",
		NewSession: func() (*repl.Session, error) { return repl.NewSession(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(context.Background()) })
	entered, ended := make(chan struct{}), make(chan struct{})
	sdk.AddTool(srv.sdk, &sdk.Tool{Name: "wait"}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, struct{}, error) {
		close(entered)
		<-ctx.Done()
		close(ended)
		return nil, struct{}{}, ctx.Err()
	})
	mux := http.NewServeMux()
	mux.Handle(HTTPPath, srv.protocolHandler(HTTPConfig{}))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	endpoint := ts.URL + HTTPPath

	ctx := context.Background()
	opened := post(ctx, t, endpoint, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_, _ = io.Copy(io.Discard, opened.Body)
	_ = opened.Body.Close()
	session := opened.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("initialize answered %s with no session", opened.Status)
	}
	_ = post(ctx, t, endpoint, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`).Body.Close()

	calling, hangUp := context.WithCancel(ctx)
	defer hangUp()
	go func() {
		req, _ := http.NewRequestWithContext(calling, http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait","arguments":{}}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", session)
		req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("the tool was never called")
	}
	hangUp()
	select {
	case <-ended:
	case <-time.After(30 * time.Second):
		t.Fatal("the tool call outlived the request its client dropped")
	}
}

// TestCallOfAnEndedRequestIsNotStarted checks a call whose POST already ended
// gets a context already over, and one no POST carried, as over stdio, its own.
func TestCallOfAnEndedRequestIsNotStarted(t *testing.T) {
	var live liveRequests
	var carried http.Header
	live.track(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { carried = r.Header })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, HTTPPath, nil))
	if carried.Get(requestHeader) == "" {
		t.Fatal("the POST was not numbered")
	}

	late, release := live.bound(context.Background(), &sdk.CallToolRequest{Extra: &sdk.RequestExtra{Header: carried}})
	defer release()
	if late.Err() == nil {
		t.Error("a call of a POST that already ended was left running")
	}
	local, release := live.bound(context.Background(), &sdk.CallToolRequest{})
	defer release()
	if local.Err() != nil {
		t.Error("a call no POST carried was ended")
	}
}
