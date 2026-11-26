package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// startSlowService is startService answering each call only after delay, or when the
// caller gives up or the test ends.
func startSlowService(t *testing.T, delay time.Duration) string {
	t.Helper()
	target, err := url.Parse(startService(t))
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	proxy := httputil.NewSingleHostReverseProxy(target)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			proxy.ServeHTTP(w, r)
		case <-r.Context().Done():
		case <-released:
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(released) })
	return server.URL
}

func TestACallToAServiceThatStopsAnsweringIsDeadlineExceeded(t *testing.T) {
	address := startSlowService(t, time.Hour)
	start := time.Now()
	_, err := dialClient(t, address, opensysml.WithCallTimeout(100*time.Millisecond)).ParseSource(context.Background(), vehicleSource)
	if !errors.Is(err, opensysml.CodeDeadlineExceeded) {
		t.Fatalf("err = %v, want CodeDeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Fatalf("the call returned after %s", elapsed)
	}
}

func TestTheCallTimeoutYieldsToTheCallersDeadlineAndToZero(t *testing.T) {
	address := startSlowService(t, 300*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := dialClient(t, address, opensysml.WithCallTimeout(20*time.Millisecond)).ParseSource(ctx, vehicleSource); err != nil {
		t.Errorf("a context with its own deadline was cut short: %v", err)
	}
	if _, err := dialClient(t, address, opensysml.WithCallTimeout(0)).ParseSource(context.Background(), vehicleSource); err != nil {
		t.Errorf("a zero call timeout bounded the call: %v", err)
	}
	if _, err := dialClient(t, address).ParseSource(context.Background(), vehicleSource); err != nil {
		t.Errorf("the default call timeout cut a 300ms call: %v", err)
	}
}
