package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
)

// The health endpoint is what an orchestrator polls, so it answers /health with
// the build it is serving, and nothing else.
func TestHealthHandlerAnswersOnlyHealth(t *testing.T) {
	handler := healthHandler("v1.2.3")

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal %q: %v", res.Body.String(), err)
	}
	want := map[string]string{"status": "ok", "service": "sysml-grpc", "version": "v1.2.3"}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("body[%q] = %q, want %q", key, body[key], value)
		}
	}

	other := httptest.NewRecorder()
	handler.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/", nil))
	if other.Code != http.StatusNotFound {
		t.Errorf("/ status = %d, want 404: the health server serves one route", other.Code)
	}
}

// The interceptor only logs, so a call's answer and its failure both reach the
// client unchanged.
func TestUnaryAccessInterceptorPassesTheCallThrough(t *testing.T) {
	interceptor := unaryAccessInterceptor(access{})
	info := &grpc.UnaryServerInfo{FullMethod: "/sysml.SysMLService/GetServerInfo"}

	res, err := interceptor(context.Background(), "request", info,
		func(ctx context.Context, req interface{}) (interface{}, error) { return "response", nil })
	if err != nil {
		t.Fatalf("a successful call reported %v", err)
	}
	if res != "response" {
		t.Errorf("response = %v, want %q", res, "response")
	}

	failure := errors.New("no such model")
	res, err = interceptor(context.Background(), "request", info,
		func(ctx context.Context, req interface{}) (interface{}, error) { return nil, failure })
	if !errors.Is(err, failure) {
		t.Errorf("error = %v, want %v", err, failure)
	}
	if res != nil {
		t.Errorf("a failed call answered %v, want nil", res)
	}
}
