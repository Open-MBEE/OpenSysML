package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// The service endpoints a deployment probes, served open on the same listener
// as HTTPPath: liveness, which answers as soon as the listener is bound, and
// readiness, which answers 200 only once the server is built and prewarmed.
const (
	HealthPath = "/health"
	ReadyPath  = "/ready"
)

// serviceName is what the service endpoints report themselves as.
const serviceName = "sysml-mcp"

// readiness is the gate between the service endpoints, which answer from the
// moment the listener is bound, and the protocol, which answers once the
// server behind it is built and prewarmed.
type readiness struct {
	version  string
	ready    atomic.Bool
	draining atomic.Bool
	protocol atomic.Pointer[http.Handler]
}

// ServeHealth answers GET with the build, as sysml-grpc does on its main port:
// a probe carries no token, and liveness names the build and no more.
func (r *readiness) ServeHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName, "version": r.version})
}

// ServeReady answers GET with 200 once the server is built and prewarmed, and
// 503 before that and again while it drains, so a balancer routes to it only
// while a client's first call would be answered in full.
func (r *readiness) ServeReady(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	status := "starting"
	code := http.StatusServiceUnavailable
	switch {
	case r.draining.Load():
		status = "draining"
	case r.ready.Load():
		status, code = "ready", http.StatusOK
	}
	if code != http.StatusOK {
		w.Header().Set("Retry-After", "1")
	}
	writeJSON(w, code, map[string]string{"status": status, "service": serviceName})
}

// ServeProtocol hands a request to the protocol once the server is built, and
// answers 503 with Retry-After before that, so a client arriving during the
// workspace index is told to come back rather than answered partially.
func (r *readiness) ServeProtocol(w http.ResponseWriter, req *http.Request) {
	if h := r.protocol.Load(); h != nil {
		(*h).ServeHTTP(w, req)
		return
	}
	w.Header().Set("Retry-After", "1")
	http.Error(w, "Service Unavailable: the server is still indexing the workspace and preparing the runtime; GET /ready answers 200 once it is ready", http.StatusServiceUnavailable)
}

// serve marks the server ready, routing the protocol paths to h.
func (r *readiness) serve(h http.Handler) {
	r.protocol.Store(&h)
	r.ready.Store(true)
}

// drain turns readiness off, for a stopping server to leave the rotation
// before its connections are closed.
func (r *readiness) drain() { r.draining.Store(true) }

// writeJSON answers with v as JSON; a write failing is a client gone, logged.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("service response write failed", "error", err)
	}
}

// ServeHTTP binds the service endpoints and the protocol path on over.Listener,
// then builds the server from cfg behind them: GET HealthPath answers from the
// start, GET ReadyPath and HTTPPath answer 503 until the workspace is indexed
// and the runtime prewarmed, and the protocol is served from then until ctx
// ends, when the live sessions are closed and the connections drained within
// the timeout.
func ServeHTTP(ctx context.Context, cfg Config, over HTTPConfig) error {
	return serveGated(ctx, over, cfg.Version, func(ctx context.Context) (*Server, error) {
		srv, err := New(ctx, cfg)
		if err != nil {
			return nil, err
		}
		select {
		case <-srv.Prewarmed():
		case <-ctx.Done():
		}
		return srv, nil
	})
}

// serveGated is ServeHTTP with the server built by build once the listener is
// serving; build returns when the server is ready for its first call.
func serveGated(ctx context.Context, over HTTPConfig, version string, build func(context.Context) (*Server, error)) error {
	if (over.CertFile == "") != (over.KeyFile == "") {
		return errors.New("a certificate file and a key file are both needed for TLS")
	}
	addr := over.Listener.Addr()
	if !IsLoopbackAddr(addr) && !over.Token.Set() {
		return fmt.Errorf("%s is an address other machines reach: %w", addr, ErrNoToken)
	}
	logger := over.Logger
	if logger == nil {
		logger = slog.Default()
	}
	gate := &readiness{version: version}
	httpSrv := &http.Server{Handler: serviceMux(gate, over), ReadHeaderTimeout: 10 * time.Second}
	security, auth := over.describe(addr, logger)
	if over.CertFile != "" {
		httpSrv.TLSConfig = tlsConfig()
	}
	logger.Info("listening for Streamable HTTP; the protocol follows the workspace index and the runtime prewarm",
		"addr", addr.String(), "health", HealthPath, "ready", ReadyPath, "security", security)

	errs := make(chan error, 1)
	go func() {
		if over.CertFile != "" {
			errs <- httpSrv.ServeTLS(over.Listener, over.CertFile, over.KeyFile)
			return
		}
		errs <- httpSrv.Serve(over.Listener)
	}()

	buildCtx, cancelBuild := context.WithCancel(ctx)
	defer cancelBuild()
	built := make(chan buildResult, 1)
	go func() {
		srv, err := build(buildCtx)
		built <- buildResult{srv, err}
	}()

	var srv *Server
	select {
	case err := <-errs:
		cancelBuild()
		if res := <-built; res.srv != nil {
			res.srv.Close(context.WithoutCancel(ctx))
		}
		return serveError(err)
	case res := <-built:
		if res.err != nil {
			_ = httpSrv.Close()
			return res.err
		}
		srv = res.srv
	}
	defer srv.Close(context.WithoutCancel(ctx))
	if ctx.Err() != nil {
		gate.drain()
		return shutdown(ctx, over, httpSrv, srv, logger)
	}
	gate.serve(srv.protocolHandler(over))
	logger.Info("serving the Model Context Protocol over Streamable HTTP",
		"addr", addr.String(), "path", HTTPPath, "security", security, "auth", auth, "allowed_origins", over.Origins.Allowed())

	select {
	case err := <-errs:
		return serveError(err)
	case <-ctx.Done():
	}
	gate.drain()
	return shutdown(ctx, over, httpSrv, srv, logger)
}

// serviceMux routes one listener: the service endpoints open, the protocol at
// HTTPPath behind the origin policy and gate, and the token's resource
// metadata open behind the origin policy when a token is required.
func serviceMux(gate *readiness, over HTTPConfig) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, gate.ServeHealth)
	mux.HandleFunc(ReadyPath, gate.ServeReady)
	mux.Handle(HTTPPath, over.Origins.Handler(http.HandlerFunc(gate.ServeProtocol)))
	if over.Token.Set() {
		for _, path := range over.Token.MetadataPaths() {
			mux.Handle(path, over.Origins.Handler(http.HandlerFunc(over.Token.ServeMetadata)))
		}
	}
	return mux
}

// buildResult is what the goroutine building the server hands back.
type buildResult struct {
	srv *Server
	err error
}

// serveError is the error Serve returned, with the one a shutdown causes
// read as a clean stop.
func serveError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// shutdown closes the live sessions and drains the connections within the
// configured timeout, closing what did not drain.
func shutdown(ctx context.Context, over HTTPConfig, httpSrv *http.Server, srv *Server, logger *slog.Logger) error {
	timeout := over.ShutdownTimeout
	if timeout == 0 {
		timeout = DefaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- httpSrv.Shutdown(shutdownCtx) }()
	// A client's event stream stays open until its session ends, so the
	// sessions are closed for the shutdown to see the connections idle.
	for sess := range srv.sdk.Sessions() {
		_ = sess.Close()
	}
	if err := <-done; err != nil {
		_ = httpSrv.Close()
		return fmt.Errorf("stopping the HTTP server: %w", err)
	}
	logger.Info("stopped serving over Streamable HTTP")
	return nil
}
