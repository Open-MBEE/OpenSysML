package sysmlapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsePastTheBoundIsATypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	c.maxBody = 1024
	_, _, err := c.Do(context.Background(), http.MethodGet, server.URL+"/projects", nil, "", nil)
	var tooLarge *ResponseTooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.Limit != 1024 {
		t.Fatalf("err = %v, want a ResponseTooLargeError at 1024 bytes", err)
	}

	c.maxBody = 2048
	if content, _, err := c.Do(context.Background(), http.MethodGet, server.URL+"/projects", nil, "", nil); err != nil || len(content) != 2048 {
		t.Fatalf("a body of exactly the bound: %d bytes, %v", len(content), err)
	}
}
